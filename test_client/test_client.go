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
		Host:  "127.0.0.1",
		Port:  6667,
		SSL:   false,
	}

	err := conn.Connect()
	if err != nil {
		log.Printf("Connection failure: %s", err.Error())
		os.Exit(1)
	}

	err = conn.Join("#test")
	if err != nil {
		log.Printf("Join failure: %s", err.Error())
		os.Exit(1)
	}

	conn.Loop()
	log.Printf("Done")
}
