// Copyright (c) 2016-2018 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	semver "github.com/Masterminds/semver/v3"
)

func TestParallelProcess(t *testing.T) {
	const testLine = "test line\n"

	fs := FieldSet{}
	ts, err := NewTestStream("Codec", fs, 5*time.Second)
	if err != nil {
		t.Fatalf("Unable to create TestStream: %s", err)
	}
	defer CleanupTestStreams([]*TestStream{ts})

	file, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("Failed to create temporary config file: %s", err)
	}
	configPaths := []string{file.Name()}

	// Pretend it's an old Logstash; if NewInvocation() is called
	// for 5.0 or newer it'll try to copy configuration files so
	// we'd have to generate such files too.
	v, err := semver.NewVersion("2.4.0")
	if err != nil {
		t.Fatalf("Unable to parse version number: %s", err)
	}

	inv, err := NewInvocation(os.Args[0], []string{}, v, configPaths...)
	if err != nil {
		t.Fatalf("Unable to create Invocation: %s", err)
	}
	defer inv.Release()
	p, err := NewParallelProcess(inv, []*TestStream{ts}, []string{})
	if err != nil {
		t.Fatalf("Unable to create ParallelProcess: %s", err)
	}
	defer p.Release()

	p.child.Env = append(os.Environ(), "TEST_MAIN=logstash-mock", "TEST_SOCKET="+ts.senderPath)

	if err = p.Start(); err != nil {
		t.Fatalf("Unable to start ParallelProcess: %s", err)
	}

	_, err = ts.Write([]byte(testLine))
	if err != nil {
		t.Fatalf("Unable to write to TestStream: %s", err)
	}
	if err = ts.Close(); err != nil {
		t.Fatalf("Unable to close TestStream: %s", err)
	}

	result, err := p.WaitAndRead()
	if err != nil {
		t.Fatalf("Error while waiting for ParallelProcess to finish: %s", err)
	}
	if result.Output != testLine {
		t.Errorf("Unexpected return from ParallelProcess, expected: %s, got: %s", testLine, result.Output)
	}
}

func TestGetSocketInOutPlugins(t *testing.T) {
	// Create a single temporary file that all test cases can use.
	receiver, err := newDeletedTempFile("", "")
	if err != nil {
		t.Fatalf("Unable to create temporary file: %s", err)
	}
	defer receiver.Close()

	cases := []struct {
		streams         []*TestStream
		expectedInputs  []string
		expectedOutputs []string
		err             error
	}{
		// Single TestStream struct.
		{
			[]*TestStream{
				{
					senderPath: "/tmp/foo",
					inputCodec: "any_codec",
					fields:     FieldSet{},
					receiver:   receiver,
				},
			},
			[]string{
				"unix { mode => \"client\" path => \"/tmp/foo\" codec => any_codec " +
					"add_field => { \"[@metadata][__lfv_testcase]\" => \"0\" } }",
			},
			[]string{
				fmt.Sprintf("if [@metadata][__lfv_testcase] == \"0\" { file { path => %q codec => \"json_lines\" } }", receiver.Name()),
			},
			nil,
		},
		// Multiple TestStream structs.
		{
			[]*TestStream{
				{
					senderPath: "/tmp/foo",
					inputCodec: "any_codec",
					fields:     FieldSet{},
					receiver:   receiver,
				},
				{
					senderPath: "/tmp/bar",
					inputCodec: "other_codec",
					fields:     FieldSet{},
					receiver:   receiver,
				},
			},
			[]string{
				"unix { mode => \"client\" path => \"/tmp/foo\" codec => any_codec " +
					"add_field => { \"[@metadata][__lfv_testcase]\" => \"0\" } }",
				"unix { mode => \"client\" path => \"/tmp/bar\" codec => other_codec " +
					"add_field => { \"[@metadata][__lfv_testcase]\" => \"1\" } }",
			},
			[]string{
				fmt.Sprintf("if [@metadata][__lfv_testcase] == \"0\" { file { path => %q codec => \"json_lines\" } }", receiver.Name()),
				fmt.Sprintf("if [@metadata][__lfv_testcase] == \"1\" { file { path => %q codec => \"json_lines\" } }", receiver.Name()),
			},
			nil,
		},
		// Single TestStream struct with additional fields set.
		{
			[]*TestStream{
				{
					senderPath: "/tmp/foo",
					inputCodec: "any_codec",
					fields: FieldSet{
						"@metadata": map[string]interface{}{
							"foo": "bar",
						},
					},
					receiver: receiver,
				},
			},
			[]string{
				"unix { mode => \"client\" path => \"/tmp/foo\" codec => any_codec " +
					"add_field => { \"[@metadata][__lfv_testcase]\" => \"0\" \"[@metadata][foo]\" => \"bar\" } }",
			},
			[]string{
				fmt.Sprintf("if [@metadata][__lfv_testcase] == \"0\" { file { path => %q codec => \"json_lines\" } }", receiver.Name()),
			},
			nil,
		},
		// Single TestStream struct with a non-map @metadata
		// field should result in an error.
		{
			[]*TestStream{
				{
					senderPath: "/tmp/foo",
					inputCodec: "any_codec",
					fields: FieldSet{
						"@metadata": "foo",
					},
					receiver: receiver,
				},
			},
			nil,
			nil,
			errors.New("the supplied contents of the @metadata field must be a hash (found string instead)"),
		},
	}
	for i, c := range cases {
		inputs, outputs, err := getSocketInOutPlugins(c.streams)

		if err == nil && c.err != nil {
			t.Errorf("Test %d: Expected failure, got success.", i)
		} else if err != nil && c.err == nil {
			t.Errorf("Test %d: Expected success, got this error instead: %#v", i, err)
		} else if err != nil && c.err != nil && err.Error() != c.err.Error() {
			t.Errorf("Test %d: Didn't get the expected error.\nExpected:\n%s\nGot:\n%s", i, c.err, err)
		} else {
			if !reflect.DeepEqual(c.expectedInputs, inputs) {
				t.Errorf("Test %d:\nExpected:\n%#v\nGot:\n%#v", i, c.expectedInputs, inputs)
			}
			if !reflect.DeepEqual(c.expectedOutputs, outputs) {
				t.Errorf("Test %d:\nExpected:\n%#v\nGot:\n%#v", i, c.expectedOutputs, outputs)
			}
		}
	}
}
