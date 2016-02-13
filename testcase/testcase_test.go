// Copyright (c) 2015-2016 Magnus Bäck <magnus@noun.se>

package testcase

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/magnusbaeck/logstash-filter-verifier/logstash"
)

func TestNew(t *testing.T) {
	cases := []struct {
		input    string
		expected TestCase
	}{
		// Happy flow relying on the default codec.
		{
			`{"fields": {"type": "mytype"}}`,
			TestCase{
				Codec: "plain",
				InputFields: logstash.FieldSet{
					"type": "mytype",
				},
				IgnoredFields: []string{"@version"},
			},
		},
		// Happy flow with a custom codec.
		{
			`{"fields": {"type": "mytype"}, "codec": "json"}`,
			TestCase{
				Codec: "json",
				InputFields: logstash.FieldSet{
					"type": "mytype",
				},
				IgnoredFields: []string{"@version"},
			},
		},
		// Additional fields to ignore are appended to the default.
		{
			`{"ignore": ["foo"]}`,
			TestCase{
				Codec:         "plain",
				IgnoredFields: []string{"@version", "foo"},
			},
		},
	}
	for i, c := range cases {
		tc, err := New(bytes.NewReader([]byte(c.input)))
		if err != nil {
			t.Errorf("Test %d: %q input: %s", i, c.input, err)
			break
		}
		resultJSON := marshalTestCase(t, tc)
		expectedJSON := marshalTestCase(t, &c.expected)
		if expectedJSON != resultJSON {
			t.Errorf("Test %d:\nExpected:\n%s\nGot:\n%s", i, expectedJSON, resultJSON)
		}
	}
}

// TestNewFromFile smoketests NewFromFile and makes sure it returns
// an absolute path even if a relative path was given as input.
func TestNewFromFile(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf(err.Error())
	}
	olddir, err := os.Getwd()
	if err != nil {
		t.Fatalf(err.Error())
	}
	defer os.RemoveAll(tempdir)
	defer os.Chdir(olddir)
	if err = os.Chdir(tempdir); err != nil {
		t.Fatalf(err.Error())
	}

	fullTestCasePath := filepath.Join(tempdir, "test.json")
	if err = ioutil.WriteFile(fullTestCasePath, []byte(`{"type": "test"}`), 0644); err != nil {
		t.Fatalf(err.Error())
	}

	tc, err := NewFromFile("test.json")
	if err != nil {
		t.Fatalf("NewFromFile() unexpectedly returned an error: %s", err)
	}

	if tc.File != fullTestCasePath {
		t.Fatalf("Expected test case path to be %q, got %q instead.", fullTestCasePath, tc.File)
	}
}

func TestCompare(t *testing.T) {
	// Create an empty tempdir so that we can construct a path to
	// a diff binary that's guaranteed to not exist.
	tempdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf(err.Error())
	}
	defer os.RemoveAll(tempdir)

	cases := []struct {
		testcase     *TestCase
		actualEvents []logstash.Event
		diffCommand  []string
		result       error
	}{
		// Empty test case with no messages is okay.
		{
			&TestCase{
				File: "/path/to/filename.json",
				InputFields: logstash.FieldSet{
					"type": "test",
				},
				Codec:          "plain",
				InputLines:     []string{},
				ExpectedEvents: []logstash.Event{},
			},
			[]logstash.Event{},
			[]string{"diff"},
			nil,
		},
		// Too few messages received.
		{
			&TestCase{
				File: "/path/to/filename.json",
				InputFields: logstash.FieldSet{
					"type": "test",
				},
				Codec:      "plain",
				InputLines: []string{},
				ExpectedEvents: []logstash.Event{
					{
						"a": "b",
					},
					{
						"c": "d",
					},
				},
			},
			[]logstash.Event{
				{
					"a": "b",
				},
			},
			[]string{"diff"},
			ComparisonError{
				ActualCount:   1,
				ExpectedCount: 2,
				Mismatches:    []MismatchedEvent{},
			},
		},
		// Too many messages received.
		{
			&TestCase{
				File: "/path/to/filename.json",
				InputFields: logstash.FieldSet{
					"type": "test",
				},
				Codec:      "plain",
				InputLines: []string{},
				ExpectedEvents: []logstash.Event{
					{
						"a": "b",
					},
				},
			},
			[]logstash.Event{
				{
					"a": "b",
				},
				{
					"c": "d",
				},
			},
			[]string{"diff"},
			ComparisonError{
				ActualCount:   2,
				ExpectedCount: 1,
				Mismatches:    []MismatchedEvent{},
			},
		},
		// Different fields.
		{
			&TestCase{
				File: "/path/to/filename.json",
				InputFields: logstash.FieldSet{
					"type": "test",
				},
				Codec:      "plain",
				InputLines: []string{},
				ExpectedEvents: []logstash.Event{
					{
						"a": "b",
					},
				},
			},
			[]logstash.Event{
				{
					"c": "d",
				},
			},
			[]string{"diff"},
			ComparisonError{
				ActualCount:   1,
				ExpectedCount: 1,
				Mismatches: []MismatchedEvent{
					{
						Actual: logstash.Event{
							"c": "d",
						},
						Expected: logstash.Event{
							"a": "b",
						},
						Index: 0,
					},
				},
			},
		},
		// Same field with different values.
		{
			&TestCase{
				File: "/path/to/filename.json",
				InputFields: logstash.FieldSet{
					"type": "test",
				},
				Codec:      "plain",
				InputLines: []string{},
				ExpectedEvents: []logstash.Event{
					{
						"a": "b",
					},
				},
			},
			[]logstash.Event{
				{
					"a": "B",
				},
			},
			[]string{"diff"},
			ComparisonError{
				ActualCount:   1,
				ExpectedCount: 1,
				Mismatches: []MismatchedEvent{
					{
						Actual: logstash.Event{
							"a": "B",
						},
						Expected: logstash.Event{
							"a": "b",
						},
						Index: 0,
					},
				},
			},
		},
		// Ignored fields are ignored.
		{
			&TestCase{
				File: "/path/to/filename.json",
				InputFields: logstash.FieldSet{
					"type": "test",
				},
				Codec:         "plain",
				IgnoredFields: []string{"ignored"},
				InputLines:    []string{},
				ExpectedEvents: []logstash.Event{
					{
						"not_ignored": "value",
					},
				},
			},
			[]logstash.Event{
				{
					"ignored":     "ignoreme",
					"not_ignored": "value",
				},
			},
			[]string{"diff"},
			nil,
		},
		// Diff command execution errors are propagated correctly.
		{
			&TestCase{
				File: "/path/to/filename.json",
				InputFields: logstash.FieldSet{
					"type": "test",
				},
				Codec:      "plain",
				InputLines: []string{},
				ExpectedEvents: []logstash.Event{
					{
						"a": "b",
					},
				},
			},
			[]logstash.Event{
				{
					"a": "b",
				},
			},
			[]string{filepath.Join(tempdir, "does-not-exist")},
			&os.PathError{},
		},
	}

	for i, c := range cases {
		actualResult := c.testcase.Compare(c.actualEvents, true, c.diffCommand)
		if actualResult == nil && c.result != nil {
			t.Errorf("Test %d: Expected failure, got success.", i)
		} else if actualResult != nil && c.result == nil {
			t.Errorf("Test %d: Expected success, got this error instead: %#v", i, actualResult)
		} else if actualResult != nil && c.result != nil {
			// Check if we get the right kind of error.
			actualType := reflect.TypeOf(actualResult)
			expectedType := reflect.TypeOf(c.result)
			if actualType == expectedType {
				switch e := actualResult.(type) {
				case ComparisonError:
					if !reflect.DeepEqual(c.result, e) {
						t.Errorf("Test %d:\nExpected:\n%#v\nGot:\n%#v", i, c.result, e)
					}
				default:
					// Except in the explicitly
					// handled cases above we just
					// care that the types match.
				}
			} else {
				t.Errorf("Test %d:\nExpected error:\n%s\nGot:\n%s (%s)", i, expectedType, actualType, actualResult)
			}
		}
	}
}

func TestMarshalToFile(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf(err.Error())
	}
	defer os.RemoveAll(tempdir)

	// Implicitly test that subdirectories are created as needed.
	fullpath := filepath.Join(tempdir, "a", "b", "c.json")

	if err = marshalToFile(logstash.Event{}, fullpath); err != nil {
		t.Fatalf(err.Error())
	}

	// We won't verify the actual contents that was marshalled,
	// we'll just check that it can be unmarshalled again and that
	// the file ends with a newline.
	buf, err := ioutil.ReadFile(fullpath)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if len(buf) > 0 && buf[len(buf)-1] != '\n' {
		t.Errorf("Expected non-empty file ending with a newline: %q", string(buf))
	}
	var event logstash.Event
	if err = json.Unmarshal(buf, &event); err != nil {
		t.Errorf("%s: %q", err.Error(), string(buf))
	}
}

func marshalTestCase(t *testing.T, tc *TestCase) string {
	resultBuf, err := json.MarshalIndent(tc, "", "  ")
	if err != nil {
		t.Errorf("Failed to marshal %+v as JSON: %s", tc, err)
		return ""
	}
	return string(resultBuf)
}
