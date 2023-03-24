// Copyright (c) 2017 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	semver "github.com/Masterminds/semver/v3"
)

// Invocation represents an invocation of Logstash, including the
// details of the input arguments and how to capture its log
// output.
type Invocation struct {
	LogstashPath string

	args         []string
	ioConfigFile *os.File
	logDir       string
	logFile      io.ReadCloser
	tempDir      string
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
	for _, dir := range []string{configDir, dataDir, logDir, pipelineDir} {
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
	ioConfigFile, err := ioutil.TempFile(pipelineDir, "ioconfig.*.conf")
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
	if logstashVersion.GreaterThan(semver.MustParse("2.2.0")) || logstashVersion.Equal(semver.MustParse("2.2.0")) {
		// The ordering of messages within a batch is
		// non-deterministic as of Logstash 7, resulting in
		// flaky test results. This can be addressed with a
		// batch size of one (1). Although the problem hasn't
		// been seen with earlier Logstash releases, limit the
		// batch size everywhere we can for consistent
		// behavior across Logstash versions. It appears the
		// option was introduced in Logstash 2.2.
		args = append(args, "-b", "1")
	}
	if logstashVersion.GreaterThan(semver.MustParse("5.0.0")) || logstashVersion.Equal(semver.MustParse("5.0.0")) {
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
		err := ioutil.WriteFile(filepath.Join(configDir, "logstash.yml"), []byte{}, 0600)
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
		ioConfigFile: ioConfigFile,
		logDir:       logDir,
		logFile:      logFile,
		tempDir:      tempDir,
	}, nil
}

// Args returns a complete slice of Logstash command arguments for the
// given input and output plugin configuration strings.
func (inv *Invocation) Args(inputs string, outputs string) ([]string, error) {
	// We don't explicitly disallow multiple calls to Args(),
	// so make sure to overwrite any existing file contents every time.
	ioConfig := []byte(fmt.Sprintf("%s\n%s", inputs, outputs))
	if _, err := inv.ioConfigFile.WriteAt(ioConfig, 0); err != nil {
		return nil, err
	}
	if err := inv.ioConfigFile.Truncate(int64(len(ioConfig))); err != nil {
		return nil, err
	}
	return inv.args, nil
}

// Release releases any resources allocated by the struct.
func (inv *Invocation) Release() {
	inv.ioConfigFile.Close()
	inv.logFile.Close()
	_ = os.RemoveAll(inv.tempDir)
}

// copyConfigFiles copies the non-pipeline related configuration files
// from the Logstash distribution so that e.g. JVM and logging is set
// up properly even though we provide our own logstash.yml.
//
// The files to copy are either dug up from ../config or ../etc/logstash relative to the
// Logstash executable or from /etc/logstash, where they're stored in
// the RPM/Debian case. If successful, the directory where the files
// were found is returned.
func copyConfigFiles(logstashPath string, configDir string) (string, error) {
	sourceDirs := []string{
		filepath.Clean(filepath.Join(filepath.Dir(logstashPath), "../config")),
		filepath.Clean(filepath.Join(filepath.Dir(logstashPath), "../etc/logstash")),
		"/etc/logstash",
	}
	sourceFiles := []string{
		"jvm.options",
		"log4j2.properties",
	}
	return copyAllFiles(sourceDirs, sourceFiles, configDir)
}
