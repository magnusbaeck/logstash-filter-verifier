// Copyright (c) 2016 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestCopyFile(t *testing.T) {
	testData := "random string\n"

	// Create the source file.
	source, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("Unexpected error when creating file: %s", err)
	}
	defer os.Remove(source.Name())
	source.Write([]byte(testData))

	// Create the destination directory.
	tempdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Unexpected error when creating temp dir: %s", err)
	}
	defer os.RemoveAll(tempdir)
	destPath := filepath.Join(tempdir, "arbitrary-filename")

	err = copyFile(source.Name(), destPath)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	buf, err := ioutil.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Unexpected error reading destination file: %s", err)
	}
	actualData := string(buf)
	if actualData != testData {
		t.Fatalf("Destination file contained %q after copying, expected %q.", actualData, testData)
	}
}

func TestGetConfigFileDir(t *testing.T) {
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

		var configFiles []string
		for _, f := range c.files {
			err = os.MkdirAll(filepath.Join(tempdir, filepath.Dir(f)), 0755)
			if err != nil {
				t.Fatalf("Test %d: Unexpected error when creating temp dir: %s", i, err)
			}
			err = ioutil.WriteFile(filepath.Join(tempdir, f), []byte(filepath.Base(f)), 0644)
			if err != nil {
				t.Fatalf("Test %d: Unexpected error when writing to temp file: %s", i, err)
			}
			configFiles = append(configFiles, filepath.Join(tempdir, f))
		}

		// Call the function under test.
		dir, actualResult := getConfigFileDir(configFiles)
		if dir != "" {
			defer os.RemoveAll(dir)
		}
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
		actualConfigFiles, err := getFilesInDir(dir)
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
			buf, err := ioutil.ReadFile(filepath.Join(dir, f))
			if err != nil {
				t.Fatalf("Test %d: Unexpected error when reading file: %s", i, err)
			}
			if f != string(buf) {
				t.Errorf("Test %d:\nExpected file contents:\n%#v\nGot:\n%#v", i, f, string(buf))
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
