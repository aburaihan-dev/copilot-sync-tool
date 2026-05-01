package symlink

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Check returns true if path is a symlink (regardless of target validity).
// Returns (false, nil) if path is not a symlink.
// Returns (false, err) if there was an error other than "not a symlink" or "not found".
func Check(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	return info.Mode()&os.ModeSymlink != 0, nil
}

// CanCreate returns true if symlink creation is supported in the given directory.
// On Windows this requires Developer Mode or Administrator privileges.
func CanCreate(dir string) bool {
	tmp := filepath.Join(dir, ".symlink-test-probe")
	target := filepath.Join(dir, ".symlink-test-target")

	// Create a throwaway target file so the symlink has something to point at.
	if err := os.WriteFile(target, []byte(""), 0600); err != nil {
		return false
	}
	defer os.Remove(target)
	defer os.Remove(tmp)

	if err := os.Symlink(target, tmp); err != nil {
		return false
	}
	return true
}

// Create creates a symlink at dst pointing to src.
// On Windows, if symlink creation fails, it returns a descriptive error.
func Create(src, dst string) error {
	if err := os.Symlink(src, dst); err != nil {
		if runtime.GOOS == "windows" {
			return fmt.Errorf(
				"symlink creation failed on Windows: %w\n"+
					"  → Enable Developer Mode (Settings → System → For Developers) or run as Administrator.\n"+
					"  → Alternatively, use 'install --copy' to copy files instead of symlinking.",
				err,
			)
		}
		return fmt.Errorf("creating symlink %s → %s: %w", dst, src, err)
	}
	return nil
}

// IsPointingTo returns true if path is a symlink that points to target.
func IsPointingTo(path, target string) bool {
	resolved, err := os.Readlink(path)
	if err != nil {
		return false
	}
	return resolved == target
}
