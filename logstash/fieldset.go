// Copyright (c) 2015-2018 Magnus Bäck <magnus@noun.se>

package logstash

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// FieldSet contains a set of fields for a Logstash event and can be
// marshalled as a Logstash-compatible string that's acceptable to an
// add_field option for an input or filter.
type FieldSet map[string]interface{}

// IsValid inspects the field set and returns an error if there are
// any values that Logstash obviously would disapprove of (like
// objects in arrays).
func (fs FieldSet) IsValid() error {
	if fs == nil {
		return errors.New("Fields must not be \"null\".")
	}
	_, err := fs.LogstashHash()
	return err
}

// LogstashHash converts a FieldSet into a Logstash-style hash that
// e.g. is accepted by an add_field directive in a configuration file,
// i.e. it has the form { "key1" => "value1" ... "keyN" => "valueN" }.
func (fs FieldSet) LogstashHash() (string, error) {
	result := make([]string, 0, len(fs))
	for k, v := range fs {
		ks, s, err := serializeAsLogstashLiteral(k, v)
		if err != nil || len(ks) != len(s) {
			return "", fmt.Errorf("Problem converting field %q to Logstash format: %s", k, err)
		}
		for i, k := range ks {
			if strings.LastIndex(k, "[") == 0 {
				k = strings.Trim(k, "[]")
			}
			result = append(result, fmt.Sprintf("%q => %s", k, s[i]))
		}
	}
	// Sort the strings to make writing tests easier when there's
	// more than one field in the map.
	sort.Strings(result)
	return "{ " + strings.Join(result, " ") + " }", nil
}

// serializeAsLogstashLiteral serializes a single entity into a
// Logstash value literal.
func serializeAsLogstashLiteral(k string, v interface{}) ([]string, []string, error) {
	k = fmt.Sprintf("[%s]", k)
	switch v := v.(type) {
	case bool:
		return []string{k}, []string{fmt.Sprintf("%v", v)}, nil
	case float64:
		// large floats must not be converted to exponential notation, because this is not valid for Logstash
		// https://github.com/elastic/logstash/blob/master/logstash-core/lib/logstash/config/grammar.treetop#L92
		if !strings.Contains(fmt.Sprintf("%v", v), "e") {
			return []string{k}, []string{fmt.Sprintf("%v", v)}, nil
		}
		return []string{k}, []string{fmt.Sprintf("%f", v)}, nil
	case string:
		return []string{k}, []string{fmt.Sprintf("%q", v)}, nil
	case []interface{}:
		result := make([]string, len(v))
		for i, element := range v {
			if v, ok := element.(map[string]interface{}); ok {
				return []string{k}, []string{}, fmt.Errorf("Unsupported type %T in %T: %#v", v, []interface{}{}, v)
			}
			ks, s, err := serializeAsLogstashLiteral(k, element)
			if err != nil {
				return ks, []string{}, err
			}
			result[i] = s[0]
		}
		return []string{k}, []string{"[" + strings.Join(result, ", ") + "]"}, nil
	case map[string]interface{}:
		result := make([]string, 0, len(v))
		keys := make([]string, 0, len(v))
		i := 0
		for ik, iv := range v {
			ks, s, err := serializeAsLogstashLiteral(ik, iv)
			if err != nil {
				return nil, nil, err
			}
			for i := range ks {
				ks[i] = fmt.Sprintf("%s%s", k, ks[i])
			}
			result = append(result, s...)
			keys = append(keys, ks...)
			i++
		}
		return keys, result, nil
	default:
		return []string{k}, []string{}, fmt.Errorf("Unsupported type %T: %#v", v, v)
	}
}
