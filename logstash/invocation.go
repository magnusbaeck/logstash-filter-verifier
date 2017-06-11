// Copyright (c) 2017 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// Invocation represents an invocation of Logstash, including the
// details of the input arguments and how to capture its log
// output.
type Invocation struct {
	LogstashPath string

	args      []string
	configDir string
	logFile   io.ReadCloser
}

// NewInvocation creates a new Invocation struct that contains all
// information required to start Logstash with a caller-selected set
// of configurations.
func NewInvocation(logstashPath string, logstashArgs []string, configs ...string) (*Invocation, error) {
	if len(configs) == 0 {
		return nil, errors.New("must provide non-empty list of configuration file or directory names")
	}

	logFile, err := newDeletedTempFile("", "")
	if err != nil {
		return nil, err
	}

	configDir, err := getConfigFileDir(configs)
	if err != nil {
		_ = logFile.Close()
		return nil, err
	}

	args := []string{
		"-w", // Make messages arrive in order.
		"1",
		"--debug",
		"-f",
		configDir,
		"-l",
		logFile.Name(),
	}
	args = append(args, logstashArgs...)

	return &Invocation{
		LogstashPath: logstashPath,
		args:         args,
		configDir:    configDir,
		logFile:      logFile,
	}, nil
}

// Args returns a complete slice of Logstash command arguments for the
// given input and output plugin configuration strings.
func (inv *Invocation) Args(inputs string, outputs string) []string {
	return append(inv.args, "-e", fmt.Sprintf("%s %s", inputs, outputs))
}

// Release releases any resources allocated by the struct.
func (inv *Invocation) Release() {
	inv.logFile.Close()
	_ = os.RemoveAll(inv.configDir)
}
