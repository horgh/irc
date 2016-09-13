/*
 * Package recordips makes a client watch for user connection notices (as
 * operator).
 *
 * Record each IP to a file (if it is not present), along with the nick and
 * date.
 *
 * My use case is to add connecting IPs to a firewall rule.
 */

package recordips

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"summercat.com/irc"
	"summercat.com/irc/client"
)

func init() {
	client.Hooks = append(client.Hooks, Hook)
}

// Hook fires when an IRC message of some kind occurs.
// We look for CLICONN notices and record the IP.
// The notices look like:
// :irc.example.com NOTICE * :*** Notice -- CLICONN will will example.com 192.168.1.2 opers will 192.168.1.2 0 will
// Note this is likely ircd-ratbox specific.
func Hook(conn *client.Conn, message irc.Message) {
	if message.Command != "NOTICE" {
		return
	}

	// 2 parameters. * and the full notice as a single parameter.
	if len(message.Params) != 2 {
		return
	}

	noticePieces := strings.Fields(message.Params[1])

	if len(noticePieces) < 8 {
		return
	}

	if noticePieces[3] != "CLICONN" {
		return
	}

	ipFile, exists := conn.Config["record-ip-file"]
	if !exists {
		return
	}

	nick := noticePieces[4]
	ip := noticePieces[7]

	err := recordIP(ipFile, nick, ip)
	if err != nil {
		log.Printf("record_connecting_ips: Unable to record IP: %s", err)
	}
}

// recordIP records the IP to the IP file.
//
// The format for the IP is a line by itself containing:
// ip/32
//
// We do not write anything to the file if the IP is already present.
func recordIP(ipFile string, nick string, ip string) error {
	alreadyRecorded, err := ipIsInFile(ipFile, ip)
	if err != nil {
		return fmt.Errorf("Unable to check if IP is in file: %s", err)
	}

	if alreadyRecorded {
		return nil
	}

	fh, err := os.OpenFile(ipFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("Unable to open: %s: %s", ipFile, err)
	}

	defer fh.Close()

	output := fmt.Sprintf("# %s @ %s\n%s/32\n", nick,
		time.Now().Format(time.RFC1123), ip)

	sz, err := fh.WriteString(output)
	if err != nil {
		return fmt.Errorf("Unable to write: %s", err)
	}

	if sz != len(output) {
		return fmt.Errorf("Short write")
	}

	return nil
}

// ipIsInFile checks if the IP is in the file.
//
// To be in the file, we say there must be a line like so:
// ip/32
func ipIsInFile(file string, ip string) (bool, error) {
	_, err := os.Lstat(file)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("Unable to stat file: %s", err)
	}

	fh, err := os.Open(file)
	if err != nil {
		return false, fmt.Errorf("Unable to open: %s", err)
	}

	defer fh.Close()

	scanner := bufio.NewScanner(fh)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == ip+"/32" {
			return true, nil
		}
	}

	if scanner.Err() != nil {
		return false, fmt.Errorf("Scanner error: %s", scanner.Err())
	}

	return false, nil
}
