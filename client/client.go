// Package client is an IRC client library.
package client

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/horgh/irc"
)

// Conn holds an IRC connection.
type Conn struct {
	// conn: The connection if we are actively connected.
	conn net.Conn

	// rw: Read/write handle to the connection
	rw *bufio.ReadWriter

	// Nick is the desired nickname.
	Nick string

	// Name is the realname to use.
	Name string

	// Ident is the ident portion to use.
	Ident string

	// Host is the IP/hostname of the IRC server to connect to.
	Host string

	// Port is the port of the host of the IRC server to connect to.
	Port int

	// TLS toggles whether we connect with TLS/SSL or not.
	TLS bool

	// Config holds the parsed config file data.
	Config map[string]string

	// connected: Whether currently connected or not
	connected bool

	// ActualNick: The nick we have if we are currently connected. The requested
	// nick may not always be available.
	ActualNick string

	// sentQUIT tracks whether we sent a QUIT message.
	sentQUIT bool
}

// timeoutConnect is how long we wait for connection attempts to time out.
const timeoutConnect = 30 * time.Second

// timeoutTime is how long we wait on network I/O by default.
const timeoutTime = 5 * time.Minute

// Hooks are functions to call for each message. Packages can take actions
// this way.
var Hooks []func(*Conn, irc.Message)

// Connect attempts to connect to a server.
func (c *Conn) Connect() error {
	if c.TLS {
		dialer := &net.Dialer{Timeout: timeoutConnect}
		conn, err := tls.DialWithDialer(dialer, "tcp",
			fmt.Sprintf("%s:%d", c.Host, c.Port),
			&tls.Config{
				// Typically IRC servers won't have valid certs.
				InsecureSkipVerify: true,
			})
		if err != nil {
			return err
		}
		c.conn = conn
	} else {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port),
			timeoutConnect)
		if err != nil {
			return err
		}
		c.conn = conn
	}

	c.connected = true
	c.rw = bufio.NewReadWriter(bufio.NewReader(c.conn), bufio.NewWriter(c.conn))

	return c.greet()
}

// ReadMessage reads a line from the connection and parses it as an IRC message.
func (c Conn) ReadMessage() (irc.Message, error) {
	buf, err := c.read()
	if err != nil {
		return irc.Message{}, err
	}

	m, err := irc.ParseMessage(buf)
	if err != nil && err != irc.ErrTruncated {
		return irc.Message{}, fmt.Errorf("unable to parse message: %s: %s", buf,
			err)
	}

	return m, nil
}

// read reads a line from the connection.
func (c Conn) read() (string, error) {
	if err := c.conn.SetDeadline(time.Now().Add(timeoutTime)); err != nil {
		return "", fmt.Errorf("unable to set deadline: %s", err)
	}

	line, err := c.rw.ReadString('\n')
	if err != nil {
		return "", err
	}

	log.Printf("Read: %s", strings.TrimRight(line, "\r\n"))

	return line, nil
}

// WriteMessage writes an IRC message to the connection.
func (c Conn) WriteMessage(m irc.Message) error {
	buf, err := m.Encode()
	if err != nil && err != irc.ErrTruncated {
		return fmt.Errorf("unable to encode message: %s", err)
	}

	return c.write(buf)
}

// write writes a string to the connection
func (c Conn) write(s string) error {
	if err := c.conn.SetDeadline(time.Now().Add(timeoutTime)); err != nil {
		return fmt.Errorf("unable to set deadline: %s", err)
	}

	sz, err := c.rw.WriteString(s)
	if err != nil {
		return err
	}

	if sz != len(s) {
		return fmt.Errorf("short write")
	}

	if err := c.rw.Flush(); err != nil {
		return fmt.Errorf("flush error: %s", err)
	}

	log.Printf("Sent: %s", strings.TrimRight(s, "\r\n"))

	return nil
}

// greet runs connection initiation (NICK, USER)
func (c *Conn) greet() error {
	if err := c.SendGreeting(); err != nil {
		return err
	}

	for {
		msg, err := c.ReadMessage()
		if err != nil {
			return err
		}

		c.hooks(msg)

		// Look for numeric reply 1. This is RPL_WELCOME welcoming our connection.
		if msg.Command == "001" {
			c.ActualNick = c.Nick
			return nil
		}
	}
}

// SendGreeting sends the client's greeting. This consists of NICK and USER.
func (c *Conn) SendGreeting() error {
	if err := c.WriteMessage(irc.Message{
		Command: "NICK",
		Params:  []string{c.Nick},
	}); err != nil {
		return fmt.Errorf("failed to send NICK: %s", err)
	}

	if err := c.WriteMessage(irc.Message{
		Command: "USER",
		Params:  []string{c.Ident, "0", "*", c.Name},
	}); err != nil {
		return fmt.Errorf("failed to send NICK: %s", err)
	}

	return nil
}

// Loop enters a loop reading from the server.
//
// We maintain the IRC connection.
//
// Hook events will fire.
func (c *Conn) Loop() error {
	for {
		if !c.connected {
			return c.Connect()
		}

		msg, err := c.ReadMessage()
		if err != nil {
			return err
		}

		if msg.Command == "PING" {
			if err := c.Pong(msg); err != nil {
				return err
			}
		}

		if msg.Command == "ERROR" {
			// After sending QUIT, the server acknowledges it with an ERROR
			// command.
			if c.sentQUIT {
				return c.conn.Close()
			}
		}

		c.hooks(msg)
	}
}

// hooks calls each registered IRC package hook.
func (c *Conn) hooks(message irc.Message) {
	for _, hook := range Hooks {
		hook(c, message)
	}
}

// Pong sends a PONG in response to the given PING message.
func (c *Conn) Pong(ping irc.Message) error {
	return c.WriteMessage(irc.Message{
		Command: "PONG",
		Params:  []string{ping.Params[0]},
	})
}

// Join joins a channel.
func (c *Conn) Join(name string) error {
	return c.WriteMessage(irc.Message{
		Command: "JOIN",
		Params:  []string{name},
	})
}

// Message sends a message.
//
// If the message is too long for a single line, then it will be split over
// several lines.
func (c *Conn) Message(target string, message string) error {
	// 512 is the maximum IRC protocol length.
	// However, user and host takes up some of that. Let's cut down a bit.
	// This is arbitrary.
	maxMessage := 412

	// Number of overhead bytes.
	overhead := len("PRIVMSG ") + len(" :") + len("\r\n")

	for i := 0; i < len(message); i += maxMessage - overhead {
		endIndex := i + maxMessage - overhead
		if endIndex > len(message) {
			endIndex = len(message)
		}
		piece := message[i:endIndex]

		if err := c.WriteMessage(irc.Message{
			Command: "PRIVMSG",
			Params:  []string{target, piece},
		}); err != nil {
			return nil
		}
	}

	return nil
}

// Quit sends a quit.
//
// We track when we send this as we expect an ERROR message in response.
func (c *Conn) Quit(message string) error {
	if err := c.WriteMessage(irc.Message{
		Command: "QUIT",
		Params:  []string{message},
	}); err != nil {
		return err
	}

	c.sentQUIT = true
	return nil
}

// Oper sends an OPER command
func (c *Conn) Oper(name string, password string) error {
	return c.WriteMessage(irc.Message{
		Command: "OPER",
		Params:  []string{name, password},
	})
}

// UserMode sends a MODE command.
func (c *Conn) UserMode(nick string, modes string) error {
	return c.WriteMessage(irc.Message{
		Command: "MODE",
		Params:  []string{nick, modes},
	})
}
