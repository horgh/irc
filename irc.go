/*
 * Provide IRC functionality
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

type Conn struct {
	Nick  string
	Name  string
	Ident string
	Host  string
	Port  int
	SSL   bool

	connected  bool
	conn       net.Conn
	rw         *bufio.ReadWriter
	actualNick string
}

// Connect attempts to initialize a connection to a server.
func (c *Conn) Connect() error {
	if c.SSL {
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

// Loop enters a loop. We maintain the IRC connection. Hook events
// will fire.
func (c *Conn) Loop() error {
	for {
		line, err := c.rw.ReadString('\n')
		if err != nil {
			return err
		}

		log.Printf("Read line [%s]", line)

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

// Join joins a channel
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
