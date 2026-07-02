package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestForceMirror(t *testing.T) {
	tmp := t.TempDir()
	localParent := filepath.Join(tmp, "local")
	dotfiles := filepath.Join(tmp, "dotfiles", "skills")

	// Two local skills: "keep" (will change) and "gone" (will be deleted before mirroring).
	mustWrite(t, filepath.Join(localParent, "keep", "SKILL.md"), "v1")
	mustWrite(t, filepath.Join(localParent, "gone", "SKILL.md"), "v1")
	local := []string{filepath.Join(localParent, "keep"), filepath.Join(localParent, "gone")}

	if _, err := ForceMirror(local, dotfiles); err != nil {
		t.Fatalf("initial mirror: %v", err)
	}
	if names, _ := List(dotfiles); len(names) != 2 {
		t.Fatalf("expected 2 skills mirrored, got %v", names)
	}

	// Simulate: "keep" modified locally, "gone" removed locally, "new" added locally.
	mustWrite(t, filepath.Join(localParent, "keep", "SKILL.md"), "v2")
	os.RemoveAll(filepath.Join(localParent, "gone"))
	mustWrite(t, filepath.Join(localParent, "new", "SKILL.md"), "v1")
	local = []string{filepath.Join(localParent, "keep"), filepath.Join(localParent, "new")}

	diff, err := ForceMirror(local, dotfiles)
	if err != nil {
		t.Fatalf("second mirror: %v", err)
	}
	if len(diff.Modified) != 1 || diff.Modified[0] != "keep" {
		t.Errorf("expected 'keep' modified, got %v", diff.Modified)
	}
	if len(diff.Added) != 1 || diff.Added[0] != "new" {
		t.Errorf("expected 'new' added, got %v", diff.Added)
	}
	if len(diff.Removed) != 1 || diff.Removed[0] != "gone" {
		t.Errorf("expected 'gone' removed, got %v", diff.Removed)
	}
	if _, err := os.Stat(filepath.Join(dotfiles, "gone")); !os.IsNotExist(err) {
		t.Errorf("'gone' should have been deleted from dotfiles")
	}
	got, err := os.ReadFile(filepath.Join(dotfiles, "keep", "SKILL.md"))
	if err != nil || string(got) != "v2" {
		t.Errorf("expected 'keep/SKILL.md' == v2, got %q err=%v", got, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
