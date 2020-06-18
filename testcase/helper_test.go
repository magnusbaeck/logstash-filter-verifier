package testcase

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractBracketFields(t *testing.T) {
	cases := []struct {
		key      string
		expected []string
	}{
		// Correct nested field notation.
		{
			"[log][file][path]",
			[]string{"log", "file", "path"},
		},
		// Plain non-nested field notation.
		{
			"message",
			[]string{"message"},
		},
		// Bad syntax for nested fields.
		{
			"[log]badformat",
			[]string{"[log]badformat"},
		},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, extractBracketFields(c.key))
	}
}

// TestParseBracketProperty test keys that contain bracket notation are converted to sub structure.
func TestParseBracketProperty(t *testing.T) {
	cases := []struct {
		key      []string
		value    string
		expected map[string]interface{}
	}{
		// Created nested fields when input key is nested.
		{
			key:   []string{"log", "file", "path"},
			value: "/tmp/test.log",
			expected: map[string]interface{}{
				"log": map[string]interface{}{
					"file": map[string]interface{}{
						"path": "/tmp/test.log",
					},
				},
			},
		},
		// Do nothing when the input key isn't nested.
		{
			key:   []string{"message"},
			value: "/tmp/test.log",
			expected: map[string]interface{}{
				"message": "/tmp/test.log",
			},
		},
	}
	for _, c := range cases {
		result := make(map[string]interface{})
		parseBracketProperty(c.key, c.value, result)
		assert.Equal(t, c.expected, result)
	}
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
	cases := []struct {
		keys     []string
		data     map[string]interface{}
		expected map[string]interface{}
	}{
		// Non-nested fields removed.
		{
			keys: []string{"message"},
			data: map[string]interface{}{
				"message": "my message",
				"log": map[string]interface{}{
					"file": map[string]interface{}{
						"path": "/tmp/file.log",
					},
				},
				"source": "test",
			},
			expected: map[string]interface{}{
				"log": map[string]interface{}{
					"file": map[string]interface{}{
						"path": "/tmp/file.log",
					},
				},
				"source": "test",
			},
		},
		// Nested fields removed and the removed field is the last remaining field.
		{
			keys: []string{"log", "file", "path"},
			data: map[string]interface{}{
				"message": "my message",
				"log": map[string]interface{}{
					"file": map[string]interface{}{
						"path": "/tmp/file.log",
					},
				},
				"source": "test",
			},
			expected: map[string]interface{}{
				"message": "my message",
				"source":  "test",
			},
		},
		// Nested fields and the removed field has siblings.
		{
			keys: []string{"log", "file", "path"},
			data: map[string]interface{}{
				"message": "my message",
				"log": map[string]interface{}{
					"file": map[string]interface{}{
						"path": "/tmp/file.log",
						"size": "test",
					},
				},
				"source": "test",
			},
			expected: map[string]interface{}{
				"message": "my message",
				"log": map[string]interface{}{
					"file": map[string]interface{}{
						"size": "test",
					},
				},
				"source": "test",
			},
		},
	}
	for _, c := range cases {
		removeField(c.keys, c.data)
		assert.Equal(t, c.expected, c.data)
	}
}

// TestRemoveFields tests that ignored field are removed from actual events
// It supports bracket notation.
func TestRemoveFields(t *testing.T) {
	cases := []struct {
		key      string
		data     map[string]interface{}
		expected map[string]interface{}
	}{
		// Non-nested fields removed.
		{
			key: "message",
			data: map[string]interface{}{
				"message": "my message",
				"log": map[string]interface{}{
					"file": map[string]interface{}{
						"path": "/tmp/file.log",
					},
				},
				"source": "test",
			},
			expected: map[string]interface{}{
				"log": map[string]interface{}{
					"file": map[string]interface{}{
						"path": "/tmp/file.log",
					},
				},
				"source": "test",
			},
		},
		// Nested fields removed.
		{
			key: "[log][file][path]",
			data: map[string]interface{}{
				"message": "my message",
				"log": map[string]interface{}{
					"file": map[string]interface{}{
						"path": "/tmp/file.log",
					},
				},
				"source": "test",
			},
			expected: map[string]interface{}{
				"message": "my message",
				"source":  "test",
			},
		},
	}
	for _, c := range cases {
		removeFields(c.key, c.data)
		assert.Equal(t, c.expected, c.data)
	}
}
