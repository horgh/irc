/*
 * Provide IRC functionality
 */

package irc

import (
	"bufio"
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

	connected bool
	conn      net.Conn
	rw        *bufio.ReadWriter
}

// Connect attempts to initialize a connection to a server.
func (c *Conn) Connect() error {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port))
	if err != nil {
		return err
	}

	c.connected = true
	c.conn = conn

	c.rw = bufio.NewReadWriter(bufio.NewReader(c.conn), bufio.NewWriter(c.conn))

	err = c.init()
	if err != nil {
		return err
	}

	return nil
}

// init runs connection initiation (nick, etc)
func (c *Conn) init() error {
	_, err := c.rw.WriteString(fmt.Sprintf("NICK %s\r\n", c.Nick))
	if err != nil {
		return fmt.Errorf("Failed to send NICK: %s", err.Error())
	}

	_, err = c.rw.WriteString(fmt.Sprintf("USER %s 0 * :%s\r\n", c.Ident,
		c.Name))
	if err != nil {
		return fmt.Errorf("Failed to send NICK: %s", err.Error())
	}

	err = c.rw.Flush()
	if err != nil {
		return err
	}

	for {
		line, err := c.rw.ReadString('\n')
		if err != nil {
			return err
		}

		log.Printf("Read line [%s]", line)

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

// Loop enters a connection loop
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

			_, err = c.rw.WriteString(fmt.Sprintf("PONG %s\r\n", server))
			if err != nil {
				return fmt.Errorf("Failed to send PONG: %s", err.Error())
			}

			err = c.rw.Flush()
			if err != nil {
				return err
			}

			log.Printf("Sent PONG.")
		}
	}
}

// Join joins a channel
func (c *Conn) Join(name string) error {
	_, err := c.rw.WriteString(fmt.Sprintf("JOIN %s\r\n", name))
	if err != nil {
		return fmt.Errorf("Failed to send JOIN: %s", err.Error())
	}

	err = c.rw.Flush()
	if err != nil {
		return err
	}

	return nil
}
