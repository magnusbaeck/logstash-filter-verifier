// Copyright (c) 2016 Magnus Bäck <magnus@noun.se>

package testhelpers

import (
	"reflect"
	"testing"
)

// CompareErrors compares two error values and reports an error if
// their types aren't identical (and therefore also if either value is
// nil but the other one isn't).
func CompareErrors(t *testing.T, idx int, expected, actual error) {
	if actual == nil && expected != nil || actual != nil && expected == nil {
		t.Errorf("Test %d: Expected result:\n%#v\nGot:\n%#v", idx, expected, actual)
	} else if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		t.Errorf("Test %d: Expected result:\n%#v\nGot:\n%#v", idx, expected, actual)
	}
}

// AssertEqual compares any two values and reports an error if their
// values are not equal.
func AssertEqual(t *testing.T, expected interface{}, actual interface{}) {
	if expected != actual {
		t.Errorf("%v != %v", expected, actual)
	}
}