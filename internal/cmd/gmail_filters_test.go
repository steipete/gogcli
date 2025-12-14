package cmd

import "testing"

func TestFiltersCommandsExist(t *testing.T) {
	// Unit tests for the actual API calls live in integration; here we just ensure
	// the commands exist and are properly structured. (Compile-time coverage.)
	_ = newGmailFiltersCmd
	_ = newGmailFiltersListCmd
	_ = newGmailFiltersGetCmd
	_ = newGmailFiltersCreateCmd
	_ = newGmailFiltersDeleteCmd
}
