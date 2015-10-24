// Copyright (c) 2015 Magnus Bäck <magnus@noun.se>

package logging

import (
	oplogging "github.com/op/go-logging"
)

const (
	LOG_MODULE = "logstash-filter-verifier"
)

var (
	log = oplogging.MustGetLogger(LOG_MODULE)
)

// MustGetLogger returns the application's default logger.
func MustGetLogger() *oplogging.Logger {
	return log
}

// SetLevel sets the desired log level for the default logger.
func SetLevel(level oplogging.Level) {
	oplogging.SetLevel(level, LOG_MODULE)
}
