// Copyright (c) 2016 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// BadLogstashOutputError indicates that Logstash emitted a line that
// couldn't be interpreted as an event, typically because it wasn't
// valid JSON.
type BadLogstashOutputError struct {
	output string
}

// Error returns a string representation of the error.
func (e BadLogstashOutputError) Error() string {
	return fmt.Sprintf("Logstash emitted this line which can't be parsed as JSON: %s", e.output)
}

// readEvents reads zero, one, or more Logstash events from a stream
// of newline-separated JSON strings.
func readEvents(r io.Reader) ([]Event, error) {
	events := []Event{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		var event Event
		err := json.Unmarshal([]byte(scanner.Text()), &event)
		if err != nil {
			return events, BadLogstashOutputError{scanner.Text()}
		}
		events = append(events, event)
	}
	return events, scanner.Err()
}

// formatAndPRintEvents reads zero, one, or more Logstash events from a stream
// of newline-separated JSON strings and pretty-prints them to stdout.
func formatAndPrintEvents(r io.Reader) ([]Event, error) {
	events := []Event{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		var event Event
		err := json.Unmarshal([]byte(scanner.Text()), &event)
		if err != nil {
			return events, BadLogstashOutputError{scanner.Text()}
		}
		buf, err := json.MarshalIndent(event, "", "  ")
		if err != nil {
			return events, fmt.Errorf("Failed to marshal %+v as JSON: %s", event, err)
		}
		fmt.Printf(string(buf) + "\n")
	}
	return events, scanner.Err()
}
