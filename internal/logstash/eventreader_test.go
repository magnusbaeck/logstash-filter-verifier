// Copyright (c) 2016 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/testhelpers"
)

func TestReadEvents(t *testing.T) {
	cases := []struct {
		input  string
		events []Event
		err    error
	}{
		// Empty input results in an empty slice.
		{
			input:  "",
			events: []Event{},
			err:    nil,
		},
		// Single event is okay.
		{
			input: "{\"a\": 1}",
			events: []Event{
				{
					"a": 1,
				},
			},
			err: nil,
		},
		// Multiple events are okay.
		{
			input: "{\"a\": 1}\n{\"b\": 2}",
			events: []Event{
				{
					"a": 1,
				},
				{
					"b": 2,
				},
			},
			err: nil,
		},
		// Broken JSON output is correctly reported.
		{
			input:  "this is not JSON",
			events: []Event{},
			err:    BadLogstashOutputError{},
		},
		// Already collected events are returned when broken
		// JSON is encountered.
		{
			input: "{\"a\": 1}\nthis is not JSON",
			events: []Event{
				{
					"a": 1,
				},
			},
			err: BadLogstashOutputError{},
		},
	}
	for i, c := range cases {
		events, err := readEvents(bytes.NewBufferString(c.input))

		expectedEvents := fmt.Sprintf("%#v", c.events)
		actualEvents := fmt.Sprintf("%#v", events)
		if expectedEvents != actualEvents {
			t.Errorf("Test %d:\nExpected events:\n%q\nGot:\n%q", i, expectedEvents, actualEvents)
		}

		testhelpers.CompareErrors(t, i, c.err, err)
	}
}
