// Copyright (c) 2016-2018 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/breml/logstash-config/ast"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/testhelpers"
)

func createLogstashConfigWithString(s string) string {
	plugin := ast.NewPlugin("mutate", ast.NewStringAttribute("id", s, ast.DoubleQuoted))
	conf := ast.NewConfig(nil, []ast.PluginSection{ast.NewPluginSection(ast.Filter, plugin)}, nil)
	return conf.String()
}

func addPathPrefix(prefix string, strings []string) []string {
	result := make([]string, len(strings))
	for i, s := range strings {
		result[i] = filepath.Join(prefix, s)
	}
	return result
}

func TestFlattenFilenames(t *testing.T) {
	cases := []struct {
		existingFiles []testhelpers.FileWithMode
		inputFiles    []string
		expected      []string
	}{
		// Files only.
		{
			[]testhelpers.FileWithMode{
				{"a", 0600, ""},
				{"b", 0600, ""},
			},
			[]string{"a", "b"},
			[]string{"a", "b"},
		},
		// Files only, and only a subset of them.
		{
			[]testhelpers.FileWithMode{
				{"a", 0600, ""},
				{"b", 0600, ""},
				{"c", 0600, ""},
			},
			[]string{"a", "b"},
			[]string{"a", "b"},
		},
		// Files and an empty subdirectory.
		{
			[]testhelpers.FileWithMode{
				{"a", 0600, ""},
				{"b", 0600, ""},
				{"c", os.ModeDir | 0700, ""},
			},
			[]string{"a", "b", "c"},
			[]string{"a", "b"},
		},
		// Files and a file in a subdirectory.
		{
			[]testhelpers.FileWithMode{
				{"a", 0600, ""},
				{"b", 0600, ""},
				{"c", os.ModeDir | 0700, ""},
				{"c/d", 0600, ""},
			},
			[]string{"a", "b", "c"},
			[]string{"a", "b", "c/d"},
		},
		// Files and multiple levels of subdirectories.
		{
			[]testhelpers.FileWithMode{
				{"a", 0600, ""},
				{"b", 0600, ""},
				{"c", os.ModeDir | 0700, ""},
				{"c/d", os.ModeDir | 0700, ""},
				{"c/d/e", 0600, ""},
				{"c/f", 0600, ""},
			},
			[]string{"a", "b", "c"},
			[]string{"a", "b", "c/f"},
		},
		// Just as directory with files.
		{
			[]testhelpers.FileWithMode{
				{"a", os.ModeDir | 0700, ""},
				{"a/b", 0600, ""},
			},
			[]string{"a"},
			[]string{"a/b"},
		},
	}
	for i, c := range cases {
		tempdir, err := ioutil.TempDir("", "")
		if err != nil {
			t.Fatalf("Test %d: Unexpected error when creating temp dir: %s", i, err)
		}
		defer os.RemoveAll(tempdir)

		for _, fwm := range c.existingFiles {
			if err = fwm.Create(tempdir); err != nil {
				t.Fatalf("Test %d: Unexpected error when creating test file: %s", i, err)
			}
		}
		expected := addPathPrefix(tempdir, c.expected)
		sort.Strings(expected)

		actual, err := flattenFilenames(addPathPrefix(tempdir, c.inputFiles))
		if err != nil {
			t.Errorf("Test %d: Error unexpectedly returned: %s", i, err)
			continue
		}
		sort.Strings(actual)

		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("Test %d:\nExpected:\n%#v\nGot:\n%#v", i, expected, actual)
		}
	}
}

func TestGetPipelineConfigDir(t *testing.T) {
	cases := []struct {
		files  []string
		result error
	}{
		{
			[]string{"file1"},
			nil,
		},
		{
			[]string{"file1", "file2"},
			nil,
		},
		{
			[]string{"file1", "dir1/file2", "dir2/file3"},
			nil,
		},
		{
			[]string{"file1", "dir1/file1"},
			errors.New("some error message"),
		},
	}
	for i, c := range cases {
		// Create the files listed in the test case in a new
		// temporary directory. The content of each file is
		// the base of its own name.
		tempdir, err := ioutil.TempDir("", "")
		if err != nil {
			t.Fatalf("Test %d: Unexpected error when creating temp dir: %s", i, err)
		}
		defer os.RemoveAll(tempdir)

		resultDir, err := ioutil.TempDir("", "")
		if err != nil {
			t.Fatalf("Test %d: Unexpected error when creating temp dir: %s", i, err)
		}
		defer os.RemoveAll(resultDir)

		var configFiles []string
		for _, f := range c.files {
			err = os.MkdirAll(filepath.Join(tempdir, filepath.Dir(f)), 0700)
			if err != nil {
				t.Fatalf("Test %d: Unexpected error when creating temp dir: %s", i, err)
			}

			err = ioutil.WriteFile(filepath.Join(tempdir, f), []byte(createLogstashConfigWithString(filepath.Base(f))), 0600)
			if err != nil {
				t.Fatalf("Test %d: Unexpected error when writing to temp file: %s", i, err)
			}
			configFiles = append(configFiles, filepath.Join(tempdir, f))
		}

		// Call the function under test.
		actualResult := getPipelineConfigDir(resultDir, configFiles)
		if actualResult == nil && c.result != nil {
			t.Fatalf("Test %d: Expected failure, got success.", i)
		} else if actualResult != nil && c.result == nil {
			t.Fatalf("Test %d: Expected success, got this error instead: %#v", i, actualResult)
		}

		if actualResult != nil {
			continue
		}

		// Get a sorted list of names of the files in the
		// returned directory.
		actualConfigFiles, err := getFilesInDir(resultDir, false)
		if err != nil {
			t.Fatalf("Test %d: Unexpected error when reading dir: %s", i, err)
		}

		// Get a sorted list of the basenames of the input files.
		var expectedFiles []string
		for _, f := range c.files {
			expectedFiles = append(expectedFiles, filepath.Base(f))
		}
		sort.Strings(expectedFiles)

		if !reflect.DeepEqual(expectedFiles, actualConfigFiles) {
			t.Errorf("Test %d:\nExpected files:\n%#v\nGot:\n%#v", i, expectedFiles, actualConfigFiles)
		}

		// Check that each file contains its own name.
		for _, f := range actualConfigFiles {
			buf, err := ioutil.ReadFile(filepath.Join(resultDir, f))
			if err != nil {
				t.Fatalf("Test %d: Unexpected error when reading file: %s", i, err)
			}
			expected := createLogstashConfigWithString(f)
			if expected != string(buf) {
				t.Errorf("Test %d: Expected file contents:\n%#v\nGot:\n%#v", i, expected, string(buf))
			}
		}
	}
}

func TestGetFilesInDir(t *testing.T) {
	cases := []struct {
		files    []string
		dirs     []string
		expected []string
		result   error
	}{
		{
			[]string{},
			[]string{},
			[]string{},
			nil,
		},
		{
			[]string{"file1", "file2"},
			[]string{},
			[]string{"file1", "file2"},
			nil,
		},
		{
			[]string{"file1", "file2"},
			[]string{"dir1", "dir2"},
			[]string{"file1", "file2"},
			nil,
		},
	}
	for i, c := range cases {
		tempdir, err := ioutil.TempDir("", "")
		if err != nil {
			t.Fatalf("Test %d: Unexpected error when creating temp dir: %s", i, err)
		}
		defer os.RemoveAll(tempdir)

		for _, filename := range c.files {
			f, err := os.Create(filepath.Join(tempdir, filename))
			if err != nil {
				t.Fatalf("Test %d: Unexpected error when creating temp file: %s", i, err)
			}
			f.Close()
		}
		for _, filename := range c.dirs {
			err := os.MkdirAll(filepath.Join(tempdir, filename), 0700)
			if err != nil {
				t.Fatalf("Test %d: Unexpected error when creating temp dir: %s", i, err)
			}
		}

		// Call the function under test.
		files, actualResult := getFilesInDir(tempdir, false)
		if actualResult == nil && c.result != nil {
			t.Fatalf("Test %d: Expected failure, got success.", i)
		} else if actualResult != nil && c.result == nil {
			t.Fatalf("Test %d: Expected success, got this error instead: %#v", i, actualResult)
		}

		if actualResult != nil {
			continue
		}

		if !reflect.DeepEqual(c.expected, files) {
			t.Errorf("Test %d:\nExpected files:\n%#v\nGot:\n%#v", i, c.expected, files)
		}
	}
}

func TestRemoveInputOutput(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{
			// Input, output, and filter
			"input { beats { port => 5044 } } filter { grok {} } output{ elasticsearch {} }",
			"filter {  grok {      }}",
		},
		{
			// Input and filter
			"input { beats { port => 5044 } } filter { grok {} }",
			"filter {  grok {      }}",
		},
		{
			// Output and filter
			"filter { grok {} } output{ elasticsearch {} }",
			"filter {  grok {      }}",
		},
		{
			// Filter only
			"filter { grok {} }",
			"filter {  grok {      }}",
		},
		{
			// Empty file
			"",
			"",
		},
	}
	for i, c := range cases {
		f, err := ioutil.TempFile("", "lsconf")
		if err != nil {
			t.Fatalf("Test %d: Unexpected error when creating temp file: %s", i, err)
		}

		path := f.Name()
		defer os.Remove(path)

		err = ioutil.WriteFile(path, []byte(c.input), 0600)
		if err != nil {
			t.Fatalf("Test %d: Unexpected error when writing to temp file: %s", i, err)
		}

		err = removeInputOutput(path)
		if err != nil {
			t.Errorf("Test %d: Unexpected error returned: %s", i, err)
			continue
		}

		data, err := ioutil.ReadFile(path)
		if err != nil {
			t.Error(err)
		}

		actual := strings.Replace(string(data), "\n", "", -1)
		if actual != c.expected {
			t.Errorf("Test %d: Expected:\n%#v\nGot:\n%#v", i, c.expected, actual)
		}
	}
}
