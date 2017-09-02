// This is an IRC client that connects, joins a channel, sends a message, and
// quits. I intend to use it for event notifications.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/horgh/irc/client"
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

	c := client.New(*nick, *nick, *nick, *host, 6667, false)

	if err := c.Connect(); err != nil {
		log.Fatalf("Connection failure: %s", err)
	}

	if err := c.Register(); err != nil {
		log.Fatalf("Registration failure: %s", err)
	}

	if err := c.Join(*channel); err != nil {
		log.Fatalf("Join failure: %s", err)
	}

	if err := c.Message(*channel, *message); err != nil {
		log.Fatalf("Message failure: %s", err)
	}

	if err := c.Quit("Bye"); err != nil {
		log.Fatalf("Quit failure: %s", err)
	}

	if err := c.Loop(); err != nil {
		log.Fatalf("Loop reported failure: %s", err)
	}

	log.Printf("Done!")
}
