package database

import (
	"bytes"
	"strings"
	"testing"
)

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: "NULL",
		},
		{
			name:     "integer",
			input:    42,
			expected: "42",
		},
		{
			name:     "string",
			input:    "hello",
			expected: "'hello'",
		},
		{
			name:     "string with quote",
			input:    "it's cool",
			expected: "'it''s cool'",
		},
		{
			name:     "boolean true",
			input:    true,
			expected: "true",
		},
		{
			name:     "boolean false",
			input:    false,
			expected: "false",
		},
		{
			name:     "float",
			input:    3.14,
			expected: "3.14",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatValue(tt.input)
			if result != tt.expected {
				t.Errorf("formatValue(%v) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEscapeSQLString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no special chars",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "single quote",
			input:    "it's cool",
			expected: "it''s cool",
		},
		{
			name:     "backslash",
			input:    "path\\to\\file",
			expected: "path\\\\to\\\\file",
		},
		{
			name:     "both quote and backslash",
			input:    "it's a\\path",
			expected: "it''s a\\\\path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeSQLString(tt.input)
			if result != tt.expected {
				t.Errorf("escapeSQLString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateSQL(t *testing.T) {
	records := []Record{
		{
			Table:   "users",
			Columns: []string{"id", "email", "active"},
			Values:  []interface{}{1, "test@example.com", true},
		},
		{
			Table:   "posts",
			Columns: []string{"id", "user_id", "title"},
			Values:  []interface{}{100, 1, "Test Post"},
		},
	}

	var buf bytes.Buffer
	err := GenerateSQL(records, &buf)
	if err != nil {
		t.Fatalf("GenerateSQL failed: %v", err)
	}

	output := buf.String()

	// Check for expected content
	expectedStrings := []string{
		"BEGIN;",
		"COMMIT;",
		"INSERT INTO users",
		"INSERT INTO posts",
		"ON CONFLICT DO NOTHING",
		"test@example.com",
		"Test Post",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Generated SQL missing expected string: %q\nGot:\n%s", expected, output)
		}
	}

	t.Logf("Generated SQL:\n%s", output)
}
