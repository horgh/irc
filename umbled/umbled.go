// This program is an IRC bot that just sits in a channel and keeps the
// connection open. It is different in that it is more accepting of errors that
// over the lifetime of a connection. This is to try to see if such behaviour
// helps recovery. Specifically I want to see if it is possible for retrying to
// make any difference in keeping a connection alive in the face of an
// unreliable connection.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/horgh/irc"
	"github.com/horgh/irc/client"
)

func main() {
	args, err := getArgs()
	if err != nil {
		log.Fatal(err)
		return
	}

	conf, err := parseConfig(args.Config)
	if err != nil {
		log.Fatal(err)
	}

	client := client.New(conf.Nick, conf.Nick, conf.Nick, conf.ServerHost,
		conf.ServerPort, true)

	run(conf, client)
}

// Args hold command line arguments.
type Args struct {
	Config string
}

func getArgs() (*Args, error) {
	config := flag.String("conf", "", "Configuration file.")

	flag.Parse()

	if *config == "" {
		flag.PrintDefaults()
		return nil, fmt.Errorf("no config file provided")
	}

	return &Args{
		Config: *config,
	}, nil
}

// Config holds what we parsed from a config file.
type Config struct {
	Channels   []string
	Nick       string
	ServerHost string
	ServerPort int
}

func parseConfig(path string) (*Config, error) {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %s: %s", path, err)
	}

	reader := bytes.NewReader(buf)
	scanner := bufio.NewScanner(reader)

	m := map[string]string{}

	for scanner.Scan() {
		text := scanner.Text()
		text = strings.TrimSpace(text)
		if text == "" || text[0] == '#' {
			continue
		}

		pieces := strings.SplitN(text, "=", 2)
		if len(pieces) != 2 {
			return nil, fmt.Errorf("malformed line: %s", text)
		}

		key := strings.TrimSpace(pieces[0])
		value := strings.TrimSpace(pieces[1])

		if key == "" {
			return nil, fmt.Errorf("key is blank: %s", text)
		}

		// Allow value to be blank

		if _, ok := m[key]; ok {
			return nil, fmt.Errorf("duplicate key: %s", key)
		}

		m[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning file: %s: %s", path, err)
	}

	conf := &Config{}

	channelsRaw := strings.Split(m["channels"], ",")
	for _, c := range channelsRaw {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if c[0] != '#' {
			return nil, fmt.Errorf("malformed channel name: %s", c)
		}
		// We could look for dupes.
		conf.Channels = append(conf.Channels, c)
	}
	if len(conf.Channels) == 0 {
		return nil, fmt.Errorf("you must specify at least one channel")
	}

	if v := m["nick"]; v == "" {
		return nil, fmt.Errorf("you must specify a nick")
	}
	conf.Nick = m["nick"]

	if v := m["server-host"]; v == "" {
		return nil, fmt.Errorf("you must specify a server-host")
	}
	conf.ServerHost = m["server-host"]

	p, err := strconv.ParseInt(m["server-port"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid server-port: %s", err)
	}
	conf.ServerPort = int(p)

	return conf, nil
}

type state struct {
	lastActivity time.Time
	reports      []string
}

const (
	waitPeriod = 10 * time.Minute
)

func run(conf *Config, c *client.Client) {
	s := &state{}

	for {
		if !c.IsConnected() {
			if err := connect(conf, c, s); err != nil {
				log.Printf("error connecting: %s", err)
				_ = c.Close()
				continue
			}
			s.reports = nil
			continue
		}

		m, err := c.ReadMessage()
		if err != nil {
			if time.Now().Sub(s.lastActivity) < waitPeriod {
				log.Printf("error reading: %s, continuing", err)
				s.reports = append(s.reports, fmt.Sprintf("read problem (%s)",
					time.Now()))
				continue
			}
			log.Printf("error reading: %s, giving up", err)
			_ = c.Close()
			continue
		}

		if m.Command == "ERROR" {
			log.Printf("got ERROR: %s", m)
			_ = c.Close()
			continue
		}

		s.lastActivity = time.Now()

		if m.Command != "PING" {
			continue
		}

		if err := c.Pong(m); err != nil {
			if time.Now().Sub(s.lastActivity) < waitPeriod {
				s.reports = append(s.reports, fmt.Sprintf("pong problem (%s)",
					time.Now()))
				log.Printf("error ponging: %s, continuing", err)
				continue
			}
			log.Printf("error ponging: %s, giving up", err)
			_ = c.Close()
			continue
		}

		for _, ch := range conf.Channels {
			for _, r := range s.reports {
				if err := c.Message(ch, r); err != nil {
					if time.Now().Sub(s.lastActivity) < waitPeriod {
						s.reports = append(s.reports, fmt.Sprintf("message problem (%s)",
							time.Now()))
						log.Printf("error messaging: %s, continuing", err)
						continue
					}
					log.Printf("error messaging: %s, giving up", err)
					_ = c.Close()
					continue
				}
			}
		}
	}
}

func connect(conf *Config, c *client.Client, s *state) error {
	if err := c.Connect(); err != nil {
		return err
	}

	if err := c.Register(); err != nil {
		return err
	}

	for {
		m, err := c.ReadMessage()
		if err != nil {
			return err
		}

		if m.Command == irc.ReplyWelcome {
			c.SetRegistered()
			for _, ch := range conf.Channels {
				if err := c.Join(ch); err != nil {
					return fmt.Errorf("error joining channel: %s: %s", ch, err)
				}
			}
			s.lastActivity = time.Now()
			return nil
		}

		if m.Command == "ERROR" {
			return fmt.Errorf("received ERROR: %s", m)
		}
	}
}
