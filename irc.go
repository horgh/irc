/*
 * Package irc provides functionality common to clients and servers.
 */

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
	// Conn: The connection if we are actively connected.
	Conn net.Conn

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
		Conn: conn,
		rw:   bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)),
	}
}

// Read reads a message from the connection.
func (c Conn) Read() (string, error) {
	err := c.Conn.SetDeadline(time.Now().Add(timeoutTime))
	if err != nil {
		return "", fmt.Errorf("Unable to set deadline: %s", err)
	}

	line, err := c.rw.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("Unable to read: %s", err)
	}

	log.Printf("Read: %s", strings.TrimRight(line, "\r\n"))

	if len(line) > maxLineLength {
		return "", fmt.Errorf("Line is too long.")
	}

	return line, nil
}

// Write writes a string to the connection
func (c Conn) Write(s string) error {
	if len(s) > maxLineLength {
		return fmt.Errorf("Line is too long.")
	}

	err := c.Conn.SetDeadline(time.Now().Add(timeoutTime))
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

// IsValidNick checks if a nickname is valid.
func IsValidNick(n string) bool {
	if len(n) == 0 {
		return false
	}

	// TODO: Implement
	return true
}

// IsValidUser checks if a user (USER command) is valid
func IsValidUser(u string) bool {
	if len(u) == 0 {
		return false
	}

	// TODO: Implement
	return true
}

// CanonicalizeNick converts the given nick to its canonical representation
// (which must be unique).
func CanonicalizeNick(n string) string {
	return strings.ToLower(n)
}

// CanonicalizeChannel converts the given channel to its canonical
// representation (which must be unique).
func CanonicalizeChannel(c string) string {
	return strings.ToLower(c)
}
