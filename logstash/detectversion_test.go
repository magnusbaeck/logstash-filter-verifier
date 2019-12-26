// Copyright (c) 2017 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"fmt"
	"testing"

	semver "github.com/Masterminds/semver/v3"
)

func TestParseLogstashVersionOutput(t *testing.T) {
	cases := []struct {
		input    string
		expected string
		err      error
	}{
		// Current "logstash --version" output.
		{
			"logstash 5.4.1",
			"5.4.1",
			nil,
		},
		// We should not be case sensitive.
		{
			"Logstash 5.4.1",
			"5.4.1",
			nil,
		},
		// We should accept version numbers with a "v" in front.
		{
			"logstash v5.4.1",
			"5.4.1",
			nil,
		},
		// Leading lines should be ignored.
		{
			"leading line\nlogstash 5.4.1",
			"5.4.1",
			nil,
		},
		// First matching version number should be chosen.
		{
			"logstash 5.4.1\nlogstash 1.2.3",
			"5.4.1",
			nil,
		},
		// No version number found.
		{
			"no version number here",
			"",
			fmt.Errorf("unable to find version number in output from Logstash: no version number here"),
		},
	}
	for i, c := range cases {
		var expectedVersion *semver.Version
		var err error
		if len(c.expected) > 0 {
			expectedVersion, err = semver.NewVersion(c.expected)
			if err != nil {
				t.Fatalf("Test %d: Unexpected error when parsing version number: %s", i, err)
			}
		}

		version, err := parseLogstashVersionOutput(c.input)
		if err == nil && c.err != nil {
			t.Errorf("Test %d: Expected failure, got success.", i)
		} else if err != nil && c.err == nil {
			t.Errorf("Test %d: Expected success, got this error instead: %#v", i, err)
		} else if err != nil && c.err != nil && err.Error() != c.err.Error() {
			t.Errorf("Test %d:\nExpected error:\n%s\nGot:\n%s", i, c.err, err)
		} else if version != nil && !version.Equal(expectedVersion) {
			t.Errorf("Test %d:\nExpected:\n%s\nGot:\n%s", i, expectedVersion, version)
		}
	}
}
