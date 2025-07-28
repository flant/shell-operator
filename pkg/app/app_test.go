package app

import (
	"os"
	"testing"
)

func TestIsEventDebouncerEnabled(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{
			name:     "enabled with true",
			envValue: "true",
			expected: true,
		},
		{
			name:     "disabled with false",
			envValue: "false",
			expected: false,
		},
		{
			name:     "disabled with empty",
			envValue: "",
			expected: false,
		},
		{
			name:     "disabled with random value",
			envValue: "random",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envValue != "" {
				os.Setenv("SHELL_OPERATOR_ENABLE_EVENT_DEBOUNCER", tt.envValue)
				defer os.Unsetenv("SHELL_OPERATOR_ENABLE_EVENT_DEBOUNCER")
			}

			// Reset the flag value
			EnableEventDebouncer = tt.envValue

			result := IsEventDebouncerEnabled()
			if result != tt.expected {
				t.Errorf("IsEventDebouncerEnabled() = %v, want %v", result, tt.expected)
			}
		})
	}
}
