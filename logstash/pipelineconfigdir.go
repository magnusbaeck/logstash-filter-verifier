// Copyright (c) 2016 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
)

// getPipelineConfigDir copies one or more Logstash pipeline
// configuration files into the root of the specified directory.
// Returns an error if any I/O error occurs but also if the
// basenames of the configuration files aren't unique, i.e. if
// they'd overwrite one another in the directory.
func getPipelineConfigDir(dir string, configs []string) error {
	for _, f := range configs {
		dest := filepath.Join(dir, filepath.Base(f))
		_, err := os.Stat(dest)
		if err == nil {
			_ = os.RemoveAll(dir)
			return fmt.Errorf(
				"The collected list of configuration files contains "+
					"two files with the name %q which isn't allowed.",
				filepath.Base(f))
		} else if !os.IsNotExist(err) {
			_ = os.RemoveAll(dir)
			return err
		}
		err = copyFile(f, dest)
		if err != nil {
			_ = os.RemoveAll(dir)
			return fmt.Errorf("Config file copy failed: %s", err)
		}
	}
	fileList, err := getFilesInDir(dir)
	if err == nil {
		log.Debug("Prepared configuration file directory %s with these files: %v", dir, fileList)
	} else {
		// Don't let this failure fail the whole function
		// call, but log it as a warning since it's highly
		// irregular and might be indicative of other
		// problems.
		log.Warning("Unexpected error when locating configuration files: %s", err)
	}
	return nil
}

// getFilesInDir returns a sorted list of the names of the
// (non-directory) files in the given directory.
func getFilesInDir(dir string) ([]string, error) {
	filenames := make([]string, 0)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if !f.Mode().IsDir() {
			filenames = append(filenames, f.Name())
		}
	}
	sort.Strings(filenames)
	return filenames, nil
}
