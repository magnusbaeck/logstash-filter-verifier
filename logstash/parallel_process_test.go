// Copyright (c) 2016 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestParallelProcess(t *testing.T) {
	const testLine = "test line\n"

	fs := FieldSet{}
	ts, err := NewTestStream("Codec", fs, 5*time.Second)
	if err != nil {
		t.Fatalf("Unable to create TestStream: %s", err)
	}
	defer CleanupTestStreams([]*TestStream{ts})

	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("Failed to create temporary config file: %s", err)
	}
	configPaths := []string{file.Name()}

	p, err := NewParallelProcess(os.Args[0], []*TestStream{ts}, []string{}, configPaths...)
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
		t.Fatalf("Unable to wirte to TestStream: %s", err)
	}
	if err = ts.Close(); err != nil {
		t.Fatalf("Unable to close TestStream: %s", err)
	}

	result, err := p.Wait()
	if err != nil {
		t.Fatalf("Error while Wait for ParallelProcess to finish: %s", err)
	}
	if result.Output != testLine {
		t.Errorf("Unexpected return from ParallelProcess, expected: %s, got: %s", testLine, result.Output)
	}
}
