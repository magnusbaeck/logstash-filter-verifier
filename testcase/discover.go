// Copyright (c) 2015-2016 Magnus Bäck <magnus@noun.se>

package testcase

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// DiscoverTests reads a test case JSON file or YAML file and returns a slice of
// TestCaseSet structs or, if the input path is a directory, reads all
// .json or .yaml or .yml files in that directorory and returns them as TestCaseSet
// structs.
func DiscoverTests(path string) ([]TestCaseSet, error) {
	pathinfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if pathinfo.IsDir() {
		return discoverTestDirectory(path)
	}
	return discoverTestFile(path)
}

func discoverTestDirectory(path string) ([]TestCaseSet, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("Error discovering test case files: %s", err)
	}
	result := make([]TestCaseSet, 0, len(files))
	for _, f := range files {
		if f.IsDir() || (!strings.HasSuffix(f.Name(), ".json") && !strings.HasSuffix(f.Name(), ".yaml") && !strings.HasSuffix(f.Name(), ".yml")) {
			continue
		}
		fullpath := filepath.Join(path, f.Name())

		tcs, err := NewFromFile(fullpath)
		if err != nil {
			return nil, err
		}
		result = append(result, *tcs)
	}
	return result, nil
}

func discoverTestFile(path string) ([]TestCaseSet, error) {
	tcs, err := NewFromFile(path)
	if err != nil {
		return nil, err
	}
	return []TestCaseSet{*tcs}, nil
}
