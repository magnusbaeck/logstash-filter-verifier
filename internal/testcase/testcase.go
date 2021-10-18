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
	"github.com/mikefarah/yaml/v2"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logstash"
	lfvobserver "github.com/magnusbaeck/logstash-filter-verifier/v2/internal/observer"
)

const DummyEventInputIndicator = "__lfv_dummy_event"

// TestCaseSet contains the configuration of a Logstash filter test case.
// Most of the fields are supplied by the user via a JSON file or YAML file.
type TestCaseSet struct {
	// File is the absolute path to the file from which this
	// test case was read.
	File string `json:"-" yaml:"-"`

	// The unique ID of the input plugin in the tested configuration, where the
	// test input is coming from. This is necessary, if a setup with multiple
	// inputs is tested, which either have different codecs or are part of
	// different pipelines.
	// https://www.elastic.co/guide/en/logstash/7.10/plugins-inputs-file.html#plugins-inputs-file-id
	InputPlugin string `json:"input_plugin" yaml:"input_plugin"`

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

	// ExportMetadata controls if the metadata of the event processed
	// by Logstash is returned. The metadata is contained in the field
	// `[@metadata]` in the Logstash event.
	// If the metadata is exported, the respective fields are compared
	// with the expected result of the testcase as well. (default: false)
	ExportMetadata bool `json:"export_metadata" yaml:"export_metadata"`

	// ExportOutputs controls if the ID of the output, a particular event has
	// emitted by, is kept in the event or not.
	// If this is enabled, the expected event needs to contain a field named
	// __lfv_out_passed which contains the ID of the Logstash output.
	ExportOutputs bool `json:"export_outputs" yaml:"export_outputs"`

	// TestCases is a slice of test cases, which include at minimum
	// a pair of an input and an expected event.
	// Optionally other information regarding the test case may be supplied.
	TestCases []TestCase `json:"testcases" yaml:"testcases"`

	// Events contains the fields for each event. This fields is filled
	// in the New function. The sources are: InputFields, TestCase.Event and
	// TestCase.InputLines
	Events []logstash.FieldSet `json:"-" yaml:"-"`

	descriptions []string
}

// TestCase is a pair of an input line that should be fed
// into the Logstash process and an expected event which is compared
// to the actual event produced by the Logstash process.
type TestCase struct {
	// InputLines contains the lines of input that should be fed
	// to the Logstash process.
	InputLines []string `json:"input" yaml:"input"`

	// Local fields, only added to the events of this test case.
	// These fields overwrite global fields.
	InputFields logstash.FieldSet `json:"fields" yaml:"fields"`

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
// For example, the key `[log][file][path]` will be convert by `"log": {"file": {"path": "VALUE"}}`.
func (tcs *TestCaseSet) convertBracketFields() error {
	// Convert fields in input fields
	tcs.InputFields = parseAllBracketProperties(tcs.InputFields)
	for i := range tcs.TestCases {
		tcs.TestCases[i].InputFields = parseAllBracketProperties(tcs.TestCases[i].InputFields)
	}

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

	// Convert bracket fields
	if err := tcs.convertBracketFields(); err != nil {
		return nil, err
	}

	tcs.IgnoredFields = append(tcs.IgnoredFields, defaultIgnoredFields...)
	sort.Strings(tcs.IgnoredFields)

	tcs.descriptions = make([]string, len(tcs.ExpectedEvents))

	for range tcs.InputLines {
		tcs.Events = append(tcs.Events, tcs.InputFields)
	}

	for _, tc := range tcs.TestCases {
		// Add event, if there are no input lines.
		if len(tc.InputLines) == 0 {
			tc.InputLines = []string{DummyEventInputIndicator}
		}
		tcs.InputLines = append(tcs.InputLines, tc.InputLines...)
		tcs.ExpectedEvents = append(tcs.ExpectedEvents, tc.ExpectedEvents...)
		// Process each input line
		for range tc.InputLines {
			// Global Fields first.
			tcs.Events = append(tcs.Events, tcs.InputFields)
			// Merge with test case fields, eventually overwriting global fields.
			for k, v := range tc.InputFields {
				tcs.Events[len(tcs.Events)-1][k] = v
			}
		}
		for range tc.ExpectedEvents {
			tcs.descriptions = append(tcs.descriptions, tc.Description)
		}
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
// file and the two files are passed to the diff command. Its output is
// is sent to the observer via an lfvobserver.ComparisonResult struct.
// Returns true if the current test case passes, otherwise false. A non-nil
// error value indicates a problem executing the test.
func (tcs *TestCaseSet) Compare(events []logstash.Event, diffCommand []string, liveProducer observer.Property) (bool, error) {
	status := true

	// Don't even attempt to do a deep comparison of the event
	// lists unless their lengths are equal.
	if len(tcs.ExpectedEvents) != len(events) {
		eventsJSON, err := json.MarshalIndent(events, "", "  ")
		if err != nil {
			return false, err
		}
		comparisonResult := lfvobserver.ComparisonResult{
			Status:     false,
			Name:       "Compare actual event with expected event",
			Explain:    fmt.Sprintf("Expected %d event(s), got %d instead.\nReceived events: %s", len(tcs.ExpectedEvents), len(events), string(eventsJSON)),
			Path:       filepath.Base(tcs.File),
			EventIndex: 0,
		}
		liveProducer.Update(comparisonResult)
		return false, nil
	}

	// Make sure we produce a result even if there are zero events (i.e. we
	// won't enter the for loop below).
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
// equal. The returned error value will be set if there was a problem
// running the command or if it returned an exit status other than zero
// or one. The latter is interpreted as "comparison performed successfully
// but the files were different". The output of the diff command is
// returned as a string.
func runDiffCommand(command []string, file1, file2 string) (bool, string, error) {
	fullCommand := append(command, file1)
	fullCommand = append(fullCommand, file2)
	/* #nosec */
	c := exec.Command(fullCommand[0], fullCommand[1:]...)
	stdoutStderr, err := c.CombinedOutput()

	success := err == nil
	if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
		// Exit code 1 is expected when the files differ; just ignore it.
		err = nil
	}
	return success, string(stdoutStderr), err
}
