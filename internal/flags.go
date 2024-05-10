// Copyright (c) 2023, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
)

// ParseFlags to load config file and configure the log
// it returns a Config struct and a close function meant to be called once the program finishes
func ParseFlags() (*Config, func() error, error) {
	var logFilename string
	var cfg Config
	flag.Var(&cfg, "config", "Configuration file, can be provided multiple times")
	flag.StringVar(&logFilename, "log", "", "File to write logs to.")
	flag.Parse()

	var logOutput io.Writer = os.Stderr
	close := func() error { return nil }
	if logFilename != "" {
		logFd, err := os.OpenFile(logFilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return nil, nil, err
		}
		logOutput = logFd
		log.SetOutput(logOutput)
		close = logFd.Close
	}

	if !cfg.isIntialized {
		return nil, nil, fmt.Errorf("No valid configuration file provided.  The argument -config is mandatory.")
	}
	return &cfg, close, nil
}
