package main

import (
	"flag"
	"fmt"
	"time"
)

// Config holds the configuration settings for the application.
type Config struct {
	dir          string        // Directory where files are saved.
	listenAddr   string        // Port on which the server listens.
	readTimeout  time.Duration // Timeout for reading the request.
	writeTimeout time.Duration // Timeout for writing the response.
	idleTimeout  time.Duration // Timeout for keeping idle connections.
}

func (c Config) String() string {
	return fmt.Sprintf("Config{dir: %s, listenAddr: %s, readTimeout: %v, writeTimeout: %v, idleTimeout: %v}", c.dir, c.listenAddr, c.readTimeout, c.writeTimeout, c.idleTimeout)
}

// newConfig parses command-line flags and returns a Config instance.
func newConfig() Config {
	c := Config{}

	flag.StringVar(&c.dir, "dir", "/tmp", "A path to the directory where files are saved to.")
	flag.StringVar(&c.listenAddr, "listen-addr", ":5000", "The port to listen at.")
	flag.DurationVar(&c.readTimeout, "read-timeout", 15*time.Second, "Timeout for reading the request.")
	flag.DurationVar(&c.writeTimeout, "write-timeout", 15*time.Second, "Timeout for writing the response.")
	flag.DurationVar(&c.idleTimeout, "idle-timeout", 60*time.Second, "Timeout for keeping idle connections.")

	flag.Parse()

	return c
}
