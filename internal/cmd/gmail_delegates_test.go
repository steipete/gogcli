package cmd

import "testing"

func TestDelegatesCommandsExist(t *testing.T) {
	// Unit tests for the actual API calls live in integration; here we just ensure
	// the commands exist and are properly structured. (Compile-time coverage.)
	_ = newGmailDelegatesCmd
	_ = newGmailDelegatesListCmd
	_ = newGmailDelegatesGetCmd
	_ = newGmailDelegatesAddCmd
	_ = newGmailDelegatesRemoveCmd
}
