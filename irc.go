/*
 * Provide IRC client functionality.
 *
 * I would like to be able to write bots using this package.
 */

package irc

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
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
	Config Config

	// connected: Whether currently connected or not
	connected bool

	// conn: The connection if we are actively connected.
	conn net.Conn

	// rw: Read/write handle to the connection
	rw *bufio.ReadWriter

	// ActualNick: The nick we have if we are currently connected. The requested
	// nick may not always be available.
	ActualNick string
}

// timeoutTime is how long we wait on network I/O.
const timeoutTime = 5 * time.Minute

// timeoutConnect is how long we wait for connection attempts to time out.
const timeoutConnect = 30 * time.Second

var Hooks []func(*Conn, Message)

// LoadConfig allows you to load a config file.
//
// Format:
// key=value
//
// # type comments permitted.
func (c *Conn) LoadConfig(file string) error {
	config, err := parseConfig(file)
	if err != nil {
		return err
	}

	c.Config = config
	return nil
}

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
		c.connected = true
	} else {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port),
			timeoutConnect)

		if err != nil {
			return err
		}

		c.conn = conn
		c.connected = true
	}

	c.rw = bufio.NewReadWriter(bufio.NewReader(c.conn), bufio.NewWriter(c.conn))

	err := c.greet()
	if err != nil {
		return err
	}

	return nil
}

// greet runs connection initiation (NICK, USER)
func (c *Conn) greet() error {
	err := c.write(fmt.Sprintf("NICK %s\r\n", c.Nick))
	if err != nil {
		return fmt.Errorf("Failed to send NICK: %s", err.Error())
	}

	err = c.write(fmt.Sprintf("USER %s 0 * :%s\r\n", c.Ident, c.Name))
	if err != nil {
		return fmt.Errorf("Failed to send NICK: %s", err.Error())
	}

	for {
		line, err := c.read()
		if err != nil {
			return err
		}

		msg, err := parseMessage(line)
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

		line, err := c.read()
		if err != nil {
			return err
		}

		msg, err := parseMessage(line)
		if err != nil {
			return err
		}

		if msg.Command == "PING" {
			err = c.write(fmt.Sprintf("PONG %s\r\n", msg.Params[0]))
			if err != nil {
				return fmt.Errorf("Failed to send PONG: %s", err.Error())
			}
			log.Printf("Sent PONG.")
		}

		c.hooks(msg)
	}
}

// hooks calls each registered IRC package hook.
func (c *Conn) hooks(message Message) {
	for _, hook := range Hooks {
		hook(c, message)
	}
}

// Join joins a channel.
func (c *Conn) Join(name string) error {
	return c.write(fmt.Sprintf("JOIN %s\r\n", name))
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
		err := c.write(command)
		if err != nil {
			return nil
		}
	}

	return nil
}

// Quit sends a quit.
func (c *Conn) Quit(message string) error {
	return c.write(fmt.Sprintf("QUIT :%s\r\n", message))
}

// Oper sends an OPER command
func (c *Conn) Oper(name string, password string) error {
	return c.write(fmt.Sprintf("OPER %s %s\r\n", name, password))
}

// UserMode sends a MODE command.
func (c *Conn) UserMode(nick string, modes string) error {
	return c.write(fmt.Sprintf("MODE %s %s\r\n", nick, modes))
}

// read reads a message from the connection.
func (c *Conn) read() (string, error) {
	err := c.conn.SetDeadline(time.Now().Add(timeoutTime))
	if err != nil {
		return "", fmt.Errorf("Unable to set deadline: %s", err)
	}

	line, err := c.rw.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("Unable to read: %s", err)
	}

	log.Printf("Read: %s", strings.TrimSpace(line))

	return line, nil
}

// write writes a string to the connection
func (c *Conn) write(s string) error {
	err := c.conn.SetDeadline(time.Now().Add(timeoutTime))
	if err != nil {
		return fmt.Errorf("Unable to set deadline: %s", err)
	}

	sz, err := c.rw.WriteString(s)
	if err != nil {
		return fmt.Errorf("Unable to write: %s", err)
	}

	if sz != len(s) {
		return fmt.Errorf("Short write")
	}

	err = c.rw.Flush()
	if err != nil {
		return fmt.Errorf("Flush error: %s", err)
	}

	log.Printf("Sent: %s", strings.TrimSpace(s))

	return nil
}
