// Copyright (c) 2015-2018 Magnus Bäck <magnus@noun.se>

package testcase

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/imkira/go-observer"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/logstash"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	cases := []struct {
		input    string
		expected TestCaseSet
	}{
		// Happy flow relying on the default codec.
		{
			input: `{"fields": {"type": "mytype"}}`,
			expected: TestCaseSet{
				Codec: "line",
				InputFields: logstash.FieldSet{
					"type": "mytype",
				},
				IgnoredFields: []string{"@version"},
			},
		},
		// Happy flow with a custom codec.
		{
			input: `{"fields": {"type": "mytype"}, "codec": "json_lines"}`,
			expected: TestCaseSet{
				Codec: "json_lines",
				InputFields: logstash.FieldSet{
					"type": "mytype",
				},
				IgnoredFields: []string{"@version"},
			},
		},
		// Additional fields to ignore are appended to the default.
		{
			input: `{"ignore": ["foo"]}`,
			expected: TestCaseSet{
				Codec:         "line",
				InputFields:   logstash.FieldSet{},
				IgnoredFields: []string{"@version", "foo"},
			},
		},
		// Fields with bracket notation
		{
			input: `{"fields": {"type": "mytype", "[log][file][path]": "/tmp/file.log"}}`,
			expected: TestCaseSet{
				Codec: "line",
				InputFields: logstash.FieldSet{
					"type": "mytype",
					"log": map[string]interface{}{
						"file": map[string]interface{}{
							"path": "/tmp/file.log",
						},
					},
				},
				IgnoredFields: []string{"@version"},
			},
		},
		// No handle input with bracket notation when codec is line
		{
			input: `{"input": ["{\"[test][path]\": \"test\"}"]}`,
			expected: TestCaseSet{
				Codec:         "line",
				InputLines:    []string{"{\"[test][path]\": \"test\"}"},
				IgnoredFields: []string{"@version"},
				InputFields:   logstash.FieldSet{},
			},
		},
		// handle input with bracket notation when codec is json_lines
		{
			input: `{"input": ["{\"[test][path]\": \"test\"}"], "codec": "json_lines"}`,
			expected: TestCaseSet{
				Codec:         "json_lines",
				InputLines:    []string{"{\"test\":{\"path\":\"test\"}}"},
				IgnoredFields: []string{"@version"},
				InputFields:   logstash.FieldSet{},
			},
		},
	}
	for i, c := range cases {
		tcs, err := New(bytes.NewReader([]byte(c.input)), "json")
		if err != nil {
			t.Errorf("Test %d: %q input: %s", i, c.input, err)
			break
		}
		resultJSON := marshalTestCaseSet(t, tcs)
		expectedJSON := marshalTestCaseSet(t, &c.expected)
		if expectedJSON != resultJSON {
			t.Errorf("Test %d:\nExpected:\n%s\nGot:\n%s", i, expectedJSON, resultJSON)
		}
	}
}

// TestNewFromFile smoketests NewFromFile and makes sure it returns
// an absolute path even if a relative path was given as input.
func TestNewFromFile(t *testing.T) {
	filenames := []string{
		"filename.json",
		"filename.yml",
		"filename.yaml",
	}
	for _, filename := range filenames {
		tempdir, err := ioutil.TempDir("", "")
		if err != nil {
			t.Fatalf(err.Error())
		}
		defer os.RemoveAll(tempdir)
		olddir, err := os.Getwd()
		if err != nil {
			t.Fatalf(err.Error())
		}
		defer os.Chdir(olddir)
		if err = os.Chdir(tempdir); err != nil {
			t.Fatalf(err.Error())
		}

		fullTestCasePath := filepath.Join(tempdir, filename)

		// As it happens a valid JSON file is also a valid YAML file so
		// the file we create can have the same contents regardless of
		// the file format.
		if err = ioutil.WriteFile(fullTestCasePath, []byte(`{"type": "test"}`), 0600); err != nil {
			t.Fatal(err.Error())
		}

		tcs, err := NewFromFile(filename)
		if err != nil {
			t.Fatalf("NewFromFile() unexpectedly returned an error: %s", err)
		}

		if tcs.File != fullTestCasePath {
			t.Fatalf("Expected test case path to be %q, got %q instead.", fullTestCasePath, tcs.File)
		}
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

	liveObserver := observer.NewProperty(nil)

	cases := []struct {
		testcase     *TestCaseSet
		actualEvents []logstash.Event
		diffCommand  []string
		result       bool
		err          error
	}{
		// Empty test case with no messages is okay.
		{
			&TestCaseSet{
				File: "/path/to/filename.json",
				InputFields: logstash.FieldSet{
					"type": "test",
				},
				Codec:          "line",
				InputLines:     []string{},
				ExpectedEvents: []logstash.Event{},
			},
			[]logstash.Event{},
			[]string{"diff"},
			true,
			nil,
		},
		// Too few messages received.
		{
			&TestCaseSet{
				File: "/path/to/filename.json",
				InputFields: logstash.FieldSet{
					"type": "test",
				},
				Codec:      "line",
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
			false,
			nil,
		},
		// Too many messages received.
		{
			&TestCaseSet{
				File: "/path/to/filename.json",
				InputFields: logstash.FieldSet{
					"type": "test",
				},
				Codec:      "line",
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
			false,
			nil,
		},
		// Different fields.
		{
			&TestCaseSet{
				File: "/path/to/filename.json",
				InputFields: logstash.FieldSet{
					"type": "test",
				},
				Codec:      "line",
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
			false,
			nil,
		},
		// Same field with different values.
		{
			&TestCaseSet{
				File: "/path/to/filename.json",
				InputFields: logstash.FieldSet{
					"type": "test",
				},
				Codec:      "line",
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
			false,
			nil,
		},
		// Ignored fields are ignored.
		{
			&TestCaseSet{
				File: "/path/to/filename.json",
				InputFields: logstash.FieldSet{
					"type": "test",
				},
				Codec:         "line",
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
			true,
			nil,
		},
		// Ignored fields with bracket notation are ignored
		{
			&TestCaseSet{
				File: "/path/to/filename.json",
				InputFields: logstash.FieldSet{
					"type": "test",
				},
				Codec:         "line",
				IgnoredFields: []string{"[file][log][path]"},
				InputLines:    []string{},
				ExpectedEvents: []logstash.Event{
					{
						"not_ignored": "value",
						"file": map[string]interface{}{
							"log": map[string]interface{}{
								"line": "value",
							},
						},
					},
				},
			},
			[]logstash.Event{
				{
					"file": map[string]interface{}{
						"log": map[string]interface{}{
							"line": "value",
							"path": "ignore_me",
						},
					},
					"not_ignored": "value",
				},
			},
			[]string{"diff"},
			true,
			nil,
		},
		// Ignored fields with bracket notation are ignored (when empty hash)
		{
			&TestCaseSet{
				File: "/path/to/filename.json",
				InputFields: logstash.FieldSet{
					"type": "test",
				},
				Codec:         "line",
				IgnoredFields: []string{"[file][log][path]"},
				InputLines:    []string{},
				ExpectedEvents: []logstash.Event{
					{
						"not_ignored": "value",
					},
				},
			},
			[]logstash.Event{
				{
					"file": map[string]interface{}{
						"log": map[string]interface{}{
							"path": "ignore_me",
						},
					},
					"not_ignored": "value",
				},
			},
			[]string{"diff"},
			true,
			nil,
		},
		// Diff command execution errors are propagated correctly.
		{
			&TestCaseSet{
				File: "/path/to/filename.json",
				InputFields: logstash.FieldSet{
					"type": "test",
				},
				Codec:      "line",
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
			false,
			&os.PathError{},
		},
	}

	for i, c := range cases {
		actualResult, err := c.testcase.Compare(c.actualEvents, c.diffCommand, liveObserver)
		if err != nil && c.err == nil {
			t.Errorf("Test %d: Expected no error, got error: %s", i, err)
		} else if c.err != nil && err == nil {
			t.Errorf("Test %d: Expected error, got no error.", i)
		} else if actualResult != c.result {
			t.Errorf("Test %d: Expected %t, got %t.", i, c.result, actualResult)
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

	// We won't verify the actual contents that was marshaled,
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
		t.Errorf("%s: %q", err, string(buf))
	}
}

func marshalTestCaseSet(t *testing.T, tcs *TestCaseSet) string {
	resultBuf, err := json.MarshalIndent(tcs, "", "  ")
	if err != nil {
		t.Errorf("Failed to marshal %+v as JSON: %s", tcs, err)
		return ""
	}
	return string(resultBuf)
}

// TestConvertBracketFields tests that fields contain on fields, exclude and input
// test cases are converted on sub structure if contain bracket notation.
func TestConvertBracketFields(t *testing.T) {
	testCase := &TestCaseSet{
		File: "/path/to/filename.json",
		InputFields: logstash.FieldSet{
			"type":                "test",
			"[log][file][path]":   "/tmp/file.log",
			"[log][origin][file]": "test.java",
		},
		Codec: "json_lines",
		InputLines: []string{
			`{"message": "test", "[agent][hostname]": "localhost", "[log][level]": "info"}`,
		},
		ExpectedEvents: []logstash.Event{
			{
				"type":                "test",
				"[log][file][path]":   "/tmp/file.log",
				"[log][origin][file]": "test.java",
			},
		},
	}

	expected := &TestCaseSet{
		File: "/path/to/filename.json",
		InputFields: logstash.FieldSet{
			"type": "test",
			"log": map[string]interface{}{
				"file": map[string]interface{}{
					"path": "/tmp/file.log",
				},
				"origin": map[string]interface{}{
					"file": "test.java",
				},
			},
		},
		Codec: "json_lines",
		InputLines: []string{
			`{"agent":{"hostname":"localhost"},"log":{"level":"info"},"message":"test"}`,
		},
		ExpectedEvents: []logstash.Event{
			{
				"type": "test",
				"log": map[string]interface{}{
					"file": map[string]interface{}{
						"path": "/tmp/file.log",
					},
					"origin": map[string]interface{}{
						"file": "test.java",
					},
				},
			},
		},
	}

	err := testCase.convertBracketFields()
	assert.NoError(t, err)
	assert.Equal(t, expected, testCase)
}
