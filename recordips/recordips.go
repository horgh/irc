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
	"fmt"
	"log"
	"strings"
	"time"

	"summercat.com/iptables-manage/cidrlist"
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

	comment := fmt.Sprintf("%s @ %s", nick, time.Now().Format(time.RFC1123))

	err := cidrlist.RecordIP(ipFile, ip, comment)
	if err != nil {
		log.Printf("record_connecting_ips: Unable to record IP: %s", err)
	}
}
