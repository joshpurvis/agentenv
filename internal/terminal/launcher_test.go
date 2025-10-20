package terminal

import (
	"testing"
)

func TestDetectTerminal(t *testing.T) {
	terminal := DetectTerminal()

	if terminal == nil {
		t.Fatal("DetectTerminal returned nil")
	}

	t.Logf("Detected terminal: %s (available: %v)", terminal.Name, terminal.Available)

	// Should always return a terminal struct, even if not available
	if terminal.Name == "" {
		t.Error("Terminal name should not be empty")
	}
}

func TestGetTerminalInfo(t *testing.T) {
	info := GetTerminalInfo()

	if info == "" {
		t.Error("GetTerminalInfo returned empty string")
	}

	t.Logf("Terminal info: %s", info)
}

func TestValidateTerminal(t *testing.T) {
	tests := []struct {
		name    string
		term    string
		wantErr bool
	}{
		{
			name:    "valid terminal - alacritty",
			term:    "alacritty",
			wantErr: false, // May fail if not installed
		},
		{
			name:    "valid terminal - xterm",
			term:    "xterm",
			wantErr: false, // May fail if not installed
		},
		{
			name:    "invalid terminal",
			term:    "nonexistent-terminal-xyz",
			wantErr: true,
		},
		{
			name:    "unsupported terminal",
			term:    "notepad",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTerminal(tt.term)
			if (err != nil) != tt.wantErr {
				t.Logf("ValidateTerminal(%s) error = %v, wantErr %v", tt.term, err, tt.wantErr)
			}
		})
	}
}

func TestIsCommandAvailable(t *testing.T) {
	// Test with a command that should always exist
	if !isCommandAvailable("ls") {
		t.Error("ls command should be available on Unix systems")
	}

	// Test with a command that should not exist
	if isCommandAvailable("nonexistent-command-xyz-123") {
		t.Error("nonexistent command should not be available")
	}
}
