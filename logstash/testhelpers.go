// Copyright (c) 2016 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"reflect"
	"testing"
)

func compareErrors(t *testing.T, idx int, expected, actual error) {
	if actual == nil && expected != nil || actual != nil && expected == nil {
		t.Errorf("Test %d: Expected result:\n%#v\nGot:\n%#v", idx, expected, actual)
	} else if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		t.Errorf("Test %d: Expected result:\n%#v\nGot:\n%#v", idx, expected, actual)
	}
}
