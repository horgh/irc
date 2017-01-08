/*
 * A simple example client using the irc package.
 */

package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/horgh/config"
	"github.com/horgh/irc/client"
	_ "github.com/horgh/irc/duckduckgo"
	_ "github.com/horgh/irc/oper"
	_ "github.com/horgh/irc/recordips"
)

func main() {
	log.SetFlags(0)

	nick := flag.String("nick", "", "Nickname to use. We'll use this for name and ident too.")
	host := flag.String("host", "", "Host to connect to.")
	port := flag.Int("port", 6667, "Port to connect to on the host.")
	tls := flag.Bool("tls", false, "Whether to connect with TLS.")
	channel := flag.String("channel", "", "Channel to join. For multiple, separate them by commas.")
	configFile := flag.String("config", "", "Config file to load. Optional.")

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

	conn := client.Conn{
		Nick:  *nick,
		Name:  *nick,
		Ident: *nick,
		Host:  *host,
		Port:  *port,
		TLS:   *tls,
	}

	if len(*configFile) > 0 {
		config, err := config.ReadStringMap(*configFile)
		if err != nil {
			log.Fatalf("Unable to load config: %s: %s", *configFile, err)
		}
		conn.Config = config
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
