// Copyright (c) 2015 Magnus Bäck <magnus@noun.se>

package testcase

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/magnusbaeck/logstash-filter-verifier/logging"
	"github.com/magnusbaeck/logstash-filter-verifier/logstash"
)

// TestCase contains the configuration of a Logstash filter test case.
// Most of the fields are supplied by the user via a JSON file.
type TestCase struct {
	// File is the absolute path to the file from which this
	// test case was read.
	File string `json:-`

	// Type contains the contents of the "type" field of the
	// events that are to be read. This is often important since
	// filters typically are configured based on the event's type.
	Type string `json:"type"`

	// Codec names the Logstash codec that should be used when
	// events are read. This is normally "plain" or "json".
	Codec string `json:"codec"`

	// InputLines contains the lines of input that should be fed
	// to the Logstash process.
	InputLines []string `json:"input"`

	// ExpectedEvents contains a slice of expected events to be
	// compared to the actual events produced by the Logstash
	// process.
	ExpectedEvents []logstash.Event `json:"expected"`
}

// ComparisonError indicates that there was a mismatch when the
// results of a test case was compared against the test case
// definition.
type ComparisonError struct {
	ActualCount   int
	ExpectedCount int
	Mismatches    []MismatchedEvent
}

// MismatchedEvent holds a single tuple of actual and expected events
// for a particular index in the list of events for a test case.
type MismatchedEvent struct {
	Actual   logstash.Event
	Expected logstash.Event
	Index    int
}

var (
	log = logging.MustGetLogger()
)

// New reads a test case configuration from a reader and returns a
// TestCase. Defaults to a "plain" codec.
func New(reader io.Reader) (*TestCase, error) {
	tc := TestCase{
		Codec: "plain",
	}
	buf, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(buf, &tc); err != nil {
		return nil, err
	}
	return &tc, nil
}

// NewFromFile reads a test case configuration from an on-disk file.
func NewFromFile(path string) (*TestCase, error) {
	abspath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	log.Debug("Reading test case file: %s (%s)", path, abspath)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tc, err := New(f)
	if err != nil {
		return nil, fmt.Errorf("Error reading/unmarshalling %s: %s", path, err)
	}
	tc.File = abspath
	return tc, nil
}

// Compare compares a slice of events against the expected events of
// this test case. Each event is written pretty-printed to a temporary
// file and the two files are passed to "diff -u". If quiet is true,
// the progress messages normally written to stderr will be emitted
// and the output of the diff program will be discarded.
func (tc *TestCase) Compare(events []logstash.Event, quiet bool) error {
	result := ComparisonError{
		ActualCount:   len(events),
		ExpectedCount: len(tc.ExpectedEvents),
		Mismatches:    []MismatchedEvent{},
	}

	// Don't even attempt to do a deep comparison of the event
	// lists unless their lengths are equal.
	if result.ActualCount != result.ExpectedCount {
		return result
	}

	// Create a directory structure for the JSON file being
	// compared that makes it easy for the user to identify
	// the failing test case in the diff output:
	// $TMP/<random>/<test case file>/<event number>/<actual|expected>
	tempdir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil
	}
	defer func() {
		if err := os.RemoveAll(tempdir); err != nil {
			log.Error("Problem deleting temporary directory: %s", err.Error())
		}
	}()

	for i, actualEvent := range events {
		if !quiet {
			fmt.Fprintf(os.Stderr, "Comparing message %d of %s...\n", i+1, filepath.Base(tc.File))
		}

		resultDir := filepath.Join(tempdir, filepath.Base(tc.File), strconv.Itoa(i))
		actualFilePath := filepath.Join(resultDir, "actual")
		if err = marshalToFile(actualEvent, actualFilePath); err != nil {
			return err
		}
		expectedFilePath := filepath.Join(resultDir, "expected")
		if err = marshalToFile(tc.ExpectedEvents[i], expectedFilePath); err != nil {
			return err
		}

		equal, err := runDiffCommand(expectedFilePath, actualFilePath, quiet)
		if err != nil {
			return err
		}
		if !equal {
			result.Mismatches = append(result.Mismatches, MismatchedEvent{actualEvent, tc.ExpectedEvents[i], i})
		}
	}
	if len(result.Mismatches) == 0 {
		return nil
	} else {
		return result
	}
}

// marshalToFile pretty-prints a logstash.Event and writes it to a
// file, creating the file's parent directories as necessary.
func marshalToFile(event logstash.Event, filename string) error {
	buf, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return fmt.Errorf("Failed to marshal %+v as JSON: %s", event, err.Error())
	}
	if err = os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		return err
	}
	if err = ioutil.WriteFile(filename, []byte(string(buf)+"\n"), 0600); err != nil {
		return err
	}
	return nil
}

// runDiffCommand passes two files to "diff -u" and returns whether
// the files were equal, i.e. whether the diff command returned a zero
// exit status. The returned error value will be set if there was a
// problem running the command. If quiet is true, the output of the
// diff command will be discarded. Otherwise the child process will
// inherit stdout and stderr from the parent.
func runDiffCommand(file1, file2 string, quiet bool) (bool, error) {
	c := exec.Command("diff", "-u", file1, file2)
	if !quiet {
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
	}
	log.Info("Starting %q with args %q.", c.Path, c.Args[1:])
	if err := c.Start(); err != nil {
		return false, err
	}
	if err := c.Wait(); err != nil {
		log.Info("Child with pid %d failed: %s", c.Process.Pid, err.Error())
		return false, nil
	}
	return true, nil
}

func (e ComparisonError) Error() string {
	if e.ActualCount != e.ExpectedCount {
		return fmt.Sprintf("Expected %d event(s), got %d instead.", e.ExpectedCount, e.ActualCount)
	}
	if len(e.Mismatches) > 0 {
		return fmt.Sprintf("%d message(s) did not match the expectations.", len(e.Mismatches))
	}
	return "No error"

}
