// Copyright (c) 2017 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/blang/semver"
	"github.com/magnusbaeck/logstash-filter-verifier/testhelpers"
)

func TestNewInvocation(t *testing.T) {
	cases := []struct {
		version     string
		optionTests func(options map[string]string) error
	}{
		// Logstash 2.4 gets a regular file as a log file argument.
		{
			"2.4.0",
			func(options map[string]string) error {
				logOption, exists := options["-l"]
				if !exists {
					return errors.New("no logfile option found")
				}
				fi, err := os.Stat(logOption)
				if err != nil {
					return fmt.Errorf("could not stat logfile: %s", err)
				}
				if !fi.Mode().IsRegular() {
					return fmt.Errorf("log path not a regular file: %s", fi.Name())
				}

				if _, exists = options["--path.settings"]; exists {
					return errors.New("unsupported --path.settings option provided")
				}
				return nil
			},
		},
		// Logstash 5.0 gets a directory as a log file
		// argument and --path.settings pointing to a
		// directory with the expected files.
		{
			"5.0.0",
			func(options map[string]string) error {
				logOption, exists := options["-l"]
				if !exists {
					return errors.New("no logfile option found")
				}
				fi, err := os.Stat(logOption)
				if err != nil {
					return fmt.Errorf("could not stat logfile: %s", err)
				}
				if fi.Mode().IsRegular() {
					return fmt.Errorf("log path not a regular file: %s", fi.Name())
				}

				pathOption, exists := options["--path.settings"]
				if !exists {
					return errors.New("--path.settings option missing")
				}
				requiredFiles := []string{
					"jvm.options",
					"log4j2.properties",
					"logstash.yml",
				}
				if !allFilesExist(pathOption, requiredFiles) {
					return fmt.Errorf("Not all required files found in %q: %v",
						pathOption, requiredFiles)
				}

				return nil
			},
		},
	}
	for i, c := range cases {
		tempdir, err := ioutil.TempDir("", "")
		if err != nil {
			t.Fatalf("Test %d: Unexpected error when creating temp dir: %s", i, err)
		}
		defer os.RemoveAll(tempdir)

		files := []testhelpers.FileWithMode{
			{"bin", os.ModeDir | 0755, ""},
			{"bin/logstash", 0755, ""},
			{"config", os.ModeDir | 0755, ""},
			{"config/jvm.options", 0644, ""},
			{"config/log4j2.properties", 0644, ""},
		}
		for _, fwm := range files {
			if err = fwm.Create(tempdir); err != nil {
				t.Fatalf("Test %d: Unexpected error when creating test file: %s", i, err)
			}
		}

		version, err := semver.New(c.version)
		if err != nil {
			t.Fatalf("Test %d: Unexpected error when parsing version number: %s", i, err)
		}
		configFile, err := newDeletedTempFile("", "")
		if err != nil {
			t.Fatalf("Test %d: Unexpected error when creating tempfile: %s", i, err)
		}
		defer configFile.Close()
		logstashPath := filepath.Join(tempdir, "bin/logstash")
		inv, err := NewInvocation(logstashPath, []string{}, version, configFile.Name())
		if err != nil {
			t.Fatalf("Test %d: Unexpected error when creation Invocation: %s", i, err)
		}
		defer inv.Release()

		err = c.optionTests(simpleOptionParser(inv.args))
		if err != nil {
			t.Errorf("Test %d: Command option test failed for %v: %s", i, inv.args, err)
		}
	}

}

// simpleOptionParser is a super-simple command line option parser
// that just builds a map of all the options and their values. For
// options not taking any arguments the option's value will be an
// empty string.
func simpleOptionParser(args []string) map[string]string {
	result := map[string]string{}
	for i := 0; i < len(args); i++ {
		if args[i][0] != '-' {
			continue
		}

		if i+1 < len(args) && args[i+1] != "-" {
			result[args[i]] = args[i+1]
			i++
		} else {
			result[args[i]] = ""
		}
	}
	return result
}
