// Copyright (c) 2015-2017 Magnus Bäck <magnus@noun.se>

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/alecthomas/kingpin"
	"github.com/blang/semver"
	"github.com/magnusbaeck/logstash-filter-verifier/logging"
	"github.com/magnusbaeck/logstash-filter-verifier/logstash"
	"github.com/magnusbaeck/logstash-filter-verifier/testcase"
	"github.com/mattn/go-shellwords"
	oplogging "github.com/op/go-logging"
)

var (
	log = logging.MustGetLogger()

	loglevels = []string{"CRITICAL", "ERROR", "WARNING", "NOTICE", "INFO", "DEBUG"}

	autoVersion = "auto"

	defaultKeptEnvVars = []string{
		"PATH",
	}
	defaultLogstashPaths = []string{
		"/opt/logstash/bin/logstash",
		"/usr/share/logstash/bin/logstash",
	}

	// Flags
	diffCommand = kingpin.
			Flag("diff-command", "Set the command to run to compare two events. The command will receive the two files to compare as arguments.").
			Default("diff -u").
			String()
	keptEnvVars = kingpin.
			Flag("keep-env", fmt.Sprintf("Add this environment variable to the list of variables that will be preserved from the calling process's environment. Initial list of variables: %s", strings.Join(defaultKeptEnvVars, ", "))).
			PlaceHolder("VARNAME").
			Strings()
	loglevel = kingpin.
			Flag("loglevel", fmt.Sprintf("Set the desired level of logging (one of: %s).", strings.Join(loglevels, ", "))).
			Default("WARNING").
			Enum(loglevels...)
	logstashArgs = kingpin.
			Flag("logstash-arg", "Command line arguments, which are passed to Logstash. Flag and value have to be provided as a flag each, e.g.: --logstash-arg=-n --logstash-arg=InstanceName").
			PlaceHolder("ARG").
			Strings()
	logstashOutput = kingpin.
			Flag("logstash-output", "Print the debug output of logstash.").
			Default("false").
			Bool()
	logstashPaths = kingpin.
			Flag("logstash-path", "Add a path to the list of Logstash executable paths that will be tried in order (first match is used).").
			PlaceHolder("PATH").
			Strings()
	logstashVersion = kingpin.
			Flag("logstash-version", "The version of Logstash that's being targeted.").
			PlaceHolder("VERSION").
			Default(autoVersion).
			String()
	unixSockets = kingpin.
			Flag("sockets", "Use Unix domain sockets for the communication with Logstash.").
			Default("false").
			Bool()
	unixSocketCommTimeout = kingpin.
				Flag("sockets-timeout", "Timeout (duration) for the communication with Logstash via Unix domain sockets. Has no effect unless --sockets is used.").
				Default("60s").
				Duration()

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

// findExecutable examines the passed file paths and returns the first
// one that is an existing executable file.
func findExecutable(paths []string) (string, error) {
	for _, p := range paths {
		stat, err := os.Stat(p)
		if err != nil {
			log.Debug("Logstash path candidate rejected: %s", err)
			continue
		}
		if !stat.Mode().IsRegular() || stat.Mode().Perm()&0111 != 0111 {
			log.Debug("Logstash path candidate not an executable regular file: %s", p)
			continue
		}
		log.Debug("Logstash path candidate accepted: %s", p)
		return p, nil
	}
	return "", fmt.Errorf("no existing executable found among candidates: %s", strings.Join(paths, ", "))
}

// runTests runs Logstash with a set of configuration files against a
// slice of test cases and compares the actual events against the
// expected set. Returns an error if at least one test case fails or
// if there's a problem running the tests.
func runTests(inv *logstash.Invocation, tests []testcase.TestCaseSet, diffCommand []string, keptEnvVars []string) error {
	ok := true
	for _, t := range tests {
		fmt.Printf("Running tests in %s...\n", filepath.Base(t.File))
		p, err := logstash.NewProcess(inv, t.Codec, t.InputFields, keptEnvVars)
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
			message := getLogstashOutputMessage(result.Output, result.Log)
			if err != nil {
				return fmt.Errorf("Error running Logstash: %s.%s", err, message)
			}
			userError("%s", message)
		}
		if err = t.Compare(result.Events, false, diffCommand); err != nil {
			userError("Testcase failed, continuing with the rest: %s", err)
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
func runParallelTests(inv *logstash.Invocation, tests []testcase.TestCaseSet, diffCommand []string, keptEnvVars []string) error {
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

	p, err := logstash.NewParallelProcess(inv, testStreams, keptEnvVars)
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
		message := getLogstashOutputMessage(result.Output, result.Log)
		if err != nil {
			return fmt.Errorf("Error running Logstash: %s.%s", err, message)
		}
		userError("%s", message)
	}
	ok := true
	for i, t := range tests {
		if err = t.Compare(result.Events[i], false, diffCommand); err != nil {
			userError("Testcase failed, continuing with the rest: %s", err)
			ok = false
		}
	}

	if !ok {
		return errors.New("one or more testcases failed")
	}
	return nil
}

// getLogstashOutputMessage examines the test result and prepares a
// message describing the process's output, log output, or neither
// (resulting in an empty string).
func getLogstashOutputMessage(output string, log string) string {
	var message string
	if output != "" {
		message += fmt.Sprintf("\nProcess output:\n%s", output)
	} else {
		message += "\nThe process wrote nothing to stdout or stderr."
	}
	if log != "" {
		message += fmt.Sprintf("\nLog:\n%s", log)
	} else {
		message += "\nThe process wrote nothing to its logfile."
	}
	return message
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

// mainEntrypoint functions as the main function of the program and
// returns the desired exit code.
func mainEntrypoint() int {
	kingpin.Version(fmt.Sprintf("%s %s", kingpin.CommandLine.Name, version))
	kingpin.Parse()

	level, err := oplogging.LogLevel(*loglevel)
	if err != nil {
		prefixedUserError("Bad loglevel: %s", loglevel)
		return 1
	}
	logging.SetLevel(level)

	diffCmd, err := shellwords.NewParser().Parse(*diffCommand)
	if err != nil {
		userError("Error parsing diff command %q: %s", *diffCommand, err)
		return 1
	}

	tests, err := testcase.DiscoverTests(*testcasePath)
	if err != nil {
		userError(err.Error())
		return 1
	}

	allKeptEnvVars := append(defaultKeptEnvVars, *keptEnvVars...)

	logstashPath, err := findExecutable(append(*logstashPaths, defaultLogstashPaths...))
	if err != nil {
		userError("Error locating Logstash: %s", err)
		return 1
	}

	var targetVersion *semver.Version
	if *logstashVersion == autoVersion {
		targetVersion, err = logstash.DetectVersion(logstashPath, allKeptEnvVars)
		if err != nil {
			userError("Could not auto-detect the Logstash version: %s", err)
			return 1
		}
	} else {
		targetVersion, err = semver.New(*logstashVersion)
		if err != nil {
			userError("The given Logstash version %q could not be parsed as a version number (%s).", *logstashVersion, err)
			return 1
		}
	}

	inv, err := logstash.NewInvocation(logstashPath, *logstashArgs, targetVersion, *configPaths...)
	if err != nil {
		userError("An error occurred while setting up the Logstash environment: %s", err)
		return 1
	}
	defer inv.Release()
	if *unixSockets {
		if runtime.GOOS == "windows" {
			userError("Use of Unix domain sockets for communication with Logstash is not supported on Windows.")
			return 1
		}
		fmt.Println("Use Unix domain sockets.")
		if err = runParallelTests(inv, tests, diffCmd, allKeptEnvVars); err != nil {
			userError(err.Error())
			return 1
		}
	} else {
		if err = runTests(inv, tests, diffCmd, allKeptEnvVars); err != nil {
			userError(err.Error())
			return 1
		}
	}
	return 0
}

func main() {
	os.Exit(mainEntrypoint())
}
