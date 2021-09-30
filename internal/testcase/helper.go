package testcase

import (
	"reflect"
	"regexp"
)

// parseAllBracketProperties permit to convert attributes with bracket in sub structure.
func parseAllBracketProperties(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for k, v := range data {
		listKeys := extractBracketFields(k)
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Map {
			// Contain sub map
			parseBracketProperty(listKeys, parseAllBracketProperties(v.(map[string]interface{})), result)
		} else {
			// Contain value
			parseBracketProperty(listKeys, v, result)
		}
	}

	return result
}

// extractBracketFields convert bracket notation to slice of key.
func extractBracketFields(key string) []string {
	rValidator := regexp.MustCompile(`^(\[[^\[\],]+\])+$`)
	rExtractField := regexp.MustCompile(`\[([^\[\],]+)\]`)
	listKeys := make([]string, 0, 1)

	if rValidator.MatchString(key) {
		resultsExtractedKeys := rExtractField.FindAllStringSubmatch(key, -1)
		for _, extractedKeys := range resultsExtractedKeys {
			listKeys = append(listKeys, extractedKeys[1])
		}
	} else {
		listKeys = append(listKeys, key)
	}

	return listKeys
}

// parseBracketProperty handles the recursivity to transform attributes that contain bracket in sub structure.
func parseBracketProperty(listKeys []string, value interface{}, result map[string]interface{}) {
	// Last key
	if len(listKeys) == 1 {
		result[listKeys[0]] = value
		return
	}

	// Check if substruct already exist and call recursivity
	if _, ok := result[listKeys[0]]; !ok {
		result[listKeys[0]] = make(map[string]interface{})
	}
	parseBracketProperty(listKeys[1:], value, result[listKeys[0]].(map[string]interface{}))
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

// removeFields removes a key specified in either bracket notation
// from a (possibly nested) map.
func removeFields(key string, data map[string]interface{}) {
	// Convert bracket notation
	listKeys := extractBracketFields(key)
	removeField(listKeys, data)
}
