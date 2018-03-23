// Copyright (c) 2016 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/magnusbaeck/logstash-filter-verifier/testhelpers"
)

type closeableBuffer struct {
	bytes.Buffer
}

func newCloseableBuffer(s string) *closeableBuffer {
	return &closeableBuffer{
		Buffer: *bytes.NewBufferString(s),
	}
}

func (cb *closeableBuffer) Close() error {
	return nil
}

func TestProcess(t *testing.T) {
	cases := []struct {
		// Input
		command string
		args    []string
		input   string
		env     []string

		// Command behavior
		output string
		log    string

		// Expected outcome
		result   Result
		starterr error
		waiterr  error
	}{
		// Successful run with no input or output.
		{
			command: "true",
			args:    []string{},
			input:   "",
			env:     []string{},
			output:  "",
			log:     "",
			result: Result{
				Success: true,
				Events:  []Event{},
				Log:     "",
				Output:  "",
			},
			starterr: nil,
			waiterr:  nil,
		},
		// Child returns non-zero exit code.
		{
			command: "false",
			args:    []string{},
			input:   "",
			env:     []string{},
			output:  "",
			log:     "",
			result: Result{
				Success: false,
				Events:  []Event{},
				Log:     "",
				Output:  "",
			},
			starterr: nil,
			waiterr:  &exec.ExitError{},
		},
		// Environment variables are correctly set.
		{
			command: "sh",
			args: []string{
				"-c",
				`test "$TZ" = "UTC"`,
			},
			input:  "",
			env:    []string{"TZ=UTC"},
			output: "",
			log:    "",
			result: Result{
				Success: true,
				Events:  []Event{},
				Log:     "",
				Output:  "",
			},
			starterr: nil,
			waiterr:  nil,
		},
		// Output to stdout is captured.
		{
			command: "sh",
			args: []string{
				"-c",
				`echo "hello"`,
			},
			input:  "",
			output: "",
			log:    "",
			result: Result{
				Success: true,
				Events:  []Event{},
				Log:     "",
				Output:  "hello\n",
			},
			starterr: nil,
			waiterr:  nil,
		},
		// Output to stderr is captured.
		{
			command: "sh",
			args: []string{
				"-c",
				`echo "hello" >&2`,
			},
			input:  "",
			output: "",
			log:    "",
			result: Result{
				Success: true,
				Events:  []Event{},
				Log:     "",
				Output:  "hello\n",
			},
			starterr: nil,
			waiterr:  nil,
		},
		// Successful child returns log output.
		{
			command: "true",
			args:    []string{},
			input:   "",
			env:     []string{},
			output:  "",
			log:     "log output",
			result: Result{
				Success: true,
				Events:  []Event{},
				Log:     "log output",
				Output:  "",
			},
			starterr: nil,
			waiterr:  nil,
		},
		// Failed child returns log output.
		{
			command: "false",
			args:    []string{},
			input:   "",
			env:     []string{},
			output:  "",
			log:     "log output",
			result: Result{
				Success: false,
				Events:  []Event{},
				Log:     "log output",
				Output:  "",
			},
			starterr: nil,
			waiterr:  &exec.ExitError{},
		},
		// Events are captured.
		{
			command: "cat",
			args:    []string{},
			input:   "",
			env:     []string{},
			output:  `{"a": "b"}`,
			log:     "",
			result: Result{
				Success: true,
				Events: []Event{
					{
						"a": "b",
					},
				},
				Log:    "",
				Output: "\n",
			},
			starterr: nil,
			waiterr:  nil,
		},
		// Child process gets the expected input.
		{
			command: "grep",
			args:    []string{"string to search for"},
			input:   "this string doesn't contain what we look for",
			env:     []string{},
			output:  "",
			log:     "",
			result: Result{
				Success: false,
				Events:  []Event{},
				Log:     "",
				Output:  "",
			},
			starterr: nil,
			waiterr:  &exec.ExitError{},
		},
		// Broken JSON output is correctly reported.
		{
			command: "true",
			args:    []string{},
			input:   "",
			env:     []string{},
			output:  "this is not JSON",
			log:     "",
			result: Result{
				Success: false,
				Events:  []Event{},
				Log:     "",
				Output:  "",
			},
			starterr: nil,
			waiterr:  BadLogstashOutputError{},
		},
		// A bad executable path results in the correct error.
		{
			command: "/file/does/not/exist",
			args:    []string{},
			input:   "",
			env:     []string{},
			output:  "",
			log:     "",
			result: Result{
				Success: false,
				Events:  []Event{},
				Log:     "",
				Output:  "",
			},
			starterr: &os.PathError{},
			waiterr:  nil,
		},
	}
	for i, c := range cases {
		p, err := newProcessWithArgs(c.command, c.args, c.env)
		if err != nil {
			t.Fatalf("Test %d: Unexpected error when creating process struct: %s", i, err)
		}
		defer p.Release()
		p.inv = &Invocation{
			LogstashPath: c.command,
			args:         c.args,
			logFile:      newCloseableBuffer(c.log),
		}
		defer p.inv.Release()
		p.output = newCloseableBuffer(c.output)

		err = p.Start()
		testhelpers.CompareErrors(t, i, c.starterr, err)
		if err != nil {
			break
		}

		// The processes we run are very short-lived and if
		// they terminate too quickly this write operation
		// could fail with a "broken pipe" error, so we ignore
		// any errors.
		_, _ = p.Input.Write([]byte(c.input + "\n"))
		_ = p.Input.Close()

		result, err := p.Wait()
		testhelpers.CompareErrors(t, i, c.waiterr, err)

		if result.Success != c.result.Success {
			t.Errorf("Test %d: Expected Success=%v, got Success=%v instead", i, c.result.Success, result.Success)
		}

		expectedEvents := fmt.Sprintf("%#v", c.result.Events)
		actualEvents := fmt.Sprintf("%#v", result.Events)
		if expectedEvents != actualEvents {
			t.Errorf("Test %d:\nExpected events:\n%q\nGot:\n%q", i, expectedEvents, actualEvents)
		}

		if result.Log != c.result.Log {
			t.Errorf("Test %d:\nExpected log output:\n%q\nGot:\n%q", i, c.result.Log, result.Log)
		}

		if result.Output != c.result.Output {
			t.Errorf("Test %d:\nExpected stdout/stderr:\n%q\nGot:\n%q", i, c.result.Output, result.Output)
		}
	}
}
