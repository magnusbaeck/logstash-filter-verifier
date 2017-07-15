// Copyright (c) 2017 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/blang/semver"
)

// Invocation represents an invocation of Logstash, including the
// details of the input arguments and how to capture its log
// output.
type Invocation struct {
	LogstashPath string

	args      []string
	configDir string
	logDir    string
	logFile   io.ReadCloser
	tempDir   string
}

// NewInvocation creates a new Invocation struct that contains all
// information required to start Logstash with a caller-selected set
// of configurations.
func NewInvocation(logstashPath string, logstashArgs []string, logstashVersion *semver.Version, configs ...string) (*Invocation, error) {
	if len(configs) == 0 {
		return nil, errors.New("must provide non-empty list of configuration file or directory names")
	}

	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, err
	}
	logDir := filepath.Join(tempDir, "log")
	configDir := filepath.Join(tempDir, "config")
	for _, dir := range []string{logDir, configDir} {
		if err = os.Mkdir(dir, 0755); err != nil {
			_ = os.RemoveAll(tempDir)
			return nil, err
		}
	}

	logfilePath := filepath.Join(logDir, "logstash-plain.log")
	logFile, err := os.Create(logfilePath)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, err
	}

	if err := getConfigFileDir(configDir, configs); err != nil {
		_ = logFile.Close()
		_ = os.RemoveAll(tempDir)
		return nil, err
	}

	args := []string{
		"-w", // Make messages arrive in order.
		"1",
		"--debug",
		"-f",
		configDir,
	}
	if logstashVersion.GTE(semver.MustParse("5.0.0")) {
		// Starting with Logstash 5.0 you don't configure
		// the path to the log file but the path to the log
		// directory.
		args = append(args, "-l", filepath.Dir(logfilePath))
	} else {
		args = append(args, "-l", logfilePath)
	}
	args = append(args, logstashArgs...)

	return &Invocation{
		LogstashPath: logstashPath,
		args:         args,
		configDir:    configDir,
		logDir:       logDir,
		logFile:      logFile,
		tempDir:      tempDir,
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
	_ = os.RemoveAll(inv.tempDir)
}
