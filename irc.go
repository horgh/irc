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

	// RW: Read/write handle to the connection
	RW *bufio.ReadWriter
}

// timeoutTime is how long we wait on network I/O.
const timeoutTime = 5 * time.Minute

// Read reads a message from the connection.
func (c Conn) Read() (string, error) {
	err := c.Conn.SetDeadline(time.Now().Add(timeoutTime))
	if err != nil {
		return "", fmt.Errorf("Unable to set deadline: %s", err)
	}

	line, err := c.RW.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("Unable to read: %s", err)
	}

	log.Printf("Read: %s", strings.TrimRight(line, "\r\n"))

	return line, nil
}

// Write writes a string to the connection
func (c Conn) Write(s string) error {
	err := c.Conn.SetDeadline(time.Now().Add(timeoutTime))
	if err != nil {
		return fmt.Errorf("Unable to set deadline: %s", err)
	}

	sz, err := c.RW.WriteString(s)
	if err != nil {
		return fmt.Errorf("Unable to write: %s", err)
	}

	if sz != len(s) {
		return fmt.Errorf("Short write")
	}

	err = c.RW.Flush()
	if err != nil {
		return fmt.Errorf("Flush error: %s", err)
	}

	log.Printf("Sent: %s", strings.TrimRight(s, "\r\n"))

	return nil
}
