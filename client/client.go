/*
 * Package client is an IRC client library.
 */

package client

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"time"

	"summercat.com/irc"
)

// Conn holds an IRC connection.
type Conn struct {
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
	Config irc.Config

	// connected: Whether currently connected or not
	connected bool

	conn irc.Conn

	// ActualNick: The nick we have if we are currently connected. The requested
	// nick may not always be available.
	ActualNick string

	// sentQUIT tracks whether we sent a QUIT message.
	sentQUIT bool
}

// timeoutConnect is how long we wait for connection attempts to time out.
const timeoutConnect = 30 * time.Second

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

		c.conn.Conn = conn
		c.connected = true
	} else {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port),
			timeoutConnect)

		if err != nil {
			return err
		}

		c.conn.Conn = conn
		c.connected = true
	}

	c.conn.RW = bufio.NewReadWriter(bufio.NewReader(c.conn.Conn),
		bufio.NewWriter(c.conn.Conn))

	err := c.greet()
	if err != nil {
		return err
	}

	return nil
}

// greet runs connection initiation (NICK, USER)
func (c *Conn) greet() error {
	err := c.conn.Write(fmt.Sprintf("NICK %s\r\n", c.Nick))
	if err != nil {
		return fmt.Errorf("Failed to send NICK: %s", err.Error())
	}

	err = c.conn.Write(fmt.Sprintf("USER %s 0 * :%s\r\n", c.Ident, c.Name))
	if err != nil {
		return fmt.Errorf("Failed to send NICK: %s", err.Error())
	}

	for {
		line, err := c.conn.Read()
		if err != nil {
			return err
		}

		msg, err := irc.ParseMessage(line)
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

// Loop enters a loop reading from the server.
//
// We maintain the IRC connection.
//
// Hook events will fire.
func (c *Conn) Loop() error {
	for {
		if !c.connected {
			err := c.Connect()
			return err
		}

		line, err := c.conn.Read()
		if err != nil {
			return err
		}

		msg, err := irc.ParseMessage(line)
		if err != nil {
			return err
		}

		if msg.Command == "PING" {
			err = c.conn.Write(fmt.Sprintf("PONG %s\r\n", msg.Params[0]))
			if err != nil {
				return fmt.Errorf("Failed to send PONG: %s", err.Error())
			}
			log.Printf("Sent PONG.")
		}

		if msg.Command == "ERROR" {
			// After sending QUIT, the server acknowledges it with an ERROR
			// command.
			if c.sentQUIT {
				log.Printf("Received QUIT acknowldgement. Closing connection.")
				return c.conn.Conn.Close()
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

// Join joins a channel.
func (c *Conn) Join(name string) error {
	return c.conn.Write(fmt.Sprintf("JOIN %s\r\n", name))
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

		command := fmt.Sprintf("PRIVMSG %s :%s\r\n", target, piece)
		err := c.conn.Write(command)
		if err != nil {
			return nil
		}
	}

	return nil
}

// Quit sends a quit.
//
// We track when we send this as we expect an ERROR message in response.
func (c *Conn) Quit(message string) error {
	err := c.conn.Write(fmt.Sprintf("QUIT :%s\r\n", message))
	if err == nil {
		c.sentQUIT = true
	}
	return err
}

// Oper sends an OPER command
func (c *Conn) Oper(name string, password string) error {
	return c.conn.Write(fmt.Sprintf("OPER %s %s\r\n", name, password))
}

// UserMode sends a MODE command.
func (c *Conn) UserMode(nick string, modes string) error {
	return c.conn.Write(fmt.Sprintf("MODE %s %s\r\n", nick, modes))
}
