/*
 * Make the connection oper up.
 */

package oper

import (
	"log"
	"summercat.com/irc"
)

func init() {
	irc.Hooks = append(irc.Hooks, Hook)
}

// Hook fires when an IRC message of some kind occurs.
// This can let us know whether to do anything or not.
func Hook(conn *irc.Conn, message irc.Message) {
	// RPL_WELCOME, Welcome
	if message.Command != "001" {
		return
	}
	log.Printf("Oper: have welcome")

	// Try to oper if we have both an oper name and password.
	operName, exists := conn.Config["oper-name"]
	if !exists {
		return
	}
	operPass, exists := conn.Config["oper-password"]
	if !exists {
		return
	}
	if len(operName) == 0 || len(operPass) == 0 {
		return
	}

	err := conn.Oper(operName, operPass)
	if err != nil {
		log.Printf("Unable to send OPER: %s", err)
	}
	log.Printf("Sent OPER")
}
