package agents

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// List returns sorted names of all *.agent.md files in dir.
func List(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".agent.md") {
			result = append(result, e.Name())
		}
	}
	sort.Strings(result)
	return result, nil
}

// Untracked returns agent files present in localDir but not in dotfilesDir.
func Untracked(localDir, dotfilesDir string) ([]string, error) {
	local, err := List(localDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	dotfiles, err := List(dotfilesDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	dotfileSet := map[string]bool{}
	for _, f := range dotfiles {
		dotfileSet[f] = true
	}

	var untracked []string
	for _, f := range local {
		if !dotfileSet[f] {
			untracked = append(untracked, f)
		}
	}
	return untracked, nil
}

// CaptureNew copies agent files from localDir to dotfilesDir that don't exist there yet.
// Returns the names of newly copied agents.
func CaptureNew(localDir, dotfilesDir string) ([]string, error) {
	untracked, err := Untracked(localDir, dotfilesDir)
	if err != nil {
		return nil, err
	}

	if len(untracked) == 0 {
		return nil, nil
	}

	var captured []string
	for _, name := range untracked {
		src := filepath.Join(localDir, name)
		dst := filepath.Join(dotfilesDir, name)
		if err := copyFile(src, dst); err != nil {
			return captured, fmt.Errorf("copying %s: %w", name, err)
		}
		captured = append(captured, name)
	}
	return captured, nil
}

// copyFile copies a single file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	info, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	return nil
}
