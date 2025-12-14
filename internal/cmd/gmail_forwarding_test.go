package cmd

import "testing"

func TestForwardingCommandsExist(t *testing.T) {
	// Unit tests for the actual API calls live in integration; here we just ensure
	// the commands exist and are properly structured. (Compile-time coverage.)
	_ = newGmailForwardingCmd
	_ = newGmailForwardingListCmd
	_ = newGmailForwardingGetCmd
	_ = newGmailForwardingCreateCmd
	_ = newGmailForwardingDeleteCmd
}
