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
// TestCase structs or, if the input path is a directory, reads all
// .json or .yaml files in that directorory and returns them as TestCase
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
		log.Debugf("Read file: %s", f.Name())
		if f.IsDir() || (!strings.HasSuffix(f.Name(), ".json") && !strings.HasSuffix(f.Name(), ".yaml")) {
			continue
		}
		fullpath := filepath.Join(path, f.Name())
		log.Debugf("Read file fullpath: %s", fullpath)
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
