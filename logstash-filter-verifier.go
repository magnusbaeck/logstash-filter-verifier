// Copyright (c) 2015-2016 Magnus Bäck <magnus@noun.se>

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/alecthomas/kingpin"
	"github.com/magnusbaeck/logstash-filter-verifier/logging"
	"github.com/magnusbaeck/logstash-filter-verifier/logstash"
	"github.com/magnusbaeck/logstash-filter-verifier/testcase"
	"github.com/mattn/go-shellwords"
	oplogging "github.com/op/go-logging"
)

var (
	log = logging.MustGetLogger()

	loglevels = []string{"CRITICAL", "ERROR", "WARNING", "NOTICE", "INFO", "DEBUG"}

	// Flags
	diffCommand = kingpin.
			Flag("diff-command", "Set the command to run to compare two events. The command will receive the two files to compare as arguments.").
			Default("diff -u").
			String()
	keptEnvVars = kingpin.
			Flag("keep-env", "Add this environment variable to the list of variables that will be preserved from the calling process's environment. Defaults to an empty list, except TZ which will be set to \"UTC\".").
			PlaceHolder("VARNAME").
			Strings()
	loglevel = kingpin.
			Flag("loglevel", fmt.Sprintf("Set the desired level of logging (one of: %s).", strings.Join(loglevels, ", "))).
			Default("WARNING").
			Enum(loglevels...)
	logstashPath = kingpin.
			Flag("logstash-path", "Set the path to the Logstash executable.").
			Default("/opt/logstash/bin/logstash").
			String()
	unixSockets = kingpin.
			Flag("sockets", "Use Unix domain sockets for the communication with Logstash.").
			Default("false").
			Bool()
	unixSocketCommTimeout = kingpin.
				Flag("sockets-timeout", "Timeout (duration) for the communication wit Logstash via Unix domain sockets.").
				Default("60s").
				Duration()
	logstashOutput = kingpin.
			Flag("logstash-output", "Print the debug output of logstash.").
			Default("false").
			Bool()
	logstashArgs = kingpin.
			Flag("logstash-arg", "Command line arguments, which are passed to Logstash. Flag and value have to be provided as a flag each, e.g.: --logstash-arg=-n --logstash-arg=InstanceName").
			Strings()

	// Arguments
	testcasePath = kingpin.
			Arg("testcases", "Test case file or a directory containing one or more test case files.").
			Required().
			ExistingFileOrDir()
	configPaths = kingpin.
			Arg("config", "Logstash configuration file or a directory containing one or more configuration files.").
			Required().
			ExistingFilesOrDirs()
)

// runTests runs Logstash with a set of configuration files against a
// slice of test cases and compares the actual events against the
// expected set. Returns an error if at least one test case fails or
// if there's a problem running the tests.
func runTests(logstashPath string, logstashArgs []string, tests []testcase.TestCaseSet, configPaths []string, diffCommand []string, keptEnvVars []string) error {
	ok := true
	for _, t := range tests {
		fmt.Printf("Running tests in %s...\n", filepath.Base(t.File))
		p, err := logstash.NewProcess(logstashPath, logstashArgs, t.Codec, t.InputFields, keptEnvVars, configPaths...)
		if err != nil {
			return err
		}
		defer p.Release()
		if err = p.Start(); err != nil {
			return err
		}

		for _, line := range t.InputLines {
			_, err = p.Input.Write([]byte(line + "\n"))
			if err != nil {
				return err
			}
		}
		if err = p.Input.Close(); err != nil {
			return err
		}

		result, err := p.Wait()
		if err != nil || *logstashOutput {
			var message string
			if err != nil {
				message += fmt.Sprintf("Error running Logstash: %s.", err)
			}
			if result.Output != "" {
				message += fmt.Sprintf("\nProcess output:\n%s", result.Output)
			} else {
				message += "\nThe process wrote nothing to stdout or stderr."
			}
			if result.Log != "" {
				message += fmt.Sprintf("\nLog:\n%s", result.Log)
			} else {
				message += "\nThe process wrote nothing to its logfile."
			}
			if err != nil {
				return errors.New(message)
			}
			userError("%s", message)
		}
		if err = t.Compare(result.Events, false, diffCommand); err != nil {
			userError("Testcase failed, continuing with the rest: %s", err.Error())
			ok = false
		}
	}
	if !ok {
		return errors.New("one or more testcases failed")
	}
	return nil
}

// runParallelTests runs multiple set of configuration in a single
// instance of Logstash against a slice of test cases and compares
// the actual events against the expected set. Returns an error if
// at least one test case fails or if there's a problem running the tests.
func runParallelTests(logstashPath string, logstashArgs []string, tests []testcase.TestCaseSet, configPaths []string, diffCommand []string, keptEnvVars []string) error {
	var testStreams []*logstash.TestStream

	badCodecs := map[string]string{
		"json":  "json_lines",
		"plain": "line",
	}
	for _, t := range tests {
		if repl, ok := badCodecs[t.Codec]; ok {
			log.Warning(
				"The testcase file %q uses the %q codec. That codec "+
					"will most likely not work as expected when --sockets is used. Try %q instead.",
				t.File, t.Codec, repl)
		}
	}

	for _, t := range tests {
		ts, err := logstash.NewTestStream(t.Codec, t.InputFields, *unixSocketCommTimeout)
		if err != nil {
			logstash.CleanupTestStreams(testStreams)
			return err
		}
		testStreams = append(testStreams, ts)
	}

	p, err := logstash.NewParallelProcess(logstashPath, logstashArgs, testStreams, keptEnvVars, configPaths...)
	if err != nil {
		return err
	}
	defer p.Release()
	if err = p.Start(); err != nil {
		return err
	}

	for i, t := range tests {
		for _, line := range t.InputLines {
			_, err = testStreams[i].Write([]byte(line + "\n"))
			if err != nil {
				return err
			}
		}

		if err = testStreams[i].Close(); err != nil {
			return err
		}
	}

	result, err := p.Wait()
	if err != nil || *logstashOutput {
		var message string
		if err != nil {
			message += fmt.Sprintf("Error running Logstash: %s.", err)
		}
		if result.Output != "" {
			message += fmt.Sprintf("\nProcess output:\n%s", result.Output)
		} else {
			message += "\nThe process wrote nothing to stdout or stderr."
		}
		if result.Log != "" {
			message += fmt.Sprintf("\nLog:\n%s", result.Log)
		} else {
			message += "\nThe process wrote nothing to its logfile."
		}
		if err != nil {
			return errors.New(message)
		}
		userError("%s", message)
	}
	ok := true
	for i, t := range tests {
		if err = t.Compare(result.Events[i], false, diffCommand); err != nil {
			userError("Testcase failed, continuing with the rest: %s", err.Error())
			ok = false
		}
	}

	if !ok {
		return errors.New("one or more testcases failed")
	}
	return nil
}

// prefixedUserError prints an error message to stderr and prefixes it
// with the name of the program file (e.g. "logstash-filter-verifier:
// something bad happened.").
func prefixedUserError(format string, a ...interface{}) {
	basename := filepath.Base(os.Args[0])
	message := fmt.Sprintf(format, a...)
	if strings.HasSuffix(message, "\n") {
		fmt.Fprintf(os.Stderr, "%s: %s", basename, message)
	} else {
		fmt.Fprintf(os.Stderr, "%s: %s\n", basename, message)
	}
}

// userError prints an error message to stderr.
func userError(format string, a ...interface{}) {
	if strings.HasSuffix(format, "\n") {
		fmt.Fprintf(os.Stderr, format, a...)
	} else {
		fmt.Fprintf(os.Stderr, format+"\n", a...)
	}
}

func main() {
	kingpin.Version(fmt.Sprintf("%s %s", kingpin.CommandLine.Name, version))
	kingpin.Parse()

	level, err := oplogging.LogLevel(*loglevel)
	if err != nil {
		prefixedUserError("Bad loglevel: %s", loglevel)
		os.Exit(1)
	}
	logging.SetLevel(level)

	diffCmd, err := shellwords.NewParser().Parse(*diffCommand)
	if err != nil {
		userError("Error parsing diff command %q: %s", *diffCommand, err.Error())
		os.Exit(1)
	}

	tests, err := testcase.DiscoverTests(*testcasePath)
	if err != nil {
		userError(err.Error())
		os.Exit(1)
	}
	if *unixSockets {
		if runtime.GOOS == "windows" {
			userError("Use of Unix domain sockets for communication with Logstash is not supported on Windows.")
			os.Exit(1)
		}
		fmt.Println("Use Unix domain sockets.")
		if err = runParallelTests(*logstashPath, *logstashArgs, tests, *configPaths, diffCmd, *keptEnvVars); err != nil {
			userError(err.Error())
			os.Exit(1)
		}
	} else {
		if err = runTests(*logstashPath, *logstashArgs, tests, *configPaths, diffCmd, *keptEnvVars); err != nil {
			userError(err.Error())
			os.Exit(1)
		}
	}
	os.Exit(0)
}
