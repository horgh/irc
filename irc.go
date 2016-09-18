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

// MaxLineLength is the maximum protocol message line length.
// From RFC 2812 section 2.3. It includes CRLF.
const MaxLineLength = 512

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
		return "", err
	}

	log.Printf("Read: %s", strings.TrimRight(line, "\r\n"))

	return line, nil
}

// ReadMessage reads a line from the connection and parses it as an IRC message.
func (c Conn) ReadMessage() (Message, error) {
	buf, err := c.read()
	if err != nil {
		return Message{}, err
	}

	if len(buf) > MaxLineLength {
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
		return err
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
