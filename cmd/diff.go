package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/aburaihan-dev/copilot-sync-tool/internal/platform"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/ui"
	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show line-level diff between local and dotfiles config",
	Long: `Compares each managed config file between the local Copilot directory and the
dotfiles repository, printing a unified diff.

When a file is a symlink pointing to the dotfiles copy, the diff will be empty
(they are the same file). The output is most useful in copy-mode installs.`,
	Example: `  # Diff all managed files
  copilot-sync-tool diff

  # Diff only the MCP config
  copilot-sync-tool diff --mcp

  # Diff only settings and instructions
  copilot-sync-tool diff --settings --instructions`,
	RunE: runDiff,
}

var (
	diffMCP          bool
	diffAgents       bool
	diffSettings     bool
	diffInstructions bool
)

func init() {
	diffCmd.Flags().BoolVar(&diffMCP, "mcp", false, "Diff mcp-config.json")
	diffCmd.Flags().BoolVar(&diffAgents, "agents", false, "Diff agents/ directory")
	diffCmd.Flags().BoolVar(&diffSettings, "settings", false, "Diff settings.json")
	diffCmd.Flags().BoolVar(&diffInstructions, "instructions", false, "Diff copilot-instructions.md")
	rootCmd.AddCommand(diffCmd)
}

func runDiff(_ *cobra.Command, _ []string) error {
	dotfiles, err := GetDotfilesDir()
	if err != nil {
		return err
	}

	ui.Header("Diff: Local vs Dotfiles")
	fmt.Println()

	plat := platform.Current()
	anyFlag := diffMCP || diffAgents || diffSettings || diffInstructions
	showAll := !anyFlag

	var anyOutput bool

	// MCP config
	if showAll || diffMCP {
		local, _ := platform.MCPConfigFile()
		dotf := platform.DotfilesMCPConfig(dotfiles, plat)
		if printed := printFileDiff("mcp-config.json", local, dotf); printed {
			anyOutput = true
		}
	}

	// settings.json
	if showAll || diffSettings {
		local, _ := platform.SettingsFile()
		dotf := platform.DotfilesSettingsFile(dotfiles)
		if printed := printFileDiff("settings.json", local, dotf); printed {
			anyOutput = true
		}
	}

	// copilot-instructions.md
	if showAll || diffInstructions {
		local, _ := platform.InstructionsFile()
		dotf := platform.DotfilesInstructionsFile(dotfiles)
		if printed := printFileDiff("copilot-instructions.md", local, dotf); printed {
			anyOutput = true
		}
	}

	// agents directory
	if showAll || diffAgents {
		localDir, _ := platform.AgentsDir()
		dotfDir := platform.DotfilesAgentsDir(dotfiles)
		if printed := printDirDiff("agents/", localDir, dotfDir); printed {
			anyOutput = true
		}
	}

	if !anyOutput {
		ui.Success("All managed files are in sync.")
	}
	return nil
}

// printFileDiff prints a unified diff for a single file pair. Returns true if any output was printed.
func printFileDiff(label, localPath, dotfilesPath string) bool {
	localData, localErr := os.ReadFile(localPath)
	dotfData, dotfErr := os.ReadFile(dotfilesPath)

	if localErr != nil && dotfErr != nil {
		return false
	}

	var localLines, dotfLines []string
	if localErr == nil {
		localLines = splitLines(string(localData))
	}
	if dotfErr == nil {
		dotfLines = splitLines(string(dotfData))
	}

	if bytes.Equal(localData, dotfData) {
		return false
	}

	ui.SectionHeader(fmt.Sprintf("diff: %s", label))
	printUnifiedDiff(localLines, dotfLines, localPath, dotfilesPath)
	fmt.Println()
	return true
}

// printDirDiff diffs all *.agent.md / *.json files in two directories.
func printDirDiff(label, localDir, dotfilesDir string) bool {
	localFiles := readDirFiles(localDir)
	dotfFiles := readDirFiles(dotfilesDir)

	names := unionKeys(localFiles, dotfFiles)
	var printed bool

	ui.SectionHeader(fmt.Sprintf("diff: %s", label))
	for _, name := range names {
		localData := localFiles[name]
		dotfData := dotfFiles[name]
		if string(localData) == string(dotfData) {
			continue
		}
		printed = true
		localLines := splitLines(string(localData))
		dotfLines := splitLines(string(dotfData))
		fmt.Printf("--- local/%s\n+++ dotfiles/%s\n", name, name)
		printUnifiedDiff(localLines, dotfLines, "", "")
	}
	if !printed {
		return false
	}
	fmt.Println()
	return true
}

func readDirFiles(dir string) map[string][]byte {
	result := map[string][]byte{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return result
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(dir + "/" + e.Name())
		if err == nil {
			result[e.Name()] = data
		}
	}
	return result
}

func unionKeys(a, b map[string][]byte) []string {
	seen := map[string]bool{}
	for k := range a {
		seen[k] = true
	}
	for k := range b {
		seen[k] = true
	}
	var keys []string
	for k := range seen {
		keys = append(keys, k)
	}
	// Sort for deterministic output
	for i := 0; i < len(keys)-1; i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

func splitLines(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}

// printUnifiedDiff produces a simple unified diff output (no external dependency).
func printUnifiedDiff(aLines, bLines []string, aPath, bPath string) {
	if aPath != "" {
		fmt.Printf("--- %s\n+++ %s\n", aPath, bPath)
	}
	lcs := lcsLines(aLines, bLines)
	ai, bi, li := 0, 0, 0
	for li <= len(lcs) {
		var aEnd, bEnd int
		if li < len(lcs) {
			aEnd = lcs[li][0]
			bEnd = lcs[li][1]
		} else {
			aEnd = len(aLines)
			bEnd = len(bLines)
		}
		for ai < aEnd {
			fmt.Printf("-%s\n", aLines[ai])
			ai++
		}
		for bi < bEnd {
			fmt.Printf("+%s\n", bLines[bi])
			bi++
		}
		if li < len(lcs) {
			fmt.Printf(" %s\n", aLines[lcs[li][0]])
			ai = lcs[li][0] + 1
			bi = lcs[li][1] + 1
		}
		li++
	}
}

// lcsLines returns the LCS as pairs of [aIndex, bIndex] for common lines.
func lcsLines(a, b []string) [][2]int {
	m, n := len(a), len(b)
	// For very large files limit to avoid O(m*n) blowup
	if m > 500 || n > 500 {
		return nil
	}
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] > dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var pairs [][2]int
	i, j := 0, 0
	for i < m && j < n {
		if a[i] == b[j] {
			pairs = append(pairs, [2]int{i, j})
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			i++
		} else {
			j++
		}
	}
	return pairs
}
