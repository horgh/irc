/*
 * Package client is an IRC client library.
 */

package client

import (
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
	var conn net.Conn
	var err error

	if c.TLS {
		dialer := &net.Dialer{Timeout: timeoutConnect}
		conn, err = tls.DialWithDialer(dialer, "tcp",
			fmt.Sprintf("%s:%d", c.Host, c.Port),
			&tls.Config{
				// Typically IRC servers won't have valid certs.
				InsecureSkipVerify: true,
			})

		if err != nil {
			return err
		}

		c.connected = true
	} else {
		conn, err = net.DialTimeout("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port),
			timeoutConnect)

		if err != nil {
			return err
		}

		c.connected = true
	}

	c.conn = irc.NewConn(conn)

	err = c.greet()
	if err != nil {
		return err
	}

	return nil
}

// greet runs connection initiation (NICK, USER)
func (c *Conn) greet() error {
	err := c.conn.WriteMessage(irc.Message{
		Command: "NICK",
		Params:  []string{c.Nick},
	})
	if err != nil {
		return fmt.Errorf("Failed to send NICK: %s", err)
	}

	err = c.conn.WriteMessage(irc.Message{
		Command: "USER",
		Params:  []string{c.Ident, "0", "*", c.Name},
	})
	if err != nil {
		return fmt.Errorf("Failed to send NICK: %s", err)
	}

	for {
		msg, err := c.conn.ReadMessage()
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

		msg, err := c.conn.ReadMessage()
		if err != nil {
			return err
		}

		if msg.Command == "PING" {
			message := irc.Message{Command: "PONG", Params: []string{msg.Params[0]}}
			err = c.conn.WriteMessage(message)
			if err != nil {
				return fmt.Errorf("Failed to send PONG: %s", err)
			}
			log.Printf("Sent PONG.")
		}

		if msg.Command == "ERROR" {
			// After sending QUIT, the server acknowledges it with an ERROR
			// command.
			if c.sentQUIT {
				log.Printf("Received QUIT acknowledgement. Closing connection.")
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

// Join joins a channel.
func (c *Conn) Join(name string) error {
	return c.conn.WriteMessage(irc.Message{
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

		err := c.conn.WriteMessage(irc.Message{
			Command: "PRIVMSG",
			Params:  []string{target, piece},
		})
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
	err := c.conn.WriteMessage(irc.Message{
		Command: "QUIT",
		Params:  []string{message},
	})
	if err == nil {
		c.sentQUIT = true
	}
	return err
}

// Oper sends an OPER command
func (c *Conn) Oper(name string, password string) error {
	return c.conn.WriteMessage(irc.Message{
		Command: "OPER",
		Params:  []string{name, password},
	})
}

// UserMode sends a MODE command.
func (c *Conn) UserMode(nick string, modes string) error {
	return c.conn.WriteMessage(irc.Message{
		Command: "MODE",
		Params:  []string{nick, modes},
	})
}
