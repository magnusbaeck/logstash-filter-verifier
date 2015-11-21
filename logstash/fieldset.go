// Copyright (c) 2015 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"fmt"
	"sort"
	"strings"
)

type FieldSet map[string]interface{}

// IsValid inspects the field set and returns an error if there are
// any values that Logstash obviously would disapprove of (like
// objects since only scalar values and arrays are supported).
func (fs FieldSet) IsValid() error {
	_, err := fs.LogstashHash()
	return err
}

// LogstashHash converts a FieldSet into a Logstash-style hash that
// e.g. is accepted by an add_field directive in a configuration file,
// i.e. it has the form { "key1" => "value1" ... "keyN" => "valueN" }.
func (fs FieldSet) LogstashHash() (string, error) {
	result := make([]string, 0, len(fs))
	for k, v := range fs {
		s, err := serializeAsLogstashLiteral(v)
		if err != nil {
			return "", fmt.Errorf("Problem converting field %q to Logstash format: %s", k, err)
		}
		result = append(result, fmt.Sprintf("%q => %s", k, s))
	}
	// Sort the strings to make writing tests easier when there's
	// more than one field in the map.
	sort.Strings(result)
	return "{ " + strings.Join(result, " ") + " }", nil
}

// serializeAsLogstashLiteral serializes a single entity into a
// Logstash value literal.
func serializeAsLogstashLiteral(v interface{}) (string, error) {
	switch v := v.(type) {
	case bool:
		return fmt.Sprintf("%v", v), nil
	case int:
		return fmt.Sprintf("%v", v), nil
	case float64:
		return fmt.Sprintf("%v", v), nil
	case string:
		return fmt.Sprintf("%q", v), nil
	case []interface{}:
		result := make([]string, len(v))
		for i, element := range v {
			s, err := serializeAsLogstashLiteral(element)
			if err != nil {
				return "", err
			}
			result[i] = s
		}
		return "[" + strings.Join(result, ", ") + "]", nil
	default:
		return "", fmt.Errorf("Unsupported type %T: %#v", v, v)
	}
}
