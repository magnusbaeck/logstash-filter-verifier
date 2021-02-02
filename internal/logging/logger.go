// Copyright (c) 2015-2016 Magnus Bäck <magnus@noun.se>

package logging

import (
	"os"

	oplogging "github.com/op/go-logging"
)

type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Warning(args ...interface{})
	Warningf(format string, args ...interface{})
}

const (
	logModule = "logstash-filter-verifier"
)

var (
	log     = oplogging.MustGetLogger(logModule)
	backend = oplogging.AddModuleLevel(oplogging.NewLogBackend(os.Stderr, "", 0))
)

// MustGetLogger returns the application's default logger.
func MustGetLogger() Logger {
	log.SetBackend(backend)
	return log
}

// SetLevel sets the desired log level for the default logger.
func SetLevel(loglevel string) {
	level, err := oplogging.LogLevel(loglevel)
	if err != nil {
		level = oplogging.WARNING
		log.Warning("invalid log level, fall back to WARNING")
	}
	backend.SetLevel(level, logModule)
}
