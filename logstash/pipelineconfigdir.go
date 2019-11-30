// Copyright (c) 2016-2018 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"sort"

	lsparser "github.com/breml/logstash-config"
	"github.com/breml/logstash-config/ast"
)

// flattenFilenames flattens a list of files and directories so it
// only contains the given files and the files found in the
// directories. The flattening of directories isn't recursive,
// i.e. files in subdirectories of the provided directories aren't
// included.
func flattenFilenames(filenames []string) ([]string, error) {
	result := make([]string, 0, len(filenames))
	for _, f := range filenames {
		stat, err := os.Stat(f)
		if err != nil {
			return result, err
		}
		if stat.IsDir() {
			subdirFiles, err := getFilesInDir(f, true)
			if err != nil {
				return result, err
			}
			result = append(result, subdirFiles...)
		} else {
			result = append(result, f)
		}
	}
	return result, nil
}

// getPipelineConfigDir copies one or more Logstash pipeline
// configuration files into the root of the specified directory.
// Returns an error if any I/O error occurs but also if the
// basenames of the configuration files aren't unique, i.e. if
// they'd overwrite one another in the directory.
func getPipelineConfigDir(dir string, configs []string) error {
	allFiles, err := flattenFilenames(configs)
	if err != nil {
		return fmt.Errorf("Error listing configuration files: %s", err)
	}
	log.Debugf("Preparing configuration file directory %s with these files: %v", dir, allFiles)
	for _, f := range allFiles {
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

		err = removeInputOutput(dest)
		if err != nil {
			_ = os.RemoveAll(dir)
			return fmt.Errorf("Failed to remove the input and output sections: %s", err)
		}
	}
	return nil
}

// getFilesInDir returns a sorted list of the names of the
// (non-directory) files in the given directory. If includeDir
// is true the returned path will include the directory name.
func getFilesInDir(dir string, includeDir bool) ([]string, error) {
	filenames := make([]string, 0)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if !f.Mode().IsDir() {
			if includeDir {
				filenames = append(filenames, filepath.Join(dir, f.Name()))
			} else {
				filenames = append(filenames, f.Name())
			}
		}
	}
	sort.Strings(filenames)
	return filenames, nil
}

// getTempFileWithSuffix creates a new temporary file with a unique
// name in the supplied directory and with the supplied suffix. It
// basically does what tempfile.TempFile does except it allows the
// caller to set the prefix (required for Logstash 6+ to read a
// configuration file). The directory may be empty, in which case
// os.TempDir is used.
func getTempFileWithSuffix(dir string, suffix string) (string, error) {
	if dir == "" {
		dir = os.TempDir()
	}
	maxAttempts := 1000
	for i := 0; i < maxAttempts; i++ {
		filename := filepath.Join(dir, fmt.Sprintf("%x%s", rand.Uint32(), suffix))
		f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		if os.IsExist(err) {
			continue
		} else if err != nil {
			return "", err
		}
		defer f.Close()
		return filename, nil
	}
	return "", fmt.Errorf("unable to generate a temporary filename despite %d attempts", maxAttempts)
}

// removeInputOutput removes the input and output sections in the
// given logstash configuration file. The operation is done in place
// and the original file content is replaced.
func removeInputOutput(path string) error {
	parsed, err := lsparser.ParseFile(path)
	if err != nil {
		return err
	}

	if parsed == nil {
		return fmt.Errorf("could not parse the following Logstash config file: %v", path)
	}

	config := parsed.(ast.Config)
	config.Input = nil
	config.Output = nil

	return ioutil.WriteFile(path, []byte(config.String()), 0644)
}
