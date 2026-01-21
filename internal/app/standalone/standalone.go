package standalone

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/imkira/go-observer"
	"github.com/mattn/go-shellwords"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logstash"
	lfvobserver "github.com/magnusbaeck/logstash-filter-verifier/v2/internal/observer"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/testcase"
)

const autoVersion = "auto"

var (
	defaultKeptEnvVars = []string{
		"PATH",
	}
	defaultLogstashPaths = []string{
		"/opt/logstash/bin/logstash",
		"/usr/share/logstash/bin/logstash",
	}
)

type Standalone struct {
	quiet                 bool
	diffCommand           string
	testcasePath          string
	keptEnvVars           []string
	logstashPaths         []string
	logstashVersion       string
	logstashArgs          []string
	logstashOutput        bool
	configPaths           []string
	unixSockets           bool
	unixSocketCommTimeout time.Duration
	logfiles              []string
	logfileExtension      string

	log logging.Logger
}

func New(
	quiet bool,
	diffCommand string,
	testcasePath string,
	keptEnvVars []string,
	logstashPaths []string,
	logstashVersion string,
	logstashArgs []string,
	logstashOutput bool,
	configPaths []string,
	unixSockets bool,
	unixSocketCommTimeout time.Duration,
	logfiles []string,
	logfileExtension string,
	log logging.Logger,
) Standalone {
	return Standalone{
		quiet:                 quiet,
		diffCommand:           diffCommand,
		testcasePath:          testcasePath,
		keptEnvVars:           keptEnvVars,
		logstashPaths:         logstashPaths,
		logstashVersion:       logstashVersion,
		logstashArgs:          logstashArgs,
		logstashOutput:        logstashOutput,
		configPaths:           configPaths,
		unixSockets:           unixSockets,
		unixSocketCommTimeout: unixSocketCommTimeout,
		logfiles:              logfiles,
		logfileExtension:      logfileExtension,
		log:                   log,
	}
}

func (s Standalone) Run() error {
	var status bool

	// Set up observers
	observers := make([]lfvobserver.Interface, 0)
	liveObserver := observer.NewProperty(lfvobserver.TestExecutionStart{})
	if !s.quiet {
		observers = append(observers, lfvobserver.NewSummaryObserver(liveObserver))
	}
	for _, obs := range observers {
		if err := obs.Start(); err != nil {
			return fmt.Errorf("Initialization error: %s", err)
		}
	}

	diffCmd, err := shellwords.NewParser().Parse(s.diffCommand)
	if err != nil {
		return fmt.Errorf("Error parsing diff command %q: %s", s.diffCommand, err)
	}

	tests, err := testcase.DiscoverTests(s.testcasePath)
	if err != nil {
		return fmt.Errorf(err.Error())
	}

	allKeptEnvVars := append(defaultKeptEnvVars, s.keptEnvVars...)

	logstashPath, err := s.findExecutable(append(s.logstashPaths, defaultLogstashPaths...))
	if err != nil {
		return fmt.Errorf("Error locating Logstash: %s", err)
	}

	var targetVersion *semver.Version
	if s.logstashVersion == autoVersion {
		targetVersion, err = logstash.DetectVersion(logstashPath, allKeptEnvVars)
		if err != nil {
			return fmt.Errorf("Could not auto-detect the Logstash version: %s", err)
		}
	} else {
		targetVersion, err = semver.NewVersion(s.logstashVersion)
		if err != nil {
			return fmt.Errorf("The given Logstash version %q could not be parsed as a version number (%s).", s.logstashVersion, err)
		}
	}

	inv, err := logstash.NewInvocation(logstashPath, s.logstashArgs, targetVersion, s.configPaths...)
	if err != nil {
		return fmt.Errorf("An error occurred while setting up the Logstash environment: %s", err)
	}
	defer inv.Release()

	if len(s.logfiles) > 0 {
		if status, err = s.runLogs(inv, tests, s.logfiles, s.logfileExtension, allKeptEnvVars); err != nil {
			return fmt.Errorf(err.Error())
		}
	} else {
		if s.unixSockets {
			if runtime.GOOS == "windows" {
				return fmt.Errorf("Use of Unix domain sockets for communication with Logstash is not supported on Windows.")
			}
			fmt.Println("Use Unix domain sockets.")
			if status, err = s.runParallelTests(inv, tests, diffCmd, allKeptEnvVars, liveObserver); err != nil {
				return fmt.Errorf(err.Error())
			}
		} else {
			if status, err = s.runTests(inv, tests, diffCmd, allKeptEnvVars, liveObserver); err != nil {
				return fmt.Errorf(err.Error())
			}
		}
	}

	liveObserver.Update(lfvobserver.TestExecutionEnd{})

	for _, obs := range observers {
		if err := obs.Finalize(); err != nil {
			return fmt.Errorf(err.Error())
		}
	}

	if status {
		return nil
	}
	return errors.New("failed test cases")
}

// findExecutable examines the passed file paths and returns the first
// one that is an existing executable file.
func (s Standalone) findExecutable(paths []string) (string, error) {
	for _, p := range paths {
		stat, err := os.Stat(p)
		if err != nil {
			s.log.Debugf("Logstash path candidate rejected: %s", err)
			continue
		}
		if !stat.Mode().IsRegular() {
			s.log.Debugf("Logstash path candidate not a regular file: %s", p)
			continue
		}
		if runtime.GOOS != "windows" && stat.Mode().Perm()&0111 != 0111 {
			s.log.Debugf("Logstash path candidate not an executable file: %s", p)
			continue
		}
		s.log.Debugf("Logstash path candidate accepted: %s", p)
		return p, nil
	}
	return "", fmt.Errorf("no existing executable found among candidates: %s", strings.Join(paths, ", "))
}

// runTests runs Logstash with a set of configuration files against a
// slice of test cases and compares the actual events against the
// expected set. Returns a bool that indicates whether all tests pass
// and an error that indicates a problem running the tests.
func (s Standalone) runTests(inv *logstash.Invocation, tests []testcase.TestCaseSet, diffCommand []string, keptEnvVars []string, liveObserver observer.Property) (bool, error) {
	ok := true
	for _, t := range tests {
		fmt.Printf("Running tests in %s...\n", filepath.Base(t.File))
		p, err := logstash.NewProcess(inv, t.Codec, t.InputFields, keptEnvVars)
		if err != nil {
			return false, err
		}
		defer p.Release()
		if err = p.Start(); err != nil {
			return false, err
		}

		for _, line := range t.InputLines {
			_, err = p.Input.Write([]byte(line + "\n"))
			if err != nil {
				return false, err
			}
		}
		if err = p.Input.Close(); err != nil {
			return false, err
		}

		result, err := p.WaitAndRead()
		if err != nil || s.logstashOutput {
			message := getLogstashOutputMessage(result.Output, result.Log)
			if err != nil {
				return false, fmt.Errorf("Error running Logstash: %s.%s", err, message)
			}
			userError("%s", message)
		}

		currentOk, err := t.Compare(result.Events, diffCommand, liveObserver)
		if err != nil {
			return false, err
		}
		if !currentOk {
			ok = false
		}
	}

	return ok, nil
}

func (s Standalone) visit(files *[]string, extension string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			s.log.Fatal(err)
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != "."+extension {
			return nil
		}
		*files = append(*files, path)
		return nil
	}
}

// runLogs runs Logstash with a set of configuration files against log files.
func (s Standalone) runLogs(inv *logstash.Invocation, tests []testcase.TestCaseSet, logFilePaths []string, logFileExtension string, keptEnvVars []string) (bool, error) {
	ok := true
	for _, t := range tests {
		fmt.Printf("Using test setup from %s...\n", filepath.Base(t.File))
		p, err := logstash.NewProcess(inv, t.Codec, t.InputFields, keptEnvVars)
		if err != nil {
			return false, err
		}
		defer p.Release()
		if err = p.Start(); err != nil {
			return false, err
		}
		var file *os.File
		var logFiles []string
		var written int64
		for _, logFilePath := range logFilePaths {
			err = filepath.Walk(logFilePath, s.visit(&logFiles, logFileExtension))
			if err != nil {
				return false, err
			}
			for _, logFile := range logFiles {
				fmt.Printf("Piping %s: ", logFile)
				file, err = os.Open(logFile)
				if err != nil {
					return false, err
				}
				written, err = io.Copy(p.Input, file)
				fmt.Printf("%d bytes\n", written)
				if err != nil {
					return false, err
				}
			}
		}
		if err = p.Input.Close(); err != nil {
			return false, err
		}

		result, err := p.WaitAndPrint()
		if err != nil || s.logstashOutput {
			message := getLogstashOutputMessage(result.Output, result.Log)
			if err != nil {
				return false, fmt.Errorf("Error running Logstash: %s.%s", err, message)
			}
			userError("%s", message)
		}
	}
	return ok, nil
}

// runParallelTests runs multiple set of configuration in a single
// instance of Logstash against a slice of test cases and compares
// the actual events against the expected set. Returns a bool that
// indicates whether all tests pass and an error that indicates a
// problem running the tests.
func (s Standalone) runParallelTests(inv *logstash.Invocation, tests []testcase.TestCaseSet, diffCommand []string, keptEnvVars []string, liveProducer observer.Property) (bool, error) {
	testStreams := make([]*logstash.TestStream, 0, len(tests))

	badCodecs := map[string]string{
		"json":  "json_lines",
		"plain": "line",
	}
	for _, t := range tests {
		if repl, ok := badCodecs[t.Codec]; ok {
			s.log.Warning(
				"The testcase file %q uses the %q codec. That codec "+
					"will most likely not work as expected when --sockets is used. Try %q instead.",
				t.File, t.Codec, repl)
		}
	}

	for _, t := range tests {
		ts, err := logstash.NewTestStream(t.Codec, t.InputFields, s.unixSocketCommTimeout)
		if err != nil {
			logstash.CleanupTestStreams(testStreams)
			return false, err
		}
		testStreams = append(testStreams, ts)
	}

	p, err := logstash.NewParallelProcess(inv, testStreams, keptEnvVars)
	if err != nil {
		return false, err
	}
	defer p.Release()
	if err = p.Start(); err != nil {
		return false, err
	}

	for i, t := range tests {
		for _, line := range t.InputLines {
			_, err = testStreams[i].Write([]byte(line + "\n"))
			if err != nil {
				return false, err
			}
		}

		if err = testStreams[i].Close(); err != nil {
			return false, err
		}
	}

	result, err := p.WaitAndRead()
	if err != nil || s.logstashOutput {
		message := getLogstashOutputMessage(result.Output, result.Log)
		if err != nil {
			return false, fmt.Errorf("Error running Logstash: %s.%s", err, message)
		}
		userError("%s", message)
	}
	ok := true
	for i, t := range tests {
		currentOk, err := t.Compare(result.Events[i], diffCommand, liveProducer)
		if err != nil {
			userError("Testcase %s failed, continuing with the rest: %s", filepath.Base(t.File), err)
		}
		if !currentOk {
			ok = false
		}
	}

	return ok, nil
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

// userError prints an error message to stderr.
func userError(format string, a ...interface{}) {
	if strings.HasSuffix(format, "\n") {
		fmt.Fprintf(os.Stderr, format, a...)
	} else {
		fmt.Fprintf(os.Stderr, format+"\n", a...)
	}
}
