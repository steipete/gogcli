package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
)

func TestMainHelpDoesNotExit(t *testing.T) {
	t.Helper()

	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"gog", "--help"}
	main()
}

func TestMainExitOnError(t *testing.T) {
	if os.Getenv("GOGCLI_TEST_CHILD") == "1" {
		os.Args = []string{"gog", "nope-nope-nope"}
		main()
		return
	}

	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run", "^TestMainExitOnError$")
	cmd.Env = append(os.Environ(), "GOGCLI_TEST_CHILD=1")
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected exit error")
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if ee.ExitCode() != 2 {
			t.Fatalf("exit=%d", ee.ExitCode())
		}
		return
	}
	t.Fatalf("unexpected err: %v", err)
}
