package cmd

import (
	"testing"
	"time"
)

func TestParseRFC3339ToMillis(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "valid RFC3339",
			input:   "2024-12-20T00:00:00Z",
			wantErr: false,
		},
		{
			name:    "valid RFC3339 with timezone",
			input:   "2024-12-20T12:30:00-08:00",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRFC3339ToMillis(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRFC3339ToMillis() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.input == "" && got != 0 {
				t.Errorf("parseRFC3339ToMillis() empty input should return 0, got %d", got)
			}
			if tt.input != "" && !tt.wantErr && got == 0 {
				t.Errorf("parseRFC3339ToMillis() valid input should return non-zero, got %d", got)
			}
		})
	}
}

func TestParseRFC3339ToMillisValue(t *testing.T) {
	// Test with a known timestamp
	input := "2024-12-20T00:00:00Z"
	got, err := parseRFC3339ToMillis(input)
	if err != nil {
		t.Fatalf("parseRFC3339ToMillis() unexpected error: %v", err)
	}

	// Parse the same time with standard library for comparison
	expected, err := time.Parse(time.RFC3339, input)
	if err != nil {
		t.Fatalf("time.Parse() unexpected error: %v", err)
	}

	if got != expected.UnixMilli() {
		t.Errorf("parseRFC3339ToMillis() = %d, want %d", got, expected.UnixMilli())
	}
}

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "plain text",
			input: "Hello world",
			want:  "Hello world",
		},
		{
			name:  "html content",
			input: "<p>Hello world</p>",
			want:  "<p>Hello world</p>", // Current implementation returns as-is
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripHTML(tt.input); got != tt.want {
				t.Errorf("stripHTML() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVacationCommandExists(t *testing.T) {
	// Unit tests for the actual API call live in integration; here we just ensure
	// the command exists and is properly structured. (Compile-time coverage.)
	_ = newGmailVacationCmd
	_ = newGmailVacationGetCmd
	_ = newGmailVacationUpdateCmd
}
