// Copyright (c) 2016 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"reflect"
	"testing"
)

func TestGetLimitedEnvironment(t *testing.T) {
	cases := []struct {
		original []string
		kept     []string
		expected []string
	}{
		// Only TZ=UTC set if there are no keepers.
		{
			[]string{
				"A=B",
				"C=D",
			},
			[]string{},
			[]string{
				"TZ=UTC",
			},
		},
		// Original variables can be kept.
		{
			[]string{
				"A=B",
				"C=D",
			},
			[]string{
				"A",
			},
			[]string{
				"A=B",
				"TZ=UTC",
			},
		},
		// Multiple original variables
		{
			[]string{
				"A=B",
				"C=D",
				"E=F",
			},
			[]string{
				"A",
				"E",
			},
			[]string{
				"A=B",
				"E=F",
				"TZ=UTC",
			},
		},
		// TZ can be overridden.
		{
			[]string{
				"TZ=Europe/Stockholm",
			},
			[]string{
				"TZ",
			},
			[]string{
				"TZ=Europe/Stockholm",
			},
		},
		// Listing a keeper that isn't set is okay.
		{
			[]string{
				"A=B",
			},
			[]string{
				"UNDEFINED_KEEPER",
			},
			[]string{
				"TZ=UTC",
			},
		},
	}
	for i, c := range cases {
		actual := GetLimitedEnvironment(c.original, c.kept)
		if !reflect.DeepEqual(c.expected, actual) {
			t.Errorf("Test %d:\nExpected:\n%#v\nGot:\n%#v", i, c.expected, actual)
		}
	}
}
