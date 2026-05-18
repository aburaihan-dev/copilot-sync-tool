package cmd

import (
"fmt"

"github.com/AlecAivazis/survey/v2"
"github.com/aburaihan-dev/copilot-sync-tool/internal/agents"
"github.com/aburaihan-dev/copilot-sync-tool/internal/git"
"github.com/aburaihan-dev/copilot-sync-tool/internal/platform"
"github.com/aburaihan-dev/copilot-sync-tool/internal/symlink"
"github.com/aburaihan-dev/copilot-sync-tool/internal/ui"
"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
Use:   "sync",
Short: "Interactive sync — choose to pull dotfiles or push device state",
Long: `Shows the current sync state then presents a direction menu:

  Pull  — git pull + install (update this device from the dotfiles repo)
  Push  — capture device state → dotfiles repo (commit + push)
  Diff  — show line-level differences between local and dotfiles
  Cancel

The Push direction handles the case where agents were removed or modified
on this device and the dotfiles repo should match:

  • Symlinked setup  — agents/ IS the dotfiles dir; uncommitted git changes
                       (deletions, modifications) are staged and committed.
  • Copy-mode setup  — a full mirror sync is performed first (ForceMirror),
                       then changes are committed and pushed.

Non-interactive flags (skip the menu):
  --pull     git pull + install all (old default behaviour)
  --no-pull  install from current dotfiles without pulling`,
Example: `  # Interactive menu (default)
  copilot-sync-tool sync

  # Non-interactive: pull latest dotfiles and install
  copilot-sync-tool sync --pull

  # Non-interactive: re-install from current dotfiles without pulling
  copilot-sync-tool sync --no-pull`,
RunE: runSync,
}

var (
syncNoPull bool
syncPull   bool
)

func init() {
syncCmd.Flags().BoolVar(&syncNoPull, "no-pull", false, "Non-interactive: install from current dotfiles (no git pull)")
syncCmd.Flags().BoolVar(&syncPull, "pull", false, "Non-interactive: git pull + install all")
rootCmd.AddCommand(syncCmd)
}

// ── menu choices ──────────────────────────────────────────────────────────────

const (
syncOptPull   = "Pull dotfiles → this device   (git pull + install)"
syncOptPush   = "Push this device → dotfiles   (commit device state to repo)"
syncOptDiff   = "Show diff"
syncOptCancel = "Cancel"
)

func runSync(cmd *cobra.Command, args []string) error {
dotfiles, err := GetDotfilesDir()
if err != nil {
return err
}

// ── non-interactive fast paths ────────────────────────────────────────────
if syncPull {
ui.Header("Sync — Pull")
fmt.Println()
return runSyncPull(cmd, args, dotfiles)
}
if syncNoPull {
ui.Header("Sync — Install (no pull)")
fmt.Println()
ui.SectionHeader("Installing all components")
installAll = true
installPull = false
installMCP = false
installAgents = false
installSettings = false
installInstructions = false
installInteractive = false
installCopy = false
return runInstall(cmd, args)
}

// ── interactive menu ──────────────────────────────────────────────────────
ui.Header("Sync")
fmt.Println()

if err := printSyncStatus(dotfiles); err != nil {
ui.Warn(fmt.Sprintf("Could not determine full status: %v", err))
}
fmt.Println()

var choice string
if err := survey.AskOne(&survey.Select{
Message: "What would you like to do?",
Options: []string{syncOptPull, syncOptPush, syncOptDiff, syncOptCancel},
Default: syncOptPull,
}, &choice); err != nil {
return err
}
fmt.Println()

switch choice {
case syncOptPull:
return runSyncPull(cmd, args, dotfiles)
case syncOptPush:
return runSyncPush(dotfiles)
case syncOptDiff:
return runDiff(cmd, args)
default:
ui.Info("Cancelled.")
return nil
}
}

// printSyncStatus shows a compact state summary above the direction menu.
func printSyncStatus(dotfiles string) error {
ui.SectionHeader("Current State")

agentsLocal, _ := platform.AgentsDir()
agentsDotfiles := platform.DotfilesAgentsDir(dotfiles)
agentsLink, _ := symlink.Check(agentsLocal)

var agentNote string
if agentsLink {
gitOut, _ := git.StatusPath(dotfiles, "copilot/agents/")
if gitOut != "" {
agentNote = "symlinked — uncommitted changes pending"
} else {
agentNote = "symlinked — clean"
}
} else {
diff, _ := agents.Diff(agentsLocal, agentsDotfiles)
total := len(diff.Added) + len(diff.Modified) + len(diff.Removed)
if total == 0 {
agentNote = "copy mode — in sync"
} else {
agentNote = fmt.Sprintf("copy mode — +%d ~%d -%d vs dotfiles",
len(diff.Added), len(diff.Modified), len(diff.Removed))
}
}
ui.Info(fmt.Sprintf("%-20s %s", "agents/", agentNote))

type fileEntry struct {
label  string
pathFn func() (string, error)
}
for _, fe := range []fileEntry{
{"settings.json", platform.SettingsFile},
{"mcp-config.json", platform.MCPConfigFile},
{"instructions.md", platform.InstructionsFile},
} {
localPath, _ := fe.pathFn()
isLink, _ := symlink.Check(localPath)
if isLink {
ui.Info(fmt.Sprintf("%-20s symlinked", fe.label))
} else {
ui.Info(fmt.Sprintf("%-20s copy mode", fe.label))
}
}

branch, _ := git.Branch(dotfiles)
ahead, behind, _ := git.AheadBehind(dotfiles)
gitNote := fmt.Sprintf("branch: %s", branch)
if ahead > 0 || behind > 0 {
gitNote += fmt.Sprintf("  (%d ahead, %d behind remote)", ahead, behind)
}
fmt.Println()
ui.Info(fmt.Sprintf("%-20s %s", "git", gitNote))

return nil
}

// runSyncPull does git pull + install --all.
func runSyncPull(cmd *cobra.Command, args []string, dotfiles string) error {
ui.SectionHeader("Pulling dotfiles")
if err := git.Pull(dotfiles); err != nil {
return fmt.Errorf("git pull failed: %w", err)
}
ui.Success("Pulled latest changes.")
fmt.Println()

ui.SectionHeader("Installing all components")
installAll = true
installPull = false
installMCP = false
installAgents = false
installSettings = false
installInstructions = false
installInteractive = false
installCopy = false
return runInstall(cmd, args)
}

// runSyncPush pushes this device's state into the dotfiles repo and commits.
func runSyncPush(dotfiles string) error {
ui.SectionHeader("Push: device state → dotfiles")
fmt.Println()

plat := platform.Current()
anyChanges := false

// ── Agents ───────────────────────────────────────────────────────────────
ui.SectionHeader("Agents")

agentsLocal, _ := platform.AgentsDir()
agentsDotfiles := platform.DotfilesAgentsDir(dotfiles)
agentsLink, _ := symlink.Check(agentsLocal)

if agentsLink {
gitOut, _ := git.StatusPath(dotfiles, "copilot/agents/")
if gitOut == "" {
ui.Skip("agents/ — symlinked, no uncommitted changes")
} else {
ui.Action("agents/ — symlinked, changes pending in dotfiles repo:")
fmt.Println(gitOut)
anyChanges = true
}
} else {
diff, err := agents.Diff(agentsLocal, agentsDotfiles)
if err != nil {
return fmt.Errorf("computing agent diff: %w", err)
}
total := len(diff.Added) + len(diff.Modified) + len(diff.Removed)
if total == 0 {
ui.Skip("agents/ — copy mode, already in sync with dotfiles")
} else {
for _, a := range diff.Added {
ui.Item(fmt.Sprintf("+ %-50s  (new on device)", a))
}
for _, a := range diff.Modified {
ui.Item(fmt.Sprintf("~ %-50s  (modified on device)", a))
}
for _, a := range diff.Removed {
ui.Item(fmt.Sprintf("- %-50s  (removed from device)", a))
}
fmt.Println()

var apply bool
if err := survey.AskOne(&survey.Confirm{
Message: fmt.Sprintf("Apply %d agent change(s) to dotfiles?", total),
Default: true,
}, &apply); err != nil {
return err
}
if apply {
applied, err := agents.ForceMirror(agentsLocal, agentsDotfiles)
if err != nil {
return fmt.Errorf("mirroring agents: %w", err)
}
for _, a := range applied.Added {
ui.Success(fmt.Sprintf("Added:   %s", a))
}
for _, a := range applied.Modified {
ui.Success(fmt.Sprintf("Updated: %s", a))
}
for _, a := range applied.Removed {
ui.Success(fmt.Sprintf("Removed: %s", a))
}
anyChanges = true
}
}
}

// ── Other files (copy-mode only) ──────────────────────────────────────────
fmt.Println()
ui.SectionHeader("Other Files")

type fileEntry struct {
label      string
localFn    func() (string, error)
dotfilesFn func(string) string
}
for _, fe := range []fileEntry{
{"settings.json", platform.SettingsFile, platform.DotfilesSettingsFile},
{"copilot-instructions.md", platform.InstructionsFile, platform.DotfilesInstructionsFile},
} {
localPath, _ := fe.localFn()
isLink, _ := symlink.Check(localPath)
if isLink {
ui.Skip(fmt.Sprintf("%s — symlinked", fe.label))
continue
}
dotfPath := fe.dotfilesFn(dotfiles)
changed, err := captureFileIfChanged(localPath, dotfPath)
if err != nil {
ui.Warn(fmt.Sprintf("Could not sync %s: %v", fe.label, err))
} else if changed {
ui.Success(fmt.Sprintf("Copied %s to dotfiles", fe.label))
anyChanges = true
} else {
ui.Skip(fmt.Sprintf("%s — copy mode, in sync", fe.label))
}
}

// MCP config: copy (not merge) for push — device is authoritative
mcpLocal, _ := platform.MCPConfigFile()
mcpLink, _ := symlink.Check(mcpLocal)
if mcpLink {
ui.Skip("mcp-config.json — symlinked")
} else {
mcpDotfiles := platform.DotfilesMCPConfig(dotfiles, plat)
changed, err := captureFileIfChanged(mcpLocal, mcpDotfiles)
if err != nil {
ui.Warn(fmt.Sprintf("Could not sync mcp-config.json: %v", err))
} else if changed {
ui.Success("Copied mcp-config.json to dotfiles")
anyChanges = true
} else {
ui.Skip("mcp-config.json — copy mode, in sync")
}
}

if !anyChanges {
fmt.Println()
ui.Success("Nothing to push — dotfiles already match this device.")
return nil
}

// Secrets scan on MCP files
if err := runSecretsCheck(dotfiles, plat, []string{"mcp-config." + plat + ".json"}); err != nil {
return err
}

// ── Confirm git commit + push ─────────────────────────────────────────────
fmt.Println()
var confirm bool
if err := survey.AskOne(&survey.Confirm{
Message: "Commit and push to remote?",
Default: true,
}, &confirm); err != nil {
return err
}
if !confirm {
ui.Warn("Aborted. Local changes were applied but not committed.")
return nil
}

fmt.Println()
ui.SectionHeader("Git")
msg := fmt.Sprintf("sync: push device state from %s", platform.Hostname())
if err := git.AddAll(dotfiles); err != nil {
return fmt.Errorf("git add: %w", err)
}
if err := git.Commit(dotfiles, msg); err != nil {
ui.Warn(fmt.Sprintf("Git commit: %v (nothing to commit?)", err))
} else {
ui.Success(fmt.Sprintf("Committed: %s", msg))
if err := git.Push(dotfiles); err != nil {
ui.Warn(fmt.Sprintf("Git push failed: %v", err))
} else {
ui.Success("Pushed to remote")
}
}
return nil
}
