// Copyright (c) 2017 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/blang/semver"
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
				return nil
			},
		},
		// Logstash 5.0 gets a directory as a log file argument.
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
				return nil
			},
		},
	}
	for i, c := range cases {
		version, err := semver.New(c.version)
		if err != nil {
			t.Fatalf("Test %d: Unexpected error when parsing version number: %s", i, err)
		}
		configFile, err := newDeletedTempFile("", "")
		if err != nil {
			t.Fatalf("Test %d: Unexpected error when creating tempfile: %s", i, err)
		}
		defer configFile.Close()
		inv, err := NewInvocation("/path/to/logstash", []string{}, version, configFile.Name())
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
