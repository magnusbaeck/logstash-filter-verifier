// Copyright (c) 2016-2018 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"strings"
)

// GetLimitedEnvironment returns a list of "key=value" strings
// representing a process's environment based on an original set of
// variables (e.g. returned by os.Environ()) that's intersected with a
// list of the names of variables that should be kept.
//
// Additionally, the TZ variable is set to "UTC" unless TZ is one of
// the variables to keep. The point of this is to make the tests more
// stable and independent of the current timezone so there's no risk
// of a @timestamp mismatch just because we've gone into daylight
// savings time.
func GetLimitedEnvironment(originalVars, keptVars []string) []string {
	keepVar := func(varname string) bool {
		for _, s := range keptVars {
			if varname == s {
				return true
			}
		}
		return false
	}

	// It would've been easier to just check with os.Getenv()
	// whether a particular variable is set rather than iterating
	// over the whole environment list that we're given, but
	// os.Getenv() doesn't distinguish between unset variables and
	// variables set to an empty string.
	result := []string{}
	for _, keyval := range originalVars {
		tokens := strings.SplitN(keyval, "=", 2)
		if keepVar(tokens[0]) {
			result = append(result, keyval)
		}
	}
	if !keepVar("TZ") {
		result = append(result, "TZ=UTC")
	}
	return result
}
