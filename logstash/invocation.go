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

	args     []string
	ioConfig string
	logDir   string
	logFile  io.ReadCloser
	tempDir  string
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
	configDir := filepath.Join(tempDir, "config")
	dataDir := filepath.Join(tempDir, "data")
	logDir := filepath.Join(tempDir, "log")
	pipelineDir := filepath.Join(tempDir, "pipeline.d")
	for _, dir := range []string{configDir, logDir, pipelineDir} {
		if err = os.Mkdir(dir, 0700); err != nil {
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

	if err := getPipelineConfigDir(pipelineDir, configs); err != nil {
		_ = logFile.Close()
		_ = os.RemoveAll(tempDir)
		return nil, err
	}

	// In Logstash 6+ you're not allowed to pass both the -e and
	// -f options so the invocation-specific input and output
	// configurations need to go in a file in the pipeline
	// directory. Generate a unique file for that purpose.
	ioConfigFile, err := getTempFileWithSuffix(pipelineDir, ".conf")
	if err != nil {
		_ = logFile.Close()
		_ = os.RemoveAll(tempDir)
		return nil, err
	}

	args := []string{
		"-w", // Make messages arrive in order.
		"1",
		"--debug",
		"-f",
		pipelineDir,
	}
	if logstashVersion.GTE(semver.MustParse("5.0.0")) {
		// Starting with Logstash 5.0 you don't configure
		// the path to the log file but the path to the log
		// directory.
		args = append(args, "-l", filepath.Dir(logfilePath))

		_, err = copyConfigFiles(logstashPath, configDir)
		if err != nil {
			_ = logFile.Close()
			_ = os.RemoveAll(tempDir)
			return nil, err
		}

		// We need to create a settings file to avoid warnings
		// (and possibly preventing Logstash from defaulting
		// to another file) but right now there's nothing to
		// put there. The various path settings that we need
		// to provide can just as well be passed as command
		// arguments.
		err := ioutil.WriteFile(filepath.Join(configDir, "logstash.yml"), []byte{}, 0644)
		if err != nil {
			_ = logFile.Close()
			_ = os.RemoveAll(tempDir)
			return nil, err
		}

		args = append(args, "--path.settings", configDir)
		args = append(args, "--path.data", dataDir)
	} else {
		args = append(args, "-l", logfilePath)
	}
	args = append(args, logstashArgs...)

	return &Invocation{
		LogstashPath: logstashPath,
		args:         args,
		ioConfig:     ioConfigFile,
		logDir:       logDir,
		logFile:      logFile,
		tempDir:      tempDir,
	}, nil
}

// Args returns a complete slice of Logstash command arguments for the
// given input and output plugin configuration strings.
func (inv *Invocation) Args(inputs string, outputs string) ([]string, error) {
	ioConfig := fmt.Sprintf("%s\n%s", inputs, outputs)
	if err := ioutil.WriteFile(inv.ioConfig, []byte(ioConfig), 0600); err != nil {
		return nil, err
	}
	return inv.args, nil
}

// Release releases any resources allocated by the struct.
func (inv *Invocation) Release() {
	inv.logFile.Close()
	_ = os.RemoveAll(inv.tempDir)
}

// copyConfigFiles copies the non-pipeline related configuration files
// from the Logstash distribution so that e.g. JVM and logging is set
// up properly even though we provide our own logstash.yml.
//
// The files to copy are either dug up from ../config relative to the
// Logstash executable or from /etc/logstash, where they're stored in
// the RPM/Debian case. If successful, the directory where the files
// were found is returned.
func copyConfigFiles(logstashPath string, configDir string) (string, error) {
	sourceDirs := []string{
		filepath.Clean(filepath.Join(filepath.Dir(logstashPath), "../config")),
		"/etc/logstash",
	}
	sourceFiles := []string{
		"jvm.options",
		"log4j2.properties",
	}
	return copyAllFiles(sourceDirs, sourceFiles, configDir)
}
