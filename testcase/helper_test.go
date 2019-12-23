package testcase

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractBracketFields(t *testing.T) {
	var (
		key      string
		result   []string
		expected []string
	)

	// When bracket
	key = "[log][file][path]"
	expected = []string{
		"log",
		"file",
		"path",

	}
	result = extractBracketFields(key)
	assert.Equal(t, expected, result)

	// When no bracket
	key = "message"
	expected = []string{
		"message",
	}
	result = extractBracketFields(key)
	assert.Equal(t, expected, result)

	// When bad bracket
	key = "[log]badformat"
	expected = []string{
		"[log]badformat",
	}
	result = extractBracketFields(key)
	assert.Equal(t, expected, result)

}

// TestParseBracketProperty test keys that contain bracket notation are converted to sub structure
func TestParseBracketProperty(t *testing.T) {

	var (
		key      []string
		value    string
		result   map[string]interface{}
		expected map[string]interface{}
	)

	// Create sub structure with bracket notation
	key = []string{
		"log",
		"file",
		"path",
	}
	value = "/tmp/test.log"
	result = make(map[string]interface{})
	expected = map[string]interface{}{
		"log": map[string]interface{}{
			"file": map[string]interface{}{
				"path": "/tmp/test.log",
			},
		},
	}
	parseBracketProperty(key, value, result)
	assert.Equal(t, expected, result)

	// Do nothing when no bracket and not dot
	key = []string{"message"}
	value = "/tmp/test.log"
	result = make(map[string]interface{})
	expected = map[string]interface{}{
		"message": "/tmp/test.log",
	}
	parseBracketProperty(key, value, result)
	assert.Equal(t, expected, result)
}

// TestParseAllDotProperties tests that all keys on map that contain dot
// or bracket notation are converted on sub structure.
func TestParseAllDotProperties(t *testing.T) {
	data := map[string]interface{}{
		"message":             "my message",
		"[log][file][path]":   "/tmp/test.log",
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

	result := parseAllBracketProperties(data)
	assert.Equal(t, expected, result)
}

// TestRemoveField tests that ignored fields are removed from actual events.
// It supports bracket notation.
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

	// Test when keys is sub item with sibling fields
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

// TestRemoveFields tests that ignored field are removed from actual events
// It supports bracket notation.
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

	// Test when keys uses bracket notation
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
