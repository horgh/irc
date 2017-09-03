// A simple example client using the client package.
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

	c := client.New(*nick, *nick, *nick, *host, *port, *tls)

	if len(*configFile) > 0 {
		config, err := config.ReadStringMap(*configFile)
		if err != nil {
			log.Fatalf("Unable to load config: %s: %s", *configFile, err)
		}
		c.Config = config
	}

	if err := c.Connect(); err != nil {
		log.Fatalf("Connection failure: %s", err)
	}

	if err := c.Register(); err != nil {
		log.Fatalf("Registration failure: %s", err)
	}

	channels := strings.Split(*channel, ",")
	for _, ch := range channels {
		ch = strings.TrimSpace(ch)
		if len(ch) == 0 {
			continue
		}

		if err := c.Join(ch); err != nil {
			log.Fatalf("Join failure: %s", err)
		}
	}

	if err := c.Loop(); err != nil {
		log.Fatalf("Loop failure: %s", err)
	}

	log.Printf("Done")
}
