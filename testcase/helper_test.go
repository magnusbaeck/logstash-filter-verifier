package testcase

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

// TestParseDotProperty test keys that contain dot or bracket notation are converted to sub structure
func TestParseDotProperty(t *testing.T) {

	var (
		key      string
		value    string
		result   map[string]interface{}
		expected map[string]interface{}
	)

	// Create sub structure with dot notation
	key = "log.file.path"
	value = "/tmp/test.log"
	result = make(map[string]interface{})
	expected = map[string]interface{}{
		"log": map[string]interface{}{
			"file": map[string]interface{}{
				"path": "/tmp/test.log",
			},
		},
	}
	parseDotProperty(key, value, result)
	assert.Equal(t, expected, result)

	// Create sub structure with bracket notation
	key = "[log][file][path]"
	value = "/tmp/test.log"
	result = make(map[string]interface{})
	expected = map[string]interface{}{
		"log": map[string]interface{}{
			"file": map[string]interface{}{
				"path": "/tmp/test.log",
			},
		},
	}
	parseDotProperty(key, value, result)
	assert.Equal(t, expected, result)

	// Do nothin when no brancket and not dot
	key = "message"
	value = "/tmp/test.log"
	result = make(map[string]interface{})
	expected = map[string]interface{}{
		"message": "/tmp/test.log",
	}
	parseDotProperty(key, value, result)
	assert.Equal(t, expected, result)
}

// TestParseAllDotProperties test that all keys on map that contain dot or bracket notation are converted on sub structure
func TestParseAllDotProperties(t *testing.T) {

	data := map[string]interface{}{
		"message":             "my message",
		"log.file.path":       "/tmp/test.log",
		"[log][origin][line]": 2,
	}
	expected := map[string]interface{}{
		"message": "my message",
		"log": map[string]interface{}{
			"file": map[string]interface{}{
				"path": "/tmp/test.log",
			},
			"origin": map[string]interface{}{
				"line": 2,
			},
		},
	}

	result := parseAllDotProperties(data)
	assert.Equal(t, expected, result)

}

// TestRemoveField test that ignore fields are removed from actual events
// It support dot and bracket notation
func TestRemoveField(t *testing.T) {

	var (
		keys     []string
		data     map[string]interface{}
		expected map[string]interface{}
	)

	// Test without specific notation
	keys = []string{
		"message",
	}
	data = map[string]interface{}{
		"message": "my message",
		"log": map[string]interface{}{
			"file": map[string]interface{}{
				"path": "/tmp/file.log",
			},
		},
		"source": "test",
	}
	expected = map[string]interface{}{
		"log": map[string]interface{}{
			"file": map[string]interface{}{
				"path": "/tmp/file.log",
			},
		},
		"source": "test",
	}
	removeField(keys, data)
	assert.Equal(t, expected, data)

	// Test when keys is sub item and is the last item
	data = map[string]interface{}{
		"message": "my message",
		"log": map[string]interface{}{
			"file": map[string]interface{}{
				"path": "/tmp/file.log",
			},
		},
		"source": "test",
	}
	keys = []string{
		"log",
		"file",
		"path",
	}
	expected = map[string]interface{}{
		"message": "my message",
		"source":  "test",
	}
	removeField(keys, data)
	assert.Equal(t, expected, data)

	// Test when keys is sub item and is stay brother fields
	data = map[string]interface{}{
		"message": "my message",
		"log": map[string]interface{}{
			"file": map[string]interface{}{
				"path": "/tmp/file.log",
				"size": "test",
			},
		},
		"source": "test",
	}
	keys = []string{
		"log",
		"file",
		"path",
	}
	expected = map[string]interface{}{
		"message": "my message",
		"log": map[string]interface{}{
			"file": map[string]interface{}{
				"size": "test",
			},
		},
		"source": "test",
	}
	removeField(keys, data)
	assert.Equal(t, expected, data)

}

// TestRemoveFields test that ignore field are removed from actual events
// It support dot and bracket notation
func TestRemoveFields(t *testing.T) {

	var (
		key      string
		data     map[string]interface{}
		expected map[string]interface{}
	)

	// Test without specific notation
	key = "message"
	data = map[string]interface{}{
		"message": "my message",
		"log": map[string]interface{}{
			"file": map[string]interface{}{
				"path": "/tmp/file.log",
			},
		},
		"source": "test",
	}
	expected = map[string]interface{}{
		"log": map[string]interface{}{
			"file": map[string]interface{}{
				"path": "/tmp/file.log",
			},
		},
		"source": "test",
	}
	removeFields(key, data)
	assert.Equal(t, expected, data)

	// Test when keys with dot notation
	data = map[string]interface{}{
		"message": "my message",
		"log": map[string]interface{}{
			"file": map[string]interface{}{
				"path": "/tmp/file.log",
			},
		},
		"source": "test",
	}
	key = "log.file.path"
	expected = map[string]interface{}{
		"message": "my message",
		"source":  "test",
	}
	removeFields(key, data)
	assert.Equal(t, expected, data)

	// Test when keys with bracket notation
	data = map[string]interface{}{
		"message": "my message",
		"log": map[string]interface{}{
			"file": map[string]interface{}{
				"path": "/tmp/file.log",
			},
		},
		"source": "test",
	}
	key = "[log][file][path]"
	expected = map[string]interface{}{
		"message": "my message",
		"source":  "test",
	}
	removeFields(key, data)
	assert.Equal(t, expected, data)

}
