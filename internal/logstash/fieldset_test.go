// Copyright (c) 2015-2018 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsValid(t *testing.T) {
	cases := []struct {
		input FieldSet
		err   error
	}{
		// Empty field set is okay
		{
			FieldSet{},
			nil,
		},
		// Nil field set is rejected
		{
			nil,
			errors.New("Fields must not be \"null\"."),
		},
		// Value of object type is rejected
		{
			FieldSet{
				"a": []interface{}{
					map[string]interface{}{},
				},
			},
			fmt.Errorf("Problem converting field \"a\" to Logstash format: Unsupported type %T in %T: %#v",
				map[string]interface{}{}, []interface{}{}, map[string]interface{}{}),
		},
	}
	for i, c := range cases {
		err := c.input.IsValid()
		if err == nil && c.err != nil {
			t.Errorf("Test %d: Expected failure, got success.", i)
		} else if err != nil && c.err == nil {
			t.Errorf("Test %d: Expected success, got this error instead: %#v", i, err)
		} else if err != nil && c.err != nil && err.Error() != c.err.Error() {
			t.Errorf("Test %d:\nExpected:\n%s\nGot:\n%s", i, c.err, err)
		}
	}
}

func TestLogstashHash(t *testing.T) {
	cases := []struct {
		input    FieldSet
		expected string
		err      error
	}{
		// Empty field set is okay
		{
			FieldSet{},
			`{  }`,
			nil,
		},
		// Single bool value is okay
		{
			FieldSet{
				"a": true,
			},
			`{ "a" => true }`,
			nil,
		},
		// Single float value is okay
		{
			FieldSet{
				"a": 1.23,
			},
			`{ "a" => 1.23 }`,
			nil,
		},
		// Large floats must not be converted to exponential notation, because this is not valid for Logstash
		// https://github.com/elastic/logstash/blob/master/logstash-core/lib/logstash/config/grammar.treetop#L92
		{
			FieldSet{
				"a": 1234567890.123,
			},
			`{ "a" => 1234567890.123000 }`,
			nil,
		},
		// Integers should work as well.
		{
			FieldSet{
				"a": 1234,
			},
			`{ "a" => 1234 }`,
			nil,
		},
		// Single string value is okay
		{
			FieldSet{
				"a": "b",
			},
			`{ "a" => "b" }`,
			nil,
		},
		// Array field is okay
		{
			FieldSet{
				"a": []interface{}{"b", "c", "d"},
			},
			`{ "a" => ["b", "c", "d"] }`,
			nil,
		},
		// Nested array field is okay
		{
			FieldSet{
				"a": []interface{}{"b", []interface{}{"c", "d"}},
			},
			`{ "a" => ["b", ["c", "d"]] }`,
			nil,
		},
		// Multiple fields of mixed types is okay
		{
			FieldSet{
				"a": "b",
				"c": 123.0,
				"d": true,
				"e": []interface{}{"f", 123.0, true},
			},
			`{ "a" => "b" "c" => 123 "d" => true "e" => ["f", 123, true] }`,
			nil,
		},
		// Value of object with multiple values including nested object
		{
			FieldSet{
				"z": map[string]interface{}{
					"a": "b",
					"c": 123.0,
					"d": true,
					"e": []interface{}{"f", 123.0, true},
					"g": map[string]interface{}{
						"a": "b",
						"c": 123.0,
						"d": true,
						"e": []interface{}{"f", 123.0, true},
					},
				},
			},
			`{ "[z][a]" => "b" "[z][c]" => 123 "[z][d]" => true "[z][e]" => ["f", 123, true] "[z][g][a]" => "b" "[z][g][c]" => 123 "[z][g][d]" => true "[z][g][e]" => ["f", 123, true] }`,
			nil,
		},
		// Value of object type in array is rejected
		{
			FieldSet{
				"a": []interface{}{
					map[string]interface{}{},
				},
			},
			``,
			fmt.Errorf("Problem converting field \"a\" to Logstash format: Unsupported type %T in %T: %#v",
				map[string]interface{}{}, []interface{}{}, map[string]interface{}{}),
		},
	}
	for i, c := range cases {
		actual, err := c.input.LogstashHash()
		if err == nil && c.err != nil {
			t.Errorf("Test %d: Expected failure, got success.", i)
		} else if err != nil && c.err == nil {
			t.Errorf("Test %d: Expected success, got this error instead: %#v", i, err)
		} else if err != nil && c.err != nil && err.Error() != c.err.Error() {
			t.Errorf("Test %d: Didn't get the expected error.\nExpected:\n%s\nGot:\n%s", i, c.err, err)
		} else if c.expected != actual {
			t.Errorf("Test %d:\nExpected:\n%s\nGot:\n%s", i, c.expected, actual)
		}
	}
}
