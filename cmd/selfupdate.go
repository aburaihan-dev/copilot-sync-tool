package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/aburaihan-dev/copilot-sync-tool/internal/ui"
	"github.com/spf13/cobra"
)

var selfUpdateCmd = &cobra.Command{
	Use:   "self-update",
	Short: "Update copilot-sync-tool to the latest release",
	Long: `Checks GitHub Releases for a newer version of copilot-sync-tool and
replaces the running binary in-place if one is found.

The update is atomic: the new binary is downloaded to a temporary file and
renamed over the current executable only after a successful download.`,
	Example: `  # Check for and install the latest update
  copilot-sync-tool self-update

  # Check what the latest version is without updating
  copilot-sync-tool self-update --check`,
	RunE: runSelfUpdate,
}

var selfUpdateCheck bool

func init() {
	selfUpdateCmd.Flags().BoolVar(&selfUpdateCheck, "check", false, "Only print the latest version, do not download")
	rootCmd.AddCommand(selfUpdateCmd)
}

const githubReleaseAPI = "https://api.github.com/repos/aburaihan-dev/copilot-dotfiles/releases/latest"

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func runSelfUpdate(_ *cobra.Command, _ []string) error {
	ui.Header("Self-Update")
	fmt.Println()

	ui.Action("Checking latest release...")
	rel, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to fetch release info: %w", err)
	}

	ui.Info(fmt.Sprintf("Latest version: %s", rel.TagName))
	ui.Info(fmt.Sprintf("Current version: %s", Version))

	if rel.TagName == Version {
		ui.Success("Already up to date.")
		return nil
	}

	if selfUpdateCheck {
		ui.Warn(fmt.Sprintf("Update available: %s → %s (run without --check to install)", Version, rel.TagName))
		return nil
	}

	assetName := binaryAssetName(rel.TagName)
	var downloadURL string
	for _, a := range rel.Assets {
		if a.Name == assetName {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no asset named %q found in release %s", assetName, rel.TagName)
	}

	ui.Action(fmt.Sprintf("Downloading %s...", assetName))
	if err := downloadAndReplace(downloadURL); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	ui.Success(fmt.Sprintf("Updated to %s — restart the tool to use the new version.", rel.TagName))
	return nil
}

func fetchLatestRelease() (*ghRelease, error) {
	req, err := http.NewRequest(http.MethodGet, githubReleaseAPI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "copilot-sync-tool/"+Version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %s", resp.Status)
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// binaryAssetName returns the expected release asset filename for the current platform.
func binaryAssetName(version string) string {
	var os_, arch string
	switch runtime.GOOS {
	case "darwin":
		os_ = "macos"
	case "linux":
		os_ = "linux"
	default:
		os_ = "windows"
	}
	switch runtime.GOARCH {
	case "arm64":
		arch = "arm64"
	default:
		arch = "amd64"
	}
	name := fmt.Sprintf("copilot-sync-tool-%s-%s-%s", version, os_, arch)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

func downloadAndReplace(url string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot locate current executable: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("cannot resolve executable path: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "copilot-sync-tool/"+Version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %s", resp.Status)
	}

	tmp := exe + ".update"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("cannot write temp file: %w", err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(tmp)
		return fmt.Errorf("download failed: %w", err)
	}
	out.Close()

	// On Windows rename over a running exe fails; we write a .bat to finish.
	if runtime.GOOS == "windows" {
		return windowsAtomicReplace(tmp, exe)
	}
	return os.Rename(tmp, exe)
}

// windowsAtomicReplace creates a helper batch file to replace the exe after exit.
func windowsAtomicReplace(src, dst string) error {
	bat := dst + ".update.bat"
	script := fmt.Sprintf(`@echo off
:wait
timeout /t 1 /nobreak >nul
move /y "%s" "%s" >nul 2>&1
if errorlevel 1 goto wait
del "%%~f0"
`, src, dst)
	if err := os.WriteFile(bat, []byte(strings.ReplaceAll(script, "\n", "\r\n")), 0o644); err != nil {
		return err
	}
	ui.Warn(fmt.Sprintf("Windows: run the helper script to complete the update:\n  %s", bat))
	return nil
}
