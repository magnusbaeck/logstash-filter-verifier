package testcase

import (
	"reflect"
	"strings"
)

// parseAllDotProperties permit to convert attributes with dot in sub structure
func parseAllDotProperties(data map[string]interface{}) map[string]interface{} {

	result := make(map[string]interface{})
	for k, v := range data {
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Map {
			parseDotPropertie(k, parseAllDotProperties(v.(map[string]interface{})), result)
		} else {
			parseDotPropertie(k, v, result)
		}
	}

	return result
}

// parseDotPropertie handle the recursivity to transform attribute that contain dot in sub structure
func parseDotPropertie(key string, value interface{}, result map[string]interface{}) {
	if strings.Contains(key, ".") {
		listKey := strings.Split(key, ".")
		if _, ok := result[listKey[0]]; !ok {
			result[listKey[0]] = make(map[string]interface{})
		}
		parseDotPropertie(strings.Join(listKey[1:], "."), value, result[listKey[0]].(map[string]interface{}))
	} else {
		result[key] = value
	}
}

func removeFields(keys []string, data map[string]interface{}) map[string]interface{} {

	return removeField(keys, data)

}

func removeField(keys []string, data map[string]interface{}) map[string]interface{} {

	// last item
	if len(keys) == 1 {
		delete(data, keys[0])
		return data
	}

	//else recurse
	val, ok := data[keys[0]]
	if !ok {
		return data
	}

	// Value is map
	rv := reflect.ValueOf(val)
	if rv.Kind() != reflect.Map {
		return data
	}

	data[keys[0]] = removeField(keys[1:], val.(map[string]interface{}))
	return data
}
