// Copyright (c) 2015 Magnus Bäck <magnus@noun.se>

package testcase

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestDiscoverTests_File tests passing the path to a directory with
// files to DiscoverTests.
func TestDiscoverTests_Directory(t *testing.T) {
	cases := []struct {
		files    []string
		dirs     []string
		expected []string
	}{
		{
			files:    []string{},
			dirs:     []string{},
			expected: []string{},
		},
		{
			files:    []string{},
			dirs:     []string{"dir1", "dir2"},
			expected: []string{},
		},
		{
			files:    []string{"otherfile.txt", ".dotfile"},
			dirs:     []string{},
			expected: []string{},
		},
		{
			files:    []string{"test1.json", "test2.json", "test1.yaml", "test2.yaml"},
			dirs:     []string{},
			expected: []string{"test1.json", "test2.json", "test1.yaml", "test2.yaml"},
		},
		{
			files:    []string{"otherfile.txt", "test1.json", "test2.json", "test1.yaml", "test2.yaml"},
			dirs:     []string{},
			expected: []string{"test1.json", "test2.json", "test1.yaml", "test2.yaml"},
		},
	}
	for cnum, c := range cases {
		tempdir, err := ioutil.TempDir("", "")
		if err != nil {
			t.Errorf(err.Error())
			break
		}
		defer os.RemoveAll(tempdir)

		for _, f := range c.files {
			if strings.Contains(f, "/") {
				t.Errorf("This test doesn't support subdirectories: %s", f)
				break
			}
			if err = ioutil.WriteFile(filepath.Join(tempdir, f), []byte(`{"type": "test"}`), 0644); err != nil {
				t.Fatalf(err.Error())
			}
		}

		for _, d := range c.dirs {
			if strings.Contains(d, "/") {
				t.Errorf("This test doesn't support subdirectories: %s", d)
				break
			}
			if err := os.Mkdir(filepath.Join(tempdir, d), 0755); err != nil {
				t.Error(err.Error())
				break
			}
		}

		testcases, err := DiscoverTests(tempdir)
		if err != nil {
			t.Errorf("Test %d: DiscoverTests() unexpectedly returned an error: %s", cnum, err)
			break
		}

		filenames := make([]string, len(testcases))
		for i, tcs := range testcases {
			filenames[i] = filepath.Base(tcs.File)
		}
		sort.Strings(filenames)

		sexpected := make([]string, len(c.expected))
		copy(sexpected, c.expected)
		sort.Strings(sexpected)

		if len(filenames) != len(sexpected) {
			t.Errorf("Test %d:\nExpected:\n%v\nGot:\n%v", cnum, sexpected, filenames)
			break
		}
		for i := range sexpected {
			if sexpected[i] != filenames[i] {
				t.Errorf("Test %d: Expected item %d to be %q, got %q instead.", cnum, i, sexpected[i], filenames[i])
			}
		}
	}
}

// TestDiscoverTests_File tests passing the path to a single file to
// DiscoverTests.
func TestDiscoverTests_File(t *testing.T) {

	// With JSON file
	tempfile, err := ioutil.TempFile("", "*.json")
	if err != nil {
		t.Fatalf(err.Error())
	}
	defer func() {
		tempfile.Close()
		os.Remove(tempfile.Name())
	}()
	if err = ioutil.WriteFile(tempfile.Name(), []byte(`{"type": "test"}`), 0644); err != nil {
		t.Fatalf(err.Error())
	}

	testcases, err := DiscoverTests(tempfile.Name())
	if err != nil {
		t.Fatalf("DiscoverTests() unexpectedly returned an error: %s", err)
	}

	if len(testcases) != 1 {
		t.Fatalf("DiscoverTests() unexpectedly returned %d test cases: %+v", len(testcases), testcases)
	}

	if testcases[0].File != tempfile.Name() {
		t.Fatalf("DiscoverTests() unexpectedly returned %d test cases: %+v", len(testcases), testcases)
	}

	// With YAML file
	tempfile, err = ioutil.TempFile("", "*.yaml")
	if err != nil {
		t.Fatalf(err.Error())
	}
	defer func() {
		tempfile.Close()
		os.Remove(tempfile.Name())
	}()
	if err = ioutil.WriteFile(tempfile.Name(), []byte(`{"type": "test"}`), 0644); err != nil {
		t.Fatalf(err.Error())
	}

	testcases, err = DiscoverTests(tempfile.Name())
	if err != nil {
		t.Fatalf("DiscoverTests() unexpectedly returned an error: %s", err)
	}

	if len(testcases) != 1 {
		t.Fatalf("DiscoverTests() unexpectedly returned %d test cases: %+v", len(testcases), testcases)
	}

	if testcases[0].File != tempfile.Name() {
		t.Fatalf("DiscoverTests() unexpectedly returned %d test cases: %+v", len(testcases), testcases)
	}
}
