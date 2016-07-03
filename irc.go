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

	// connected: Whether currently connected or not
	connected bool

	// conn: The connection if we are actively connected.
	conn net.Conn

	// rw: Read/write handle to the connection
	rw *bufio.ReadWriter

	// actualNick: The nick we have if we are currently connected. The requested
	// nick may not always be available.
	actualNick string
}

// Connect attempts to initialize a connection to a server.
func (c *Conn) Connect() error {
	if c.TLS {
		conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port),
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
		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port))
		if err != nil {
			return err
		}
		c.conn = conn
		c.connected = true
	}

	c.rw = bufio.NewReadWriter(bufio.NewReader(c.conn), bufio.NewWriter(c.conn))

	err := c.init()
	if err != nil {
		return err
	}

	return nil
}

// init runs connection initiation (nick, etc)
func (c *Conn) init() error {
	err := c.write(fmt.Sprintf("NICK %s\r\n", c.Nick))
	if err != nil {
		return fmt.Errorf("Failed to send NICK: %s", err.Error())
	}

	err = c.write(fmt.Sprintf("USER %s 0 * :%s\r\n", c.Ident, c.Name))
	if err != nil {
		return fmt.Errorf("Failed to send NICK: %s", err.Error())
	}

	for {
		line, err := c.rw.ReadString('\n')
		if err != nil {
			return err
		}

		log.Printf("Read line [%s]", strings.TrimSpace(line))

		if !strings.HasPrefix(line, ":") {
			continue
		}

		var server string
		var code int
		_, err = fmt.Sscanf(line, ":%s %d ", &server, &code)
		if err == nil {
			if code == 1 {
				return nil
			}
			return fmt.Errorf("No welcome found")
		}
	}
}

// Loop enters a loop reading from the server.
// We maintain the IRC connection.
// Hook events will fire.
func (c *Conn) Loop() error {
	for {
		line, err := c.rw.ReadString('\n')
		if err != nil {
			return err
		}

		log.Printf("Read line [%s]", line)

		// Respond to PING.
		if strings.HasPrefix(line, "PING ") {
			var server string
			_, err = fmt.Sscanf(line, "PING :%s", &server)
			if err != nil {
				return fmt.Errorf("Unable to parse PING")
			}

			err = c.write(fmt.Sprintf("PONG %s\r\n", server))
			if err != nil {
				return fmt.Errorf("Failed to send PONG: %s", err.Error())
			}
			log.Printf("Sent PONG.")
			continue
		}

		pieces := strings.Split(line, " ")

		if len(pieces) > 1 {
			// PRIVMSG: Fire hook
			if pieces[1] == "PRIVMSG" {
				err = c.privmsg(line, pieces)
				if err != nil {
					return err
				}
				continue
			}
		}
	}
}

// Join joins a channel.
func (c *Conn) Join(name string) error {
	return c.write(fmt.Sprintf("JOIN %s\r\n", name))
}

// Message sends a message.
func (c *Conn) Message(target string, message string) error {
	return c.write(fmt.Sprintf("PRIVMSG %s :%s\r\n", target, message))
}

// Quit sends a quit.
func (c *Conn) Quit(message string) error {
	return c.write(fmt.Sprintf("QUIT :%s\r\n", message))
}

// privmsg fires when a PRIVMSG is seen.
//
// It triggers any hook functions registered for privmsg.
func (c *Conn) privmsg(line string, pieces []string) error {
	log.Printf("privmsg: todo")
	return nil
}

// write writes a string to the connection
func (c *Conn) write(s string) error {
	sz, err := c.rw.WriteString(s)
	if err != nil {
		return err
	}

	if sz != len(s) {
		return fmt.Errorf("Short write")
	}

	err = c.rw.Flush()
	if err != nil {
		return err
	}

	log.Printf("Sent: [%s]", s)

	return nil
}
