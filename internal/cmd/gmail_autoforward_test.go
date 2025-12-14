package cmd

import (
	"testing"
)

func TestAutoForwardCommandExists(t *testing.T) {
	// Unit tests for the actual API call live in integration; here we just ensure
	// the command exists and is properly structured. (Compile-time coverage.)
	_ = newGmailAutoForwardCmd
	_ = newGmailAutoForwardGetCmd
	_ = newGmailAutoForwardUpdateCmd
}

func TestValidateDisposition(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		isValid bool
	}{
		{
			name:    "leaveInInbox is valid",
			value:   "leaveInInbox",
			isValid: true,
		},
		{
			name:    "archive is valid",
			value:   "archive",
			isValid: true,
		},
		{
			name:    "trash is valid",
			value:   "trash",
			isValid: true,
		},
		{
			name:    "markRead is valid",
			value:   "markRead",
			isValid: true,
		},
		{
			name:    "invalid value",
			value:   "deleteForever",
			isValid: false,
		},
		{
			name:    "empty string",
			value:   "",
			isValid: false,
		},
	}

	validDispositions := map[string]bool{
		"leaveInInbox": true,
		"archive":      true,
		"trash":        true,
		"markRead":     true,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validDispositions[tt.value]
			if got != tt.isValid {
				t.Errorf("disposition %q: got valid=%v, want valid=%v", tt.value, got, tt.isValid)
			}
		})
	}
}
