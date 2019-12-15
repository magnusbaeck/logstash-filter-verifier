package testcase

import (
	"reflect"
	"regexp"
	"strings"
)

// parseAllDotProperties permit to convert attributes with dot in sub structure.
func parseAllDotProperties(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range data {
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Map {
			parseDotProperty(k, parseAllDotProperties(v.(map[string]interface{})), result)
		} else {
			parseDotProperty(k, v, result)
		}
	}

	return result
}

// parseDotProperty handles the recursivity to transform attributes that contain dot in sub structure.
func parseDotProperty(key string, value interface{}, result map[string]interface{}) {
	// Convert bracket notation to dot notation
	if strings.HasPrefix(key, "[") {
		r := regexp.MustCompile(`\[(\w+)\]`)
		key = strings.TrimSuffix(r.ReplaceAllString(key, `$1.`), ".")
	}

	if strings.Contains(key, ".") {
		listKey := strings.Split(key, ".")
		if _, ok := result[listKey[0]]; !ok {
			result[listKey[0]] = make(map[string]interface{})
		}
		parseDotProperty(strings.Join(listKey[1:], "."), value, result[listKey[0]].(map[string]interface{}))
	} else {
		result[key] = value
	}
}

// removeField handle the suppression of needed key before compare result and expected data.
func removeField(keys []string, data map[string]interface{}) {
	// Last item
	if len(keys) == 1 {
		delete(data, keys[0])
		return
	}

	// Keys not exist
	val, ok := data[keys[0]]
	if !ok {
		return
	}

	// Check if value is map
	rv := reflect.ValueOf(val)
	if rv.Kind() != reflect.Map {
		// Not map, is the end
		return
	}

	// Recurse if it's a map
	removeField(keys[1:], val.(map[string]interface{}))
	if len(val.(map[string]interface{})) > 0 {
		data[keys[0]] = val
	} else {
		// Empty struct, we remove parents
		delete(data, keys[0])
	}

}

// removeFields removes a key specified in either dot or bracket notation
// from a (possibly nested) map.
func removeFields(key string, data map[string]interface{}) {
	// Convert bracket notation to dot notation
	if strings.HasPrefix(key, "[") {
		r := regexp.MustCompile(`\[(\w+)\]`)
		key = strings.TrimSuffix(r.ReplaceAllString(key, `$1.`), ".")
	}
	keys := strings.Split(key, ".")
	removeField(keys, data)
}
