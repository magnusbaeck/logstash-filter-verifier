// Copyright (c) 2015-2016 Magnus Bäck <magnus@noun.se>

package testcase

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var supportedFileExtensions = []string{"json", "yml", "yaml"}

func inList(allowed []string, search string) (result bool) {
	for _, allowedEntry := range allowed {
		if search == allowedEntry {
			return true
		}
	}
	return false
}

// DiscoverTests reads a test case JSON or YAML file and returns a slice of
// TestCase structs or, if the input path is a directory, reads all
// .json/.yaml/.yml files in that directory and returns them as TestCase
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

	var result []TestCaseSet
	for _, f := range files {
		if f.IsDir() {
			continue
		}

		extensionCaseInsensitive := strings.ToLower(filepath.Ext(f.Name()))
		if len(extensionCaseInsensitive) == 0 {
			continue
		}

		extensionDotlessCaseInsensitive := extensionCaseInsensitive[1:]
		if !inList(supportedFileExtensions, extensionDotlessCaseInsensitive) {
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
