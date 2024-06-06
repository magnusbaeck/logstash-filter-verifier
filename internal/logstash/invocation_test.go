// Copyright (c) 2017 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	semver "github.com/Masterminds/semver/v3"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/testhelpers"
)

func TestArgs(t *testing.T) {
	tinv, err := createTestInvocation(t, *semver.MustParse("6.0.0"))
	if err != nil {
		t.Fatalf("%s", err)
	}

	args, err := tinv.Inv.Args("input", "output")
	if err != nil {
		t.Fatalf("Error generating args: %s", err)
	}
	options := simpleOptionParser(args)
	configOption, exists := options["-f"]
	if !exists {
		t.Fatalf("no -f option found")
	}
	configDir, err := os.Open(configOption)
	if err != nil {
		t.Fatalf("Error opening configuration file directory: %s", err)
	}

	files, err := configDir.Readdirnames(0)
	if err != nil {
		t.Fatalf("Error reading configuration file directory: %s", err)
	}

	// Three aspects of the pipeline config file directory concern us:
	//   - The file that normally contains filters exists and has the
	//     expected contents.
	//   - The file with the inputs and outputs exists and has the
	//     expected contents.
	//   - No other files are present.
	var filterOk bool
	var ioOk bool
	for _, file := range files {
		buf, err := ioutil.ReadFile(filepath.Join(configOption, file))
		if err != nil {
			t.Errorf("Error reading configuration file: %s", err)
			continue
		}
		fileContents := string(buf)

		// Filter configuration file.
		if file == tinv.configFile {
			if fileContents != tinv.configContents {
				t.Errorf("Filter configuration file didn't contain the expected data.\nExpected: %q\nGot: %q", tinv.configContents, fileContents)
			}
			filterOk = true
			continue
		}

		// Input/Output configuration file.
		if file == filepath.Base(tinv.Inv.ioConfigFile.Name()) {
			expectedIoConfig := "input\noutput"
			if fileContents != expectedIoConfig {
				t.Errorf("Input/output configuration file didn't contain the expected data.\nExpected: %q\nGot: %q",
					expectedIoConfig, fileContents)
			}
			ioOk = true
			continue
		}

		// We should never get here.
		t.Errorf("Unexpected file found: %s", file)
	}

	if !filterOk {
		t.Errorf("No filter configuration file found in %s: %v", configOption, files)
	}
	if !ioOk {
		t.Errorf("No input/output configuration file found in %s: %v", configOption, files)
	}
}

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
		tinv, err := createTestInvocation(t, *semver.MustParse(c.version))
		if err != nil {
			t.Errorf("Test %d: Error unexpectedly returned: %s", i, err)
			continue
		}
		defer tinv.Release()

		args, err := tinv.Inv.Args("input", "output")
		if err != nil {
			t.Errorf("Test %d: Error generating args: %s", i, err)
		}
		err = c.optionTests(simpleOptionParser(args))
		if err != nil {
			t.Errorf("Test %d: Command option test failed for %v: %s", i, args, err)
		}
	}
}

type testInvocation struct {
	Inv            *Invocation
	tempdir        string
	configFile     string
	configContents string
}

func createTestInvocation(t *testing.T, version semver.Version) (*testInvocation, error) {
	tempdir := t.TempDir()

	files := []testhelpers.FileWithMode{
		{"bin", os.ModeDir | 0755, ""},
		{"bin/logstash", 0755, ""},
		{"config", os.ModeDir | 0755, ""},
		{"config/jvm.options", 0644, ""},
		{"config/log4j2.properties", 0644, ""},
	}
	for _, fwm := range files {
		if err := fwm.Create(tempdir); err != nil {
			return nil, fmt.Errorf("Unexpected error when creating test file: %s", err)
		}
	}

	configFile := filepath.Join(tempdir, "configfile.conf")
	configContents := ""
	if err := ioutil.WriteFile(configFile, []byte(configContents), 0600); err != nil {
		return nil, fmt.Errorf("Unexpected error when creating dummy configuration file: %s", err)
	}
	logstashPath := filepath.Join(tempdir, "bin/logstash")
	inv, err := NewInvocation(logstashPath, []string{}, &version, configFile)
	if err != nil {
		return nil, fmt.Errorf("Unexpected error when creating Invocation: %s", err)
	}

	return &testInvocation{inv, tempdir, filepath.Base(configFile), configContents}, nil
}

func (ti *testInvocation) Release() {
	ti.Inv.Release()
	_ = os.RemoveAll(ti.tempdir)
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

		if i+1 < len(args) && args[i+1][0] != '-' {
			result[args[i]] = args[i+1]
			i++
		} else {
			result[args[i]] = ""
		}
	}
	return result
}
