package main

import (
	"log"
	"os"
)

// debugf writes diagnostics to stderr when AppCloser runs with --debug
// (or APPCLOSER_DEBUG=1), e.g. from a terminal.
var debugOn = os.Getenv("APPCLOSER_DEBUG") == "1"

func debugf(format string, args ...any) {
	if debugOn {
		log.Printf(format, args...)
	}
}
