/*
 * This is an IRC client that connects, joins a channel, sends a message, and
 * quits. I intend to use it for event notifications.
 */

package main

import (
	"flag"
	"log"
	"os"

	"summercat.com/irc/client"
)

func main() {
	nick := flag.String("nick", "", "Nickname")
	host := flag.String("host", "", "IRC server hostname to connect to.")
	channel := flag.String("channel", "", "Channel to join and message.")
	message := flag.String("message", "", "Message to send.")

	flag.Parse()

	if len(*nick) == 0 {
		log.Print("You must specify a nickname.")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if len(*host) == 0 {
		log.Print("You must specify a host.")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if len(*channel) == 0 {
		log.Print("You must specify a channel.")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if len(*message) == 0 {
		log.Print("You must specify a message.")
		flag.PrintDefaults()
		os.Exit(1)
	}

	conn := client.Conn{
		Nick:  *nick,
		Name:  *nick,
		Ident: *nick,
		Host:  *host,
		Port:  6667,
		TLS:   false,
	}

	err := conn.Connect()
	if err != nil {
		log.Printf("Connection failure: %s", err.Error())
		os.Exit(1)
	}

	err = conn.Join(*channel)
	if err != nil {
		log.Printf("Join failure: %s", err.Error())
		os.Exit(1)
	}

	err = conn.Message(*channel, *message)
	if err != nil {
		log.Printf("Message failure: %s", err.Error())
		os.Exit(1)
	}

	err = conn.Quit("Bye")
	if err != nil {
		log.Printf("Quit failure: %s", err.Error())
		os.Exit(1)
	}
	err = conn.Loop()

	if err != nil {
		log.Printf("Loop reported failure: %s", err)
		os.Exit(1)
	}

	log.Printf("Done!")
}
