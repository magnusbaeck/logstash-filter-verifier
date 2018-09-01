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
	"github.com/magnusbaeck/logstash-filter-verifier/testhelpers"
)

func createLogstashConfigWithString(s string) string {
	plugin := ast.NewPlugin("mutate", ast.NewStringAttribute("id", s, ast.DoubleQuoted))
	conf := ast.NewConfig(nil, []ast.PluginSection{ast.NewPluginSection(ast.Filter, plugin)}, nil)
	return conf.String()
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
			err = os.MkdirAll(filepath.Join(tempdir, filepath.Dir(f)), 0755)
			if err != nil {
				t.Fatalf("Test %d: Unexpected error when creating temp dir: %s", i, err)
			}

			err = ioutil.WriteFile(filepath.Join(tempdir, f), []byte(createLogstashConfigWithString(filepath.Base(f))), 0644)
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
		actualConfigFiles, err := getFilesInDir(resultDir)
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
			err := os.MkdirAll(filepath.Join(tempdir, filename), 0755)
			if err != nil {
				t.Fatalf("Test %d: Unexpected error when creating temp dir: %s", i, err)
			}
		}

		// Call the function under test.
		files, actualResult := getFilesInDir(tempdir)
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

func TestRemoveInputOutputBasicConfig(t *testing.T) {
	f, err := ioutil.TempFile("", "lsconf")
	if err != nil {
		t.Error(err)
	}

	path := f.Name()
	defer os.Remove(path)

	err = ioutil.WriteFile(path, []byte("input { beats { port => 5044 } } filter { grok {} } output{ elasticsearch {} }"), 0644)
	if err != nil {
		t.Error(err)
	}

	err = removeInputOutput(path)
	if err != nil {
		t.Error(err)
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		t.Error(err)
	}

	actual := strings.Replace(string(data), "\n", "", -1)
	testhelpers.AssertEqual(t, "filter {  grok {      }}", actual)
}

func TestRemoveInputOutputEmptyFile(t *testing.T) {
	f, err := ioutil.TempFile("", "lsconf")
	if err != nil {
		t.Error(err)
	}

	path := f.Name()
	defer os.Remove(path)

	err = removeInputOutput(path)
	if err != nil {
		t.Error(err)
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		t.Error(err)
	}

	testhelpers.AssertEqual(t, "", string(data))
}
