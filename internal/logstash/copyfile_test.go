// Copyright (c) 2016-2017 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/testhelpers"
)

func TestAllFilesExist(t *testing.T) {
	cases := []struct {
		files       []testhelpers.FileWithMode
		tempdirMode os.FileMode
		input       []string
		expected    bool
	}{
		// All files exist
		{
			[]testhelpers.FileWithMode{
				{"a", 0644, ""},
				{"b", 0644, ""},
				{"c", 0644, ""},
			},
			0755,
			[]string{"a", "b", "c"},
			true,
		},
		// Some files exist.
		{
			[]testhelpers.FileWithMode{
				{"a", 0644, ""},
				{"b", 0644, ""},
			},
			0755,
			[]string{"a", "b", "c"},
			false,
		},
		// No files exist.
		{
			[]testhelpers.FileWithMode{
				{"a", 0644, ""},
				{"b", 0644, ""},
				{"c", 0644, ""},
			},
			0755,
			[]string{"1", "2", "3"},
			false,
		},
		// All files exist but parent directory is inaccessible.
		{
			[]testhelpers.FileWithMode{
				{"a", 0644, ""},
				{"b", 0644, ""},
				{"c", 0644, ""},
			},
			0,
			[]string{"a", "b", "c"},
			false,
		},
		// File is inaccessible.
		{
			[]testhelpers.FileWithMode{
				{"a", 0, ""},
				{"b", 0644, ""},
				{"c", 0644, ""},
			},
			0755,
			[]string{"a", "b", "c"},
			true,
		},
		// File is a directory.
		{
			[]testhelpers.FileWithMode{
				{"a", os.ModeDir | 0644, ""},
				{"b", 0644, ""},
				{"c", 0644, ""},
			},
			0755,
			[]string{"a", "b", "c"},
			true,
		},
	}
	for i, c := range cases {
		tempdir := t.TempDir()
		// The test fails if it can't clean up the directory.
		defer os.Chmod(tempdir, 0700)

		for _, fwm := range c.files {
			if err := fwm.Create(tempdir); err != nil {
				t.Fatalf("Test %d: Unexpected error when creating test file: %s", i, err)
			}
		}
		if err := os.Chmod(tempdir, c.tempdirMode); err != nil {
			t.Fatalf("Test %d: Unexpected error when chmod'ing temp dir: %s", i, err)
		}

		result := allFilesExist(tempdir, c.input)
		if c.expected != result {
			t.Errorf("Test %d:\nExpected:\n%v\nGot:\n%v", i, c.expected, result)
		}
	}
}

func TestCopyAllFiles(t *testing.T) {
	cases := []struct {
		// Outer slice is for the different numbered source
		// directories.
		files  [][]testhelpers.FileWithMode
		wanted []string
		// Basename of expected directory.
		expected string
		err      error
	}{
		// Single source directory.
		{
			[][]testhelpers.FileWithMode{
				{
					{"a", 0644, "a"},
				},
			},
			[]string{"a"},
			"0",
			nil,
		},
	}
	for i, c := range cases {
		tempdir := t.TempDir()
		destdir := filepath.Join(tempdir, "dest")
		if err := os.Mkdir(destdir, 0755); err != nil {
			t.Fatalf("Test %d: Unexpected error when creating temp dir: %s", i, err)
		}

		sourcedirs := make([]string, len(c.files))
		for diridx, files := range c.files {
			thisdir := filepath.Join(tempdir, strconv.Itoa(diridx))
			sourcedirs[diridx] = thisdir
			if err := os.Mkdir(thisdir, 0755); err != nil {
				t.Fatalf("Test %d: Unexpected error when creating temp dir: %s", i, err)
			}
			for _, fwm := range files {
				if err := fwm.Create(thisdir); err != nil {
					t.Fatalf("Test %d: Unexpected error when creating test file: %s", i, err)
				}
			}
		}

		result, err := copyAllFiles(sourcedirs, c.wanted, destdir)
		testhelpers.CompareErrors(t, i, c.err, err)

		resultBase := filepath.Base(result)
		if c.expected != resultBase {
			t.Errorf("Test %d:\nExpected:\n%v\nGot:\n%v", i, c.expected, result)
		}

		// Are all files copied and do they contain the
		// expected contents (the filepath's basename)?
		for _, filename := range c.wanted {
			buf, err := os.ReadFile(filepath.Join(destdir, filename))
			if err != nil {
				t.Errorf("Test %d: Got error reading copied file: %s", i, err)
			}
			if string(buf) != filename {
				t.Errorf("Test %d: File didn't contain the expected data.\nExpected: %q\nGot: %q", i, filename, string(buf))
			}
		}
	}
}

func TestCopyFile(t *testing.T) {
	testData := "random string\n"

	// Create the source file.
	source, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("Unexpected error when creating file: %s", err)
	}
	defer os.Remove(source.Name())
	source.Write([]byte(testData))

	destPath := filepath.Join(t.TempDir(), "arbitrary-filename")
	if err = copyFile(source.Name(), destPath); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	buf, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Unexpected error reading destination file: %s", err)
	}
	actualData := string(buf)
	if actualData != testData {
		t.Fatalf("Destination file contained %q after copying, expected %q.", actualData, testData)
	}
}
