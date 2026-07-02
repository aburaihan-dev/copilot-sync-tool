// Package skills manages Copilot CLI skill directories (settings.json "skillDirectories"),
// mirroring them into/from the dotfiles repo the same way internal/agents handles agent files —
// except each skill is a directory tree (SKILL.md + assets/references), not a single file.
package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// SkillDiff holds the result of comparing local skill directories against dotfiles.
type SkillDiff struct {
	Added    []string // skill names present locally, missing from dotfiles
	Modified []string // present in both but content differs
	Removed  []string // present in dotfiles, no longer present locally
}

// resolve maps skill name -> source directory, for every existing directory in localPaths.
func resolve(localPaths []string) map[string]string {
	m := map[string]string{}
	for _, p := range localPaths {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			m[filepath.Base(p)] = p
		}
	}
	return m
}

// List returns sorted skill names (subdirectories) already tracked in dir.
func List(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// dirHash hashes relative paths + contents of every file under dir, for change detection.
func dirHash(dir string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		fmt.Fprintf(h, "%s\n", filepath.ToSlash(rel))
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		h.Write(data)
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Diff computes the three-way difference between the skill directories referenced by
// localPaths and the skill directories already tracked in dotfilesDir.
func Diff(localPaths []string, dotfilesDir string) (SkillDiff, error) {
	local := resolve(localPaths)
	dotf, err := List(dotfilesDir)
	if err != nil {
		return SkillDiff{}, err
	}
	dotfSet := make(map[string]bool, len(dotf))
	for _, n := range dotf {
		dotfSet[n] = true
	}

	names := make([]string, 0, len(local))
	for n := range local {
		names = append(names, n)
	}
	sort.Strings(names)

	var d SkillDiff
	for _, name := range names {
		if !dotfSet[name] {
			d.Added = append(d.Added, name)
			continue
		}
		sh, e1 := dirHash(local[name])
		dh, e2 := dirHash(filepath.Join(dotfilesDir, name))
		if e1 == nil && e2 == nil && sh != dh {
			d.Modified = append(d.Modified, name)
		}
	}
	for _, name := range dotf {
		if _, ok := local[name]; !ok {
			d.Removed = append(d.Removed, name)
		}
	}
	return d, nil
}

// ForceMirror makes dotfilesDir an exact mirror of the skill directories referenced by
// localPaths: new/modified skills are copied in, skills no longer present locally are removed.
func ForceMirror(localPaths []string, dotfilesDir string) (SkillDiff, error) {
	if err := os.MkdirAll(dotfilesDir, 0755); err != nil {
		return SkillDiff{}, fmt.Errorf("creating dotfiles skills dir: %w", err)
	}
	d, err := Diff(localPaths, dotfilesDir)
	if err != nil {
		return SkillDiff{}, err
	}
	local := resolve(localPaths)
	for _, name := range append(append([]string{}, d.Added...), d.Modified...) {
		dst := filepath.Join(dotfilesDir, name)
		if err := os.RemoveAll(dst); err != nil {
			return d, fmt.Errorf("clearing %s: %w", name, err)
		}
		if err := CopyDir(local[name], dst); err != nil {
			return d, fmt.Errorf("copying %s: %w", name, err)
		}
	}
	for _, name := range d.Removed {
		if err := os.RemoveAll(filepath.Join(dotfilesDir, name)); err != nil {
			return d, fmt.Errorf("removing %s: %w", name, err)
		}
	}
	return d, nil
}

// CaptureNew copies skill directories from localPaths into dotfilesDir that don't exist there yet.
// Returns the names of newly copied skills.
func CaptureNew(localPaths []string, dotfilesDir string) ([]string, error) {
	if err := os.MkdirAll(dotfilesDir, 0755); err != nil {
		return nil, fmt.Errorf("creating dotfiles skills dir: %w", err)
	}
	d, err := Diff(localPaths, dotfilesDir)
	if err != nil {
		return nil, err
	}
	if len(d.Added) == 0 {
		return nil, nil
	}
	local := resolve(localPaths)
	var captured []string
	for _, name := range d.Added {
		if err := CopyDir(local[name], filepath.Join(dotfilesDir, name)); err != nil {
			return captured, fmt.Errorf("copying %s: %w", name, err)
		}
		captured = append(captured, name)
	}
	return captured, nil
}

// CopyDir recursively copies the contents of src into dst, creating dst if needed.
func CopyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
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

	_, err = io.Copy(dstFile, srcFile)
	return err
}
