/*
 * A test IRC client
 */

package main

import (
	"log"
	"os"
	"summercat.com/irc"
)

func main() {
	conn := irc.Conn{
		Nick:  "EvilHamada",
		Name:  "Evil Hamada",
		Ident: "evilhamada",
		Host:  "192.168.3.2",
		Port:  7000,
		TLS:   true,
	}

	err := conn.Connect()
	if err != nil {
		log.Printf("Connection failure: %s", err.Error())
		os.Exit(1)
	}

	err = conn.Join("#newhell")
	if err != nil {
		log.Printf("Join failure: %s", err.Error())
		os.Exit(1)
	}

	err = conn.Loop()
	if err != nil {
		log.Printf("Loop failure: %s", err)
		os.Exit(1)
	}

	log.Printf("Done")
}
