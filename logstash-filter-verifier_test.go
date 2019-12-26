// Copyright (c) 2017 Magnus Bäck <magnus@noun.se>

package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/magnusbaeck/logstash-filter-verifier/testhelpers"
)

func TestFindExecutable(t *testing.T) {
	cases := []struct {
		// Test setup
		files []testhelpers.FileWithMode

		// Input & expected output
		inputs      []string
		expected    string
		errorRegexp *regexp.Regexp
	}{
		// No input paths.
		{
			[]testhelpers.FileWithMode{},
			[]string{},
			"",
			regexp.MustCompile(`^no existing executable found among candidates: `),
		},
		// No matches.
		{
			[]testhelpers.FileWithMode{},
			[]string{
				"foo",
				"bar",
			},
			"",
			regexp.MustCompile(`^no existing executable found among candidates: `),
		},
		// Only matching file is a directory.
		{
			[]testhelpers.FileWithMode{
				{"foo", os.ModeDir | 0755, ""},
			},
			[]string{
				"foo",
				"bar",
			},
			"",
			regexp.MustCompile(`^no existing executable found among candidates: `),
		},
		// Only matching file is not executable.
		{
			[]testhelpers.FileWithMode{
				{"foo", 0644, ""},
			},
			[]string{
				"foo",
				"bar",
			},
			"",
			regexp.MustCompile(`^no existing executable found among candidates: `),
		},
		// Multiple matches, returning first one.
		{
			[]testhelpers.FileWithMode{
				{"foo", 0755, ""},
				{"bar", 0755, ""},
			},
			[]string{
				"foo",
				"bar",
			},
			"foo",
			nil,
		},
		// Multiple matches, skipping the matching directory.
		{
			[]testhelpers.FileWithMode{
				{"foo", os.ModeDir | 0755, ""},
				{"bar", 0755, ""},
			},
			[]string{
				"foo",
				"bar",
			},
			"bar",
			nil,
		},
		// Multiple matches, skipping the matching non-executable.
		{
			[]testhelpers.FileWithMode{
				{"foo", 0644, ""},
				{"bar", 0755, ""},
			},
			[]string{
				"foo",
				"bar",
			},
			"bar",
			nil,
		},
	}
	for i, c := range cases {
		tempdir, err := ioutil.TempDir("", "")
		if err != nil {
			t.Fatalf("Test %d: Unexpected error when creating temp dir: %s", i, err)
		}
		defer os.RemoveAll(tempdir)

		for _, fwp := range c.files {
			if err = fwp.Create(tempdir); err != nil {
				t.Fatalf("Test %d: Unexpected error when creating test file: %s", i, err)
			}
		}

		// All paths in the testcase struct are relative but
		// the inputs and output of findExecutable need to be
		// made absolute.
		var absExpected string
		if c.expected != "" {
			absExpected = filepath.Join(tempdir, c.expected)
		}
		absInputs := make([]string, len(c.inputs))
		for i, p := range c.inputs {
			absInputs[i] = filepath.Join(tempdir, p)
		}

		result, err := findExecutable(absInputs)
		if err == nil && c.errorRegexp != nil {
			t.Errorf("Test %d: Expected failure, got success.", i)
		} else if err != nil && c.errorRegexp == nil {
			t.Errorf("Test %d: Expected success, got this error instead: %#v", i, err)
		} else if err != nil && c.errorRegexp != nil && !c.errorRegexp.MatchString(err.Error()) {
			t.Errorf("Test %d: Expected error to match regexp:\n%s\nGot:\n%s", i, c.errorRegexp, err)
		} else if result != absExpected {
			t.Errorf("Test %d:\nExpected:\n%s\nGot:\n%s", i, absExpected, result)
		}
	}
}
