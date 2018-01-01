// Copyright (c) 2017 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/blang/semver"
)

var (
	logstashVersionRegexp = regexp.MustCompile(`^(?i)Logstash v?(\d+\.\S+)`)
)

// DetectVersion runs "logstash --version" and tries to parse the
// result in order to determine which version of Logstash is being
// run.
func DetectVersion(logstashPath string, keptEnvVars []string) (*semver.Version, error) {
	c := exec.Command(logstashPath, "--version")
	c.Env = getLimitedEnvironment(os.Environ(), keptEnvVars)
	output, err := c.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error running \"%s --version\": %s, process output: %q", logstashPath, err, output)
	}
	return parseLogstashVersionOutput(string(output))
}

func parseLogstashVersionOutput(processOutput string) (*semver.Version, error) {
	for _, line := range strings.Split(processOutput, "\n") {
		m := logstashVersionRegexp.FindStringSubmatch(line)
		if len(m) > 1 {
			v, err := semver.New(m[1])
			if err == nil {
				return v, nil
			}
			log.Warning("Found potential version number %q in line %q in the Logstash version "+
				"output, but the string couldn't be parsed as version number (%s).",
				m[1], line, err)
		}
	}
	return nil, fmt.Errorf("unable to find version number in output from Logstash: %s", processOutput)
}
