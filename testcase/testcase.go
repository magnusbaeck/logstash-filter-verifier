// Copyright (c) 2015-2018 Magnus Bäck <magnus@noun.se>

package testcase

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	unjson "github.com/hashicorp/packer/common/json"
	"github.com/imkira/go-observer"
	"github.com/magnusbaeck/logstash-filter-verifier/logging"
	"github.com/magnusbaeck/logstash-filter-verifier/logstash"
	lfvobserver "github.com/magnusbaeck/logstash-filter-verifier/observer"
	"github.com/mikefarah/yaml/v2"
)

// TestCaseSet contains the configuration of a Logstash filter test case.
// Most of the fields are supplied by the user via a JSON file or YAML file.
type TestCaseSet struct {
	// File is the absolute path to the file from which this
	// test case was read.
	File string `json:"-" yaml:"-"`

	// Codec names the Logstash codec that should be used when
	// events are read. This is normally "line" or "json_lines".
	Codec string `json:"codec" yaml:"codec"`

	// IgnoredFields contains a list of fields that will be
	// deleted from the events that Logstash returns before
	// they're compared to the events in ExpectedEevents.
	//
	// This can be used for skipping fields that Logstash
	// populates with unpredictable contents (hostnames or
	// timestamps) that can't be hard-wired into the test case
	// file.
	//
	// It's also useful for the @version field that Logstash
	// always adds with a constant value so that one doesn't have
	// to include that field in every event in ExpectedEvents.
	IgnoredFields []string `json:"ignore" yaml:"ignore"`

	// InputFields contains a mapping of fields that should be
	// added to input events, like "type" or "tags". The map
	// values may be scalar values or arrays of scalar
	// values. This is often important since filters typically are
	// configured based on the event's type or its tags.
	InputFields logstash.FieldSet `json:"fields" yaml:"fields"`

	// InputLines contains the lines of input that should be fed
	// to the Logstash process.
	InputLines []string `json:"input" yaml:"input"`

	// ExpectedEvents contains a slice of expected events to be
	// compared to the actual events produced by the Logstash
	// process.
	ExpectedEvents []logstash.Event `json:"expected" yaml:"expected"`

	// TestCases is a slice of test cases, which include at minimum
	// a pair of an input and an expected event
	// Optionally other information regarding the test case
	// may be supplied.
	TestCases []TestCase `json:"testcases" yaml:"testcases"`

	descriptions []string `json:"descriptions" yaml:"descriptions"`
}

// TestCase is a pair of an input line that should be fed
// into the Logstash process and an expected event which is compared
// to the actual event produced by the Logstash process.
type TestCase struct {
	// InputLines contains the lines of input that should be fed
	// to the Logstash process.
	InputLines []string `json:"input" yaml:"input"`

	// ExpectedEvents contains a slice of expected events to be
	// compared to the actual events produced by the Logstash
	// process.
	ExpectedEvents []logstash.Event `json:"expected" yaml:"expected"`

	// Description contains an optional description of the test case
	// which will be printed while the tests are executed.
	Description string `json:"description" yaml:"description"`
}

var (
	log = logging.MustGetLogger()

	defaultIgnoredFields = []string{"@version"}
)

// convertBracketFields permit to replace keys that contains bracket with sub structure.
// For example, the key `[log][file][path]` will be convert by `"log": {"file": {"path": "VALUE"}}`
func (tcs *TestCaseSet) convertBracketFields() error {
	// Convert fields in input fields
	tcs.InputFields = parseAllBracketProperties(tcs.InputFields)

	// Convert fields in expected events
	for i, expected := range tcs.ExpectedEvents {
		tcs.ExpectedEvents[i] = parseAllBracketProperties(expected)
	}

	// Convert fields in input json string
	if tcs.Codec == "json_lines" {
		for i, line := range tcs.InputLines {
			var jsonObj map[string]interface{}
			if err := json.Unmarshal([]byte(line), &jsonObj); err != nil {
				return err
			}
			jsonObj = parseAllBracketProperties(jsonObj)
			data, err := json.Marshal(jsonObj)
			if err != nil {
				return err
			}
			tcs.InputLines[i] = string(data)
		}
	}

	return nil
}

// New reads a test case configuration from a reader and returns a
// TestCase. Defaults to a "line" codec and ignoring the @version
// field. If the configuration being read lists additional fields to
// ignore those will be ignored in addition to @version.
// configType must be json or yaml or yml.
func New(reader io.Reader, configType string) (*TestCaseSet, error) {
	if configType != "json" && configType != "yaml" && configType != "yml" {
		return nil, errors.New("Config type must be json or yaml or yml")
	}

	tcs := TestCaseSet{
		Codec:       "line",
		InputFields: logstash.FieldSet{},
	}
	buf, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	if configType == "json" {
		if err = unjson.Unmarshal(buf, &tcs); err != nil {
			return nil, err
		}
	} else {
		// Fix issue https://github.com/go-yaml/yaml/issues/139
		yaml.DefaultMapType = reflect.TypeOf(map[string]interface{}{})
		if err = yaml.Unmarshal(buf, &tcs); err != nil {
			return nil, err
		}
	}

	if err = tcs.InputFields.IsValid(); err != nil {
		return nil, err
	}
	tcs.IgnoredFields = append(tcs.IgnoredFields, defaultIgnoredFields...)
	sort.Strings(tcs.IgnoredFields)
	tcs.descriptions = make([]string, len(tcs.ExpectedEvents))
	for _, tc := range tcs.TestCases {
		tcs.InputLines = append(tcs.InputLines, tc.InputLines...)
		tcs.ExpectedEvents = append(tcs.ExpectedEvents, tc.ExpectedEvents...)
		for range tc.ExpectedEvents {
			tcs.descriptions = append(tcs.descriptions, tc.Description)
		}
	}

	// Convert bracket fields
	if err := tcs.convertBracketFields(); err != nil {
		return nil, err
	}

	log.Debugf("Current TestCaseSet after converting fields: %+v", tcs)
	return &tcs, nil
}

// NewFromFile reads a test case configuration from an on-disk file.
func NewFromFile(path string) (*TestCaseSet, error) {
	abspath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	ext := strings.TrimPrefix(filepath.Ext(abspath), ".")

	log.Debugf("Reading test case file: %s (%s)", path, abspath)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()

	tcs, err := New(f, ext)
	if err != nil {
		return nil, fmt.Errorf("Error reading/unmarshalling %s: %s", path, err)
	}
	tcs.File = abspath
	return tcs, nil
}

// Compare compares a slice of events against the expected events of
// this test case. Each event is written pretty-printed to a temporary
// file and the two files are passed to "diff -u". The resulting of diff command
// is sended to observer throughout lfvobserver.ComparisonResult struct.
// It return true if the current test case pass, else it return false.
func (tcs *TestCaseSet) Compare(events []logstash.Event, diffCommand []string, liveProducer observer.Property) (bool, error) {
	status := true

	// Don't even attempt to do a deep comparison of the event
	// lists unless their lengths are equal.
	if len(tcs.ExpectedEvents) != len(events) {
		comparisonResult := lfvobserver.ComparisonResult{
			Status:     false,
			Name:       "Compare actual event with expected event",
			Explain:    fmt.Sprintf("Expected %d event(s), got %d instead.", len(tcs.ExpectedEvents), len(events)),
			Path:       filepath.Base(tcs.File),
			EventIndex: 0,
		}
		liveProducer.Update(comparisonResult)
		return false, nil
	}

	// Check if test consit to validate that all event are dropped and so not failed before
	if len(events) == 0 {
		comparisonResult := lfvobserver.ComparisonResult{
			Status:     true,
			Name:       "Compare actual event with expected event",
			Explain:    "Drop all events",
			Path:       filepath.Base(tcs.File),
			EventIndex: 0,
		}
		liveProducer.Update(comparisonResult)
		return true, nil
	}

	tempdir, err := ioutil.TempDir("", "")
	if err != nil {
		return false, err
	}
	defer func() {
		if err := os.RemoveAll(tempdir); err != nil {
			log.Errorf("Problem deleting temporary directory: %s", err)
		}
	}()

	for i, actualEvent := range events {
		comparisonResult := lfvobserver.ComparisonResult{
			Path:       filepath.Base(tcs.File),
			EventIndex: i,
			Status:     true,
		}
		if (len(tcs.descriptions) > i) && (len(tcs.descriptions[i]) > 0) {
			comparisonResult.Name = fmt.Sprintf("Comparing message %d of %d (%s)", i+1, len(events), tcs.descriptions[i])
		} else {
			comparisonResult.Name = fmt.Sprintf("Comparing message %d of %d", i+1, len(events))
		}

		// Ignored fields can be in a sub object
		for _, ignored := range tcs.IgnoredFields {
			removeFields(ignored, actualEvent)
		}

		// Create a directory structure for the JSON file being
		// compared that makes it easy for the user to identify
		// the failing test case in the diff output:
		// $TMP/<random>/<test case file>/<event #>/<actual|expected>
		resultDir := filepath.Join(tempdir, filepath.Base(tcs.File), strconv.Itoa(i+1))
		actualFilePath := filepath.Join(resultDir, "actual")
		if err = marshalToFile(actualEvent, actualFilePath); err != nil {
			return false, err
		}
		expectedFilePath := filepath.Join(resultDir, "expected")
		if err = marshalToFile(tcs.ExpectedEvents[i], expectedFilePath); err != nil {
			return false, err
		}

		comparisonResult.Status, comparisonResult.Explain, err = runDiffCommand(diffCommand, expectedFilePath, actualFilePath)
		if err != nil {
			return false, err
		}
		if !comparisonResult.Status {
			status = false
		}

		liveProducer.Update(comparisonResult)
	}

	return status, nil
}

// marshalToFile pretty-prints a logstash.Event and writes it to a
// file, creating the file's parent directories as necessary.
func marshalToFile(event logstash.Event, filename string) error {
	buf, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return fmt.Errorf("Failed to marshal %+v as JSON: %s", event, err)
	}
	if err = os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		return err
	}
	return ioutil.WriteFile(filename, []byte(string(buf)+"\n"), 0600)
}

// runDiffCommand passes two files to the supplied command (executable
// path and optional arguments) and returns whether the files were
// equal, i.e. whether the diff command returned a zero exit
// status. The returned error value will be set if there was a problem
// running the command. The output of the diff command will is returned
// as string.
func runDiffCommand(command []string, file1, file2 string) (bool, string, error) {
	fullCommand := append(command, file1)
	fullCommand = append(fullCommand, file2)
	/* #nosec */
	c := exec.Command(fullCommand[0], fullCommand[1:]...)
	stdoutStderr, err := c.CombinedOutput()

	// Doesn't return error if diff command return 1
	// It's normal behavior
	if exitError, ok := err.(*exec.ExitError); ok {
		if exitError.ExitCode() == 1 {
			return false, "", nil
		}
	}
	if err != nil {
		return false, "", err
	}
	return true, string(stdoutStderr), nil
}
