package testcase

import (
	"reflect"
	"regexp"
	"strings"

	oplogging "github.com/op/go-logging"
)

// parseAllDotProperties permit to convert attributes with dot in sub structure
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

// parseDotPropertie handle the recursivity to transform attribute that contain dot in sub structure
func parseDotProperty(key string, value interface{}, result map[string]interface{}) {

	// Convert bracket notation to dot notation
	if strings.HasPrefix(key, "[") {
		r := regexp.MustCompile(`\[(\w+)\]`)

		key = strings.TrimSuffix(r.ReplaceAllString(key, `$1.`), ".")

		if log.IsEnabledFor(oplogging.DEBUG) {
			log.Debugf("Key: %s", key)
		}
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

// removeField handle the supression of needed key before compare result and expected data
func removeField(keys []string, data map[string]interface{}) {

	// last item
	if len(keys) == 1 {
		delete(data, keys[0])
		return
	}

	//Keys not exist
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

	// Is the map, need recurse
	removeField(keys[1:], val.(map[string]interface{}))
	if len(val.(map[string]interface{})) > 0 {
		data[keys[0]] = val
	} else {
		// Empty struct, we remove parents
		delete(data, keys[0])
	}

}

// removeFields permit to remove key on map
// Key can be with dot or bracket notation. In this case, the key is trasformed on sub structure
func removeFields(key string, data map[string]interface{}) {
	// Convert bracket notation to dot notation
	if strings.HasPrefix(key, "[") {
		r := regexp.MustCompile(`\[(\w+)\]`)

		key = strings.TrimSuffix(r.ReplaceAllString(key, `$1.`), ".")

		if log.IsEnabledFor(oplogging.DEBUG) {
			log.Debugf("Key: %s", key)
		}
	}

	keys := strings.Split(key, ".")

	removeField(keys, data)
}
