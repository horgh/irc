/*
 * Make the connection oper up.
 */

package oper

import (
	"log"

	"summercat.com/irc"
	"summercat.com/irc/client"
)

func init() {
	client.Hooks = append(client.Hooks, Hook)
}

// Hook fires when an IRC message of some kind occurs.
// This can let us know whether to do anything or not.
func Hook(conn *client.Conn, message irc.Message) {
	// RPL_WELCOME, Welcome
	if message.Command == "001" {
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
		return
	}

	// 381: RPL_YOUREOPER
	// Successful oper.
	// Apply user modes.
	if message.Command == "381" {
		err := sendUmode(conn)
		if err != nil {
			log.Printf("Problem sending MODE: %s", err)
		}
		return
	}
}

// sendUmode sends the oper umodes with the MODE command.
func sendUmode(conn *client.Conn) error {
	operUmodes, exists := conn.Config["oper-umodes"]
	if !exists {
		return nil
	}

	err := conn.UserMode(conn.ActualNick, operUmodes)
	if err != nil {
		return err
	}

	log.Printf("Sent MODE")
	return nil
}
