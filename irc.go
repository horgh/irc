// Package irc provides functionality common to clients and servers.
package irc

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

// Conn is a connection to a client/server.
type Conn struct {
	// conn: The connection if we are actively connected.
	conn net.Conn

	// rw: Read/write handle to the connection
	rw *bufio.ReadWriter
}

// timeoutTime is how long we wait on network I/O.
const timeoutTime = 5 * time.Minute

// From RFC 2812 section 2.3. It includes CRLF.
const maxLineLength = 512

// NewConn initializes a Conn struct
func NewConn(conn net.Conn) Conn {
	return Conn{
		conn: conn,
		rw:   bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)),
	}
}

// Close closes the underlying connection
func (c Conn) Close() error {
	return c.conn.Close()
}

// RemoteAddr returns the remote network address.
func (c Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// read reads a line from the connection.
func (c Conn) read() (string, error) {
	err := c.conn.SetDeadline(time.Now().Add(timeoutTime))
	if err != nil {
		return "", fmt.Errorf("Unable to set deadline: %s", err)
	}

	line, err := c.rw.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("Unable to read: %s", err)
	}

	log.Printf("Read: %s", strings.TrimRight(line, "\r\n"))

	return line, nil
}

// ReadMessage reads a line from the connection and parses it as an IRC message.
func (c Conn) ReadMessage() (Message, error) {
	buf, err := c.read()
	if err != nil {
		return Message{}, fmt.Errorf("Unable to read: %s", err)
	}

	if len(buf) > maxLineLength {
		return Message{}, fmt.Errorf("Line is too long.")
	}

	m, err := parseMessage(buf)
	if err != nil {
		return Message{}, fmt.Errorf("Unable to parse message: %s: %s", buf, err)
	}

	return m, nil
}

// write writes a string to the connection
func (c Conn) write(s string) error {
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

	log.Printf("Sent: %s", strings.TrimRight(s, "\r\n"))

	return nil
}

// WriteMessage writes an IRC message to the connection.
func (c Conn) WriteMessage(m Message) error {
	buf, err := m.Encode()
	if err != nil {
		return fmt.Errorf("Unable to encode message: %s", err)
	}

	return c.write(buf)
}

// IsValidNick checks if a nickname is valid.
func IsValidNick(n string) bool {
	if len(n) == 0 {
		return false
	}

	// TODO: For now I accept only a-z, 0-9, or _. RFC is more lenient.
	for _, char := range n {
		if char >= 'a' && char <= 'z' {
			continue
		}

		if char >= '0' && char <= '9' {
			continue
		}

		if char == '_' {
			continue
		}

		return false
	}

	return true
}

// IsValidUser checks if a user (USER command) is valid
func IsValidUser(u string) bool {
	if len(u) == 0 {
		return false
	}

	// TODO: For now I accept only a-z or 0-9. RFC is more lenient.
	for _, char := range u {
		if char >= 'a' && char <= 'z' {
			continue
		}

		if char >= '0' && char <= '9' {
			continue
		}

		return false
	}

	return true
}

// IsValidChannel checks a channel name for validity.
//
// You should canonicalize it before using this function.
func IsValidChannel(c string) bool {
	if len(c) == 0 {
		return false
	}

	// TODO: I accept only a-z or 0-9 as valid characters right now. RFC
	//   accepts more.
	for i, char := range c {
		if i == 0 {
			// TODO: I only allow # channels right now.
			if char == '#' {
				continue
			}
			return false
		}

		if char >= 'a' && char <= 'z' {
			continue
		}

		if char >= '0' && char <= '9' {
			continue
		}

		return false
	}

	return true
}

// CanonicalizeNick converts the given nick to its canonical representation
// (which must be unique).
//
// Note: We don't check validity or strip whitespace.
func CanonicalizeNick(n string) string {
	return strings.ToLower(n)
}

// CanonicalizeChannel converts the given channel to its canonical
// representation (which must be unique).
//
// Note: We don't check validity or strip whitespace.
func CanonicalizeChannel(c string) string {
	return strings.ToLower(c)
}
