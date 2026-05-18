package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// run executes a git command in the given directory and returns stdout output.
func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		// Give a clear error when git is not installed.
		var execErr *exec.Error
		if errors.As(err, &execErr) && execErr.Err == exec.ErrNotFound {
			return "", fmt.Errorf("git is not installed or not in PATH\n  → Install git from https://git-scm.com/downloads")
		}
		msg := strings.TrimSpace(errOut.String())
		if msg == "" {
			msg = strings.TrimSpace(out.String())
		}
		return "", fmt.Errorf("git %s: %w (%s)", args[0], err, msg)
	}
	return strings.TrimSpace(out.String()), nil
}

// Status returns the short git status string for the repo at dir.
func Status(dir string) (string, error) {
	return run(dir, "status", "--short")
}

// Branch returns the current branch name.
func Branch(dir string) (string, error) {
	return run(dir, "rev-parse", "--abbrev-ref", "HEAD")
}

// AheadBehind returns how many commits ahead and behind the current branch is vs its upstream.
func AheadBehind(dir string) (int, int, error) {
	out, err := run(dir, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err != nil {
		// No upstream configured is not a fatal error
		return 0, 0, nil
	}
	parts := strings.Fields(out)
	if len(parts) != 2 {
		return 0, 0, nil
	}
	ahead, err1 := strconv.Atoi(parts[0])
	behind, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, nil
	}
	return ahead, behind, nil
}

// AddAll stages all changes in the repo.
func AddAll(dir string) error {
	_, err := run(dir, "add", "-A")
	return err
}

// Commit creates a commit with the given message.
func Commit(dir, message string) error {
	_, err := run(dir, "commit", "-m", message)
	return err
}

// Push pushes the current HEAD to origin/main.
func Push(dir string) error {
	_, err := run(dir, "push", "origin", "HEAD:main")
	return err
}

// Pull pulls from the remote tracking branch.
func Pull(dir string) error {
	_, err := run(dir, "pull")
	return err
}

// StatusPath returns the short git status output scoped to a specific path.
// Useful for detecting uncommitted changes inside a symlinked subdirectory.
func StatusPath(dir, path string) (string, error) {
	return run(dir, "status", "--short", "--", path)
}

// RemoteStatus fetches remote status without network access (uses cached data).
func RemoteStatus(dir string) (string, error) {
	return run(dir, "remote", "-v")
}
