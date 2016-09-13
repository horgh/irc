package irc

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
)

// Config holds data from a config file.
type Config map[string]string

// LoadConfig reads and parses a config file.
//
// Format:
// key=value
//
// # type comments permitted.
func LoadConfig(file string) (Config, error) {
	fh, err := os.Open(file)
	if err != nil {
		return Config{}, fmt.Errorf("Unable to open: %s: %s", file, err)
	}

	defer fh.Close()

	scanner := bufio.NewScanner(fh)

	config := Config{}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		pieces := strings.SplitN(line, "=", 2)
		if len(pieces) != 2 {
			return Config{}, fmt.Errorf("Invalid line: %s", scanner.Text())
		}

		key := strings.ToLower(strings.TrimSpace(pieces[0]))
		value := strings.TrimSpace(pieces[1])

		if len(key) == 0 {
			return Config{}, fmt.Errorf("Blank config key: %s", scanner.Text())
		}

		config[key] = value
		log.Printf("Config: %s = %s", key, value)
	}

	if scanner.Err() != nil {
		return Config{}, fmt.Errorf("Scanner error: %s", scanner.Err())
	}

	return config, nil
}
