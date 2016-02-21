// Copyright (c) 2015-2016 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
)

// Process represents the invocation and execution of a Logstash child
// process that emits JSON events from the input and filter
// configuration files supplied by the caller.
type Process struct {
	// Input will be connected to the stdin stream of the started
	// Logstash process. Make sure to close it when all data has
	// been written so that the process will terminate.
	Input io.WriteCloser

	child  *exec.Cmd
	log    io.ReadCloser
	output io.ReadCloser
	stdio  io.Reader
}

// NewProcess prepares for the execution of a new Logstash process but
// doesn't actually start it. logstashPath is the path to the Logstash
// executable (typically /opt/logstash/bin/logstash), inputCodec is
// the desired codec for the stdin input and inputType the value of
// the "type" field for ingested events. The configs parameter is
// one or more configuration files containing Logstash filters.
func NewProcess(logstashPath, inputCodec string, fields FieldSet, configs ...string) (*Process, error) {
	if len(configs) == 0 {
		return nil, errors.New("Must provide non-empty list of configuration file or directory names.")
	}

	// Unfortunately Logstash doesn't make it easy to just read
	// events from a stdout-connected pipe and the log from a
	// stderr-connected pipe. Stdout can contain other garbage (at
	// the very least "future logs will be sent to ...") and error
	// messages could very well be sent there too. Mitigate by
	// having Logstash write output logs to a temporary file and
	// its own logs to a different temporary file.
	outputFile, err := newDeletedTempFile("", "")
	if err != nil {
		return nil, err
	}
	logFile, err := newDeletedTempFile("", "")
	if err != nil {
		_ = outputFile.Close()
		return nil, err
	}

	fieldHash, err := fields.LogstashHash()
	if err != nil {
		return nil, err
	}
	args := []string{
		"-w", // Make messages arrive in order.
		"1",
		"--debug",
		"-e",
		fmt.Sprintf(
			"input { stdin { codec => %q add_field => %s } } "+
				"output { file { path => %q codec => \"json_lines\" } }",
			inputCodec, fieldHash, outputFile.Name()),
		"--log",
		logFile.Name(),
	}
	for _, c := range configs {
		args = append(args, "--config")
		args = append(args, c)
	}
	c := exec.Command(logstashPath, args...)

	// Save the process's stdout and stderr since an early startup
	// failure (e.g. JVM issues) will get dumped there and not in
	// the log file.
	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = &b

	// The test cases must be written to be stable and independent
	// of the current timezone to there's no risk of a @timestamp
	// mismatch just because we've gone into daylight savings time.
	c.Env = []string{
		"TZ=UTC",
	}

	inputPipe, err := c.StdinPipe()
	if err != nil {
		_ = outputFile.Close()
		_ = logFile.Close()
		return nil, err
	}

	return &Process{
		Input:  inputPipe,
		child:  c,
		output: outputFile,
		log:    logFile,
		stdio:  &b,
	}, nil
}

// Start starts a Logstash child process with the previously supplied
// configuration.
func (p *Process) Start() error {
	log.Info("Starting %q with args %q.", p.child.Path, p.child.Args[1:])
	return p.child.Start()
}

// Wait blocks until the started Logstash process terminates and
// returns the result of the execution.
func (p *Process) Wait() (*Result, error) {
	if p.child.Process == nil {
		return nil, errors.New("Can't wait on an unborn process.")
	}
	log.Debug("Waiting for child with pid %d to terminate.", p.child.Process.Pid)

	waiterr := p.child.Wait()

	// Save the log output regardless of whether the child process
	// succeeded or not.
	logbuf, logerr := ioutil.ReadAll(p.log)
	if logerr != nil {
		// Log this weird error condition but don't let it
		// fail the function. We don't care about the log
		// contents unless Logstash fails, in which we'll
		// report that problem anyway.
		log.Error("Error reading the Logstash logfile: %s", logerr.Error())
	}
	outbuf, _ := ioutil.ReadAll(p.stdio)

	result := Result{
		Log:     string(logbuf),
		Output:  string(outbuf),
		Success: waiterr == nil,
	}
	if waiterr != nil {
		return &result, waiterr
	}

	scanner := bufio.NewScanner(p.output)
	for scanner.Scan() {
		var event Event
		err := json.Unmarshal([]byte(scanner.Text()), &event)
		if err != nil {
			return &result, fmt.Errorf("Logstash succeeded, but this output line couldn't be parsed as JSON: %s", scanner.Text())
		}
		result.Events = append(result.Events, event)
	}
	return &result, scanner.Err()
}

// Release frees all allocated resources connected to this process.
func (p *Process) Release() {
	_ = p.output.Close()
	_ = p.log.Close()
}
