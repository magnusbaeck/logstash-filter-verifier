// Copyright (c) 2017 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"fmt"
	"os"
	"testing"

	"github.com/blang/semver"
)

func TestNewInvocation(t *testing.T) {
	cases := []struct {
		version       string
		logOptionTest func(os.FileInfo) error
	}{
		// Logstash 2.4 gets a regular file as a log file argument.
		{
			"2.4.0",
			func(fi os.FileInfo) error {
				if !fi.Mode().IsRegular() {
					return fmt.Errorf("log path not a regular file: %s", fi.Name())
				}
				return nil
			},
		},
		// Logstash 5.0 gets a directory as a log file argument.
		{
			"5.0.0",
			func(fi os.FileInfo) error {
				if !fi.Mode().IsDir() {
					return fmt.Errorf("log path not a directory: %s", fi.Name())
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

		var logOption string
		for j := 0; j < len(inv.args)-1; j++ {
			if inv.args[j] == "-l" {
				logOption = inv.args[j+1]
				j++
			}
		}

		if logOption != "" {
			fi, err := os.Stat(logOption)
			if err != nil {
				t.Errorf("could not stat logfile: %s", err)
			} else {
				err = c.logOptionTest(fi)
				if err != nil {
					t.Errorf("Test %d: Bad logfile option: %s", i, err)
				}
			}
		} else {
			t.Errorf("Test %d: No logfile option found in args: %v", i, inv.args)
		}
	}

}
