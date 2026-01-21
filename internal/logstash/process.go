// Copyright (c) 2015-2016 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
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
	inv    *Invocation
	output io.ReadCloser
	stdio  io.Reader
}

// NewProcess prepares for the execution of a new Logstash process but
// doesn't actually start it. logstashPath is the path to the Logstash
// executable (typically /opt/logstash/bin/logstash), inputCodec is
// the desired codec for the stdin input and inputType the value of
// the "type" field for ingested events. The configs parameter is
// one or more configuration files containing Logstash filters.
func NewProcess(inv *Invocation, inputCodec string, fields FieldSet, keptEnvVars []string) (*Process, error) {
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

	fieldHash, err := fields.LogstashHash()
	if err != nil {
		_ = outputFile.Close()
		return nil, err
	}
	inputs := fmt.Sprintf("input { stdin { codec => %s add_field => %s } }", inputCodec, fieldHash)
	outputs := fmt.Sprintf("output { file { path => %q codec => \"json_lines\" } }", outputFile.Name())

	env := GetLimitedEnvironment(os.Environ(), keptEnvVars)
	args, err := inv.Args(inputs, outputs)
	if err != nil {
		_ = outputFile.Close()
		return nil, err
	}
	p, err := newProcessWithArgs(inv.LogstashPath, args, env)
	if err != nil {
		_ = outputFile.Close()
		return nil, err
	}
	p.output = outputFile
	p.inv = inv
	return p, nil
}

// newProcessWithArgs performs the non-Logstash specific low-level
// actions of preparing to spawn a child process, making it easier to
// test the code in this package.
func newProcessWithArgs(command string, args []string, env []string) (*Process, error) {
	c := exec.Command(command, args...)
	c.Env = env

	// Save the process's stdout and stderr since an early startup
	// failure (e.g. JVM issues) will get dumped there and not in
	// the log file.
	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = &b

	inputPipe, err := c.StdinPipe()
	if err != nil {
		return nil, err
	}

	return &Process{
		Input: inputPipe,
		child: c,
		stdio: &b,
	}, nil
}

// Start starts a Logstash child process with the previously supplied
// configuration.
func (p *Process) Start() error {
	log.Infof("Starting %q with args %q.", p.child.Path, p.child.Args[1:])
	return p.child.Start()
}

// Wait blocks until the started Logstash process terminates and
// returns the result of the execution.
func (p *Process) Wait_(eventFunc func(r io.Reader) ([]Event, error)) (*Result, error) {
	if p.child.Process == nil {
		return nil, errors.New("can't wait on an unborn process")
	}
	log.Debugf("Waiting for child with pid %d to terminate.", p.child.Process.Pid)

	waiterr := p.child.Wait()

	// Save the log output regardless of whether the child process
	// succeeded or not.
	logbuf, logerr := io.ReadAll(p.inv.logFile)
	if logerr != nil {
		// Log this weird error condition but don't let it
		// fail the function. We don't care about the log
		// contents unless Logstash fails, in which we'll
		// report that problem anyway.
		log.Errorf("Error reading the Logstash logfile: %s", logerr)
	}
	outbuf, _ := io.ReadAll(p.stdio)

	result := Result{
		Events:  []Event{},
		Log:     string(logbuf),
		Output:  string(outbuf),
		Success: waiterr == nil,
	}
	if waiterr != nil {
		return &result, waiterr
	}

	var err error
	result.Events, err = eventFunc(p.output)
	result.Success = err == nil
	return &result, err
}

func (p *Process) WaitAndPrint() (*Result, error) {
	return p.Wait_(formatAndPrintEvents)
}

func (p *Process) WaitAndRead() (*Result, error) {
	return p.Wait_(readEvents)
}

// Release frees all allocated resources connected to this process.
func (p *Process) Release() {
	_ = p.output.Close()
}
