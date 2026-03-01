package main

import (
	"os/exec"
	"strings"
)

func getGitCommit() (string, error) {
	// Simple git rev-parse HEAD
	// In a real CLI, we might use the git library or exec
	// Since we are inside the repo, exec is fine
	cmd := "git"
	args := []string{"rev-parse", "HEAD"}

	out, err := execCommand(cmd, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Wrapper for testing/mocking if needed
var execCommand = func(name string, arg ...string) ([]byte, error) {
	c := exec.Command(name, arg...)
	return c.Output()
}
