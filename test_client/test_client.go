/*
 * A simple example client using the irc package.
 */

package main

import (
	"flag"
	"log"
	"os"
	"strings"
	"summercat.com/irc"
	_ "summercat.com/irc/duckduckgo"
	_ "summercat.com/irc/oper"
	_ "summercat.com/irc/record_connecting_ips"
)

func main() {
	log.SetFlags(0)

	nick := flag.String("nick", "", "Nickname to use. We'll use this for name and ident too.")
	host := flag.String("host", "", "Host to connect to.")
	port := flag.Int("port", 6667, "Port to connect to on the host.")
	tls := flag.Bool("tls", false, "Whether to connect with TLS.")
	channel := flag.String("channel", "", "Channel to join. For multiple, separate them by commas.")
	config := flag.String("config", "", "Config file to load. Optional.")

	flag.Parse()

	if len(*nick) == 0 {
		log.Printf("You must provide a nick.")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if len(*host) == 0 {
		log.Printf("You must provide a host.")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if len(*channel) == 0 {
		log.Printf("You must provide a channel.")
		flag.PrintDefaults()
		os.Exit(1)
	}

	conn := irc.Conn{
		Nick:  *nick,
		Name:  *nick,
		Ident: *nick,
		Host:  *host,
		Port:  *port,
		TLS:   *tls,
	}

	if len(*config) > 0 {
		err := conn.LoadConfig(*config)
		if err != nil {
			log.Fatalf("Unable to load config: %s: %s", *config, err)
		}
	}

	err := conn.Connect()
	if err != nil {
		log.Printf("Connection failure: %s", err.Error())
		os.Exit(1)
	}

	channels := strings.Split(*channel, ",")
	for _, c := range channels {
		c = strings.TrimSpace(c)
		if len(c) == 0 {
			continue
		}

		err = conn.Join(c)
		if err != nil {
			log.Printf("Join failure: %s", err.Error())
			os.Exit(1)
		}
	}

	err = conn.Loop()
	if err != nil {
		log.Printf("Loop failure: %s", err)
		os.Exit(1)
	}

	log.Printf("Done")
}
