// Copyright (c) 2015-2016 Magnus Bäck <magnus@noun.se>

package testcase

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// DiscoverTests reads a test case JSON file and returns a slice of
// TestCase structs or, if the input path is a directory, reads all
// .json files in that directorory and returns them as TestCase
// structs.
func DiscoverTests(path string) ([]TestCase, error) {
	pathinfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if pathinfo.IsDir() {
		return discoverTestDirectory(path)
	}
	return discoverTestFile(path)
}

func discoverTestDirectory(path string) ([]TestCase, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("Error discovering test case files: %s", err)
	}
	var result []TestCase
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		fullpath := filepath.Join(path, f.Name())
		tc, err := NewFromFile(fullpath)
		if err != nil {
			return nil, err
		}
		result = append(result, *tc)
	}
	return result, nil
}

func discoverTestFile(path string) ([]TestCase, error) {
	tc, err := NewFromFile(path)
	if err != nil {
		return nil, err
	}
	return []TestCase{*tc}, nil
}
