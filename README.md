# copilot-sync-tool

[![Release](https://img.shields.io/github/v/release/aburaihan-dev/copilot-dotfiles)](https://github.com/aburaihan-dev/copilot-dotfiles/releases/latest)
[![Go](https://img.shields.io/badge/Go-1.22+-blue.svg)](https://golang.org)

A CLI tool for managing GitHub Copilot configuration — agents, MCP servers, settings, and instructions — as dotfiles. Enables cross-device sync across macOS, Linux, and Windows.

---

## Features

- **`init`** — Interactive wizard to scaffold a brand-new dotfiles repo, capture existing config, and push to a remote
- **`setup`** — Configure the tool on an existing machine: set dotfiles path (or clone a repo), install binary to PATH, and apply config
- **`status`** — Rich table showing symlink/copy sync state, agent counts, MCP server counts, git branch/ahead-behind
- **`install`** — Interactive picker to choose which components and individual items (MCP servers, agents) to install; use `--all` to skip prompts
- **`capture`** — Copy local Copilot config into the dotfiles repo; skips already-symlinked files; auto-commits and pushes
- **`sync`** — One-shot `git pull` + `install --all`; the easiest way to keep all devices current
- **`merge`** — Interactive diff + merge of MCP server configs between local device and dotfiles; per-server conflict resolution
- **`diff`** — Line-level unified diff between local Copilot config and the dotfiles repo (per file)
- **`validate`** — Checks repo structure, JSON validity, and agent frontmatter; exits 1 on errors
- **`restore`** — Interactive picker to restore `.bak` files left by previous install operations
- **`agent new`** — Scaffold a new `.agent.md` file interactively (name, description, model, instructions)
- **`mcp list/add/remove`** — Manage MCP servers in the dotfiles config; built-in registry for popular servers
- **`profile list/new/switch/delete`** — Named profiles for switching between multiple dotfiles repositories
- **`self-update`** — Check GitHub Releases and replace the running binary in-place
- **`completion install`** — Install shell completion for bash, zsh, fish, or PowerShell
- **Secrets protection** — `capture` scans MCP JSON for API key patterns and prompts before committing
- **Safe backups** — Existing files renamed to `.bak` before any install operation
- **Auto copy-mode on Windows** — Falls back to file copies when symlinks are unavailable; `status` shows `copied (in sync)` / `copied (out of sync)` accordingly
- **Auto-detection** — Finds your dotfiles repo from: `--dotfiles` flag → `COPILOT_DOTFILES_DIR` env → saved config → current directory → `~/copilot-dotfiles`
- **Git integration** — `capture` commits + pushes; `install` pulls; `status` shows working tree state
- **Colorized output** — Green/yellow/red marks, section headers, and tables

---

## Directory Layout

```
~/copilot-dotfiles/               ← your dotfiles repo
└── copilot/
    ├── agents/
    │   ├── my-agent.agent.md
    │   └── another.agent.md
    ├── mcp-config.macos.json
    ├── mcp-config.linux.json
    ├── mcp-config.windows.json
    ├── settings.json
    └── copilot-instructions.md

# macOS — managed by symlinks
~/Library/Application Support/GitHub Copilot/
├── agents/                  → ~/copilot-dotfiles/copilot/agents/
├── mcp-config.json          → ~/copilot-dotfiles/copilot/mcp-config.macos.json
├── settings.json            → ~/copilot-dotfiles/copilot/settings.json
└── copilot-instructions.md  → ~/copilot-dotfiles/copilot/copilot-instructions.md

# Linux — managed by symlinks
~/.config/github-copilot/        ← (or $XDG_CONFIG_HOME/github-copilot)

# Windows — managed by copies (symlinks require Developer Mode)
%APPDATA%\GitHub Copilot\
```

---

## Installation

### Download pre-built binary (recommended)

Go to the [Releases page](https://github.com/aburaihan-dev/copilot-dotfiles/releases/latest) and download the binary for your platform:

| Platform | File |
|----------|------|
| Windows | `copilot-sync-tool-vX.Y.Z-windows-amd64.exe` |
| macOS (Apple Silicon) | `copilot-sync-tool-vX.Y.Z-macos-arm64` |
| macOS (Intel) | `copilot-sync-tool-vX.Y.Z-macos-amd64` |
| Linux (x86_64) | `copilot-sync-tool-vX.Y.Z-linux-amd64` |
| Linux (ARM64) | `copilot-sync-tool-vX.Y.Z-linux-arm64` |

On macOS/Linux, make the binary executable:
```bash
chmod +x copilot-sync-tool-*-macos-arm64
```

### Build from source

```bash
git clone https://github.com/aburaihan-dev/copilot-dotfiles
cd copilot-dotfiles/cli
make build
# Binary: ./copilot-sync-tool
```

```bash
make build-all   # macOS (amd64+arm64), Linux (amd64+arm64), Windows (amd64)
make build-macos
make build-linux
make build-windows
```

---

## Quick Start

### New machine, no dotfiles repo yet

```bash
copilot-sync-tool init
# → choose a local directory
# → scaffold copilot/ folder structure
# → git init + add remote (optional)
# → capture existing Copilot config
# → initial commit + push (optional)
```

### New machine, dotfiles repo already exists

```bash
copilot-sync-tool setup
# → enter repo path or clone URL
# → saves path so you never need --dotfiles again
# → installs binary to PATH (optional)
# → runs install to apply config (optional)
```

---

## Commands

### Global Flags

```
--dotfiles string   Path to dotfiles repo
                    Resolution order: --dotfiles flag → $COPILOT_DOTFILES_DIR
                    → saved config → cwd (if copilot/ present) → ~/copilot-dotfiles
```

---

### `init`

Interactive wizard to create a new dotfiles repo from scratch.

```bash
copilot-sync-tool init
```

**Steps:**
1. Choose local directory (default: `~/copilot-dotfiles`)
2. Scaffold `copilot/`, `copilot/agents/`, platform MCP configs, `settings.json`, `copilot-instructions.md`
3. `git init` + optional remote URL
4. Capture existing Copilot config
5. Initial commit + push

---

### `setup`

Configure the tool on a machine that already has a dotfiles repo (local or remote).

```bash
copilot-sync-tool setup
```

**Steps:**
1. Enter a local path or git clone URL for your dotfiles repo
2. Saves the path to `%APPDATA%\copilot-sync-tool\config.json` (Windows) or `~/.config/copilot-sync-tool/config.json` (macOS/Linux)
3. Optionally installs the binary to `~/.local/bin` (macOS/Linux) or `%LOCALAPPDATA%\Programs\copilot-sync-tool\` (Windows)
4. Optionally runs `install` to apply config immediately

---

### `status`

Show the sync state of all managed Copilot config files.

```bash
copilot-sync-tool status
copilot-sync-tool status --dotfiles ~/my-dotfiles
```

**File sync states:**

| Status | Meaning |
|---|---|
| `✓ symlinked` | File is a symlink into the dotfiles repo |
| `✓ copied (in sync)` | File was copied and matches dotfiles (Windows copy-mode) |
| `⚠ copied (out of sync)` | File was copied but local differs from dotfiles — run `capture` |
| `⚠ not installed` | Dotfiles version exists but local file is missing — run `install` |
| `- missing in dotfiles` | Not yet captured — run `capture` |

**Also shows:** agent counts, MCP server counts, git branch, ahead/behind, working tree state, and untracked agents.

---

### `install`

Install Copilot config from the dotfiles repo onto this machine.

```bash
copilot-sync-tool install [flags]
```

**Default behaviour (no flags):** launches an interactive picker.

```
? Which components to install?
  ❯ ◉  Settings
    ◉  MCP Servers    ← second prompt lets you pick individual servers
    ◉  Agents         ← second prompt lets you pick individual agent files
    ◉  Instructions
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--all` | Install all components without prompting |
| `--mcp` | Install only MCP config (no prompt) |
| `--agents` | Install only agents (no prompt) |
| `--settings` | Install only settings (no prompt) |
| `--instructions` | Install only copilot-instructions.md (no prompt) |
| `--copy` | Copy files instead of symlinking (auto-enabled on Windows without Developer Mode) |
| `--pull` | Git pull before installing (default: true; use `--no-pull` to skip) |

**Examples:**

```bash
# Interactive picker (default)
copilot-sync-tool install

# Install everything silently
copilot-sync-tool install --all

# Install only agents without pulling
copilot-sync-tool install --agents --no-pull

# Force copy mode
copilot-sync-tool install --all --copy
```

---

### `capture`

Copy local Copilot config into the dotfiles repo. Already-symlinked files are skipped automatically.

```bash
copilot-sync-tool capture [flags]
```

**Flags:**

| Flag | Description |
| ---- | ----------- |
| `--agents` | Capture only agents |
| `--mcp` | Capture only MCP config |
| `--settings` | Capture only settings |
| `--instructions` | Capture only copilot-instructions.md |
| `--push` | Auto git commit+push after capture (default: true; use `--no-push` to skip) |
| `--message string` | Custom git commit message |

**Secrets protection:** Before committing, `capture` scans captured JSON files for patterns matching common API keys and tokens (GitHub PATs, OpenAI keys, AWS credentials, etc.). If any are found, you are prompted to confirm before the commit proceeds. Replace secret values with environment variable placeholders like `${MY_TOKEN}` to avoid the warning.

**Examples:**

```bash
# Capture everything and push
copilot-sync-tool capture

# Capture only agents, skip push
copilot-sync-tool capture --agents --no-push

# Custom commit message
copilot-sync-tool capture --mcp --message "feat: add new MCP servers"
```

---

### `merge`

Interactively merge MCP server configs between local device and dotfiles.

```bash
copilot-sync-tool merge [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--ai` | Generate an AI merge prompt; optionally invokes `copilot` CLI if found in PATH |

**Workflow:**
1. Loads local `mcp-config.json` and dotfiles platform config
2. Shows diff: only-local / only-dotfiles / same / different
3. Prompts to add local-only servers to dotfiles
4. For conflicts: shows both configs side-by-side and prompts for which to keep
5. Writes merged result to dotfiles

```bash
copilot-sync-tool merge
copilot-sync-tool merge --ai
```

---

### `sync`

One-shot pull + install — the easiest way to keep a machine current.

```bash
copilot-sync-tool sync

# Skip git pull, just re-apply from current dotfiles state
copilot-sync-tool sync --no-pull
```

Equivalent to running `git pull` then `install --all` in sequence.

---

### `diff`

Show a line-level unified diff between local Copilot config and the dotfiles repo.

```bash
# Diff all managed files
copilot-sync-tool diff

# Diff only MCP config and settings
copilot-sync-tool diff --mcp --settings
```

**Flags:** `--mcp`, `--agents`, `--settings`, `--instructions` (default: all)

When files are symlinked the diff will always be empty (same underlying file).
Most useful in Windows copy-mode to find what has drifted.

---

### `validate`

Check your dotfiles repo for structural and content issues before committing.

```bash
copilot-sync-tool validate
copilot-sync-tool validate --dotfiles ~/my-dotfiles
```

**Checks:**

- Required directories exist (`copilot/`, `copilot/agents/`)
- Platform MCP config files are present for windows / macos / linux
- All present JSON files parse without errors
- Every `.agent.md` file has a valid `---...---` frontmatter block

Exits with code 1 if any check fails (suitable for CI).

---

### `restore`

Restore `.bak` backup files created by previous `install` operations.

```bash
# Interactive picker
copilot-sync-tool restore

# Restore all backups without prompting
copilot-sync-tool restore --all
```

---

### `agent new`

Scaffold a new `.agent.md` file interactively.

```bash
copilot-sync-tool agent new

# Create and immediately copy to the local Copilot agents dir
copilot-sync-tool agent new --capture
```

Prompts for: name, description, model (with a curated list), and system instructions. Writes the file to `copilot/agents/` in your dotfiles repo with correct frontmatter.

---

### `mcp list/add/remove`

Manage MCP servers directly in your dotfiles config.

```bash
# List servers in dotfiles config for current platform
copilot-sync-tool mcp list

# Add from built-in registry (github, filesystem, fetch, memory, sequential-thinking)
copilot-sync-tool mcp add github

# Add interactively (pick from registry or enter custom JSON)
copilot-sync-tool mcp add

# Add to all three platform configs at once
copilot-sync-tool mcp add github --all-platforms

# Remove a server (interactive picker if name omitted)
copilot-sync-tool mcp remove
copilot-sync-tool mcp remove github
```

After adding/removing, run `copilot-sync-tool capture --mcp` to commit and push.

---

### `profile list/new/switch/delete`

Named profiles let you switch between multiple dotfiles repositories on the same machine (e.g. personal vs. work).

```bash
# List configured profiles
copilot-sync-tool profile list

# Create a new profile
copilot-sync-tool profile new work --dotfiles ~/work-dotfiles

# Switch active profile interactively
copilot-sync-tool profile switch

# Switch to a specific profile
copilot-sync-tool profile switch work

# Delete a profile
copilot-sync-tool profile delete work
```

All other commands (`install`, `capture`, `status`, `diff`, etc.) automatically use the active profile's dotfiles directory.

---

### `self-update`

Download and install the latest release binary in-place.

```bash
# Check for update and install if available
copilot-sync-tool self-update

# Only print the latest version, don't download
copilot-sync-tool self-update --check
```

On Windows, a helper `.bat` file is written to complete the replacement after the process exits (Windows cannot replace a running executable directly).

---

### `completion install`

Install tab-completion for your shell.

```bash
# Auto-detect shell and install
copilot-sync-tool completion install

# Specify shell explicitly
copilot-sync-tool completion install --shell zsh

# Generate script to stdout for manual setup
copilot-sync-tool completion bash
copilot-sync-tool completion zsh
copilot-sync-tool completion fish
copilot-sync-tool completion powershell
```

Writes the completion script to the standard location for your shell and appends a source line to your profile file.

---

## Cross-Platform Notes

### macOS / Linux

Symlinks work out of the box. Default install mode creates symlinks so `git pull` in the dotfiles repo instantly reflects everywhere.

### Windows

Symlink creation requires one of:
- **Developer Mode** enabled (Settings → Privacy & Security → For developers)
- Running terminal **as Administrator**

Without either, the tool automatically switches to `--copy` mode. `status` will show `copied (in sync)` or `copied (out of sync)` instead of `symlinked`.

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `COPILOT_DOTFILES_DIR` | Override the dotfiles repo path |

---

## Development

```bash
# Build for current platform
make build

# Build all platforms
make build-all

# Tidy dependencies
make tidy

# Clean build artifacts
make clean
```

**Project structure:**

```
cli/
├── main.go
├── Makefile
├── go.mod
└── cmd/
│   ├── root.go          # cobra root, GetDotfilesDir(), version
│   ├── init.go          # init command
│   ├── setup.go         # setup command
│   ├── status.go        # status command
│   ├── capture.go       # capture command (+ secrets scan)
│   ├── install.go       # install command
│   ├── merge.go         # merge command
│   ├── sync.go          # sync command
│   ├── diff.go          # diff command
│   ├── validate.go      # validate command
│   ├── restore.go       # restore command
│   ├── agent.go         # agent new subcommand
│   ├── mcpmanage.go     # mcp list/add/remove subcommands
│   ├── profile.go       # profile list/new/switch/delete subcommands
│   ├── selfupdate.go    # self-update command
│   └── completion.go    # completion install command
└── internal/
    ├── platform/        # path resolution per OS
    ├── mcp/             # MCP config load/save/diff/merge
    ├── agents/          # agent file listing and capture
    ├── symlink/         # symlink create/check with Windows handling
    ├── git/             # git status/branch/commit/push/pull
    ├── config/          # saved tool config (dotfiles path + profiles)
    ├── secrets/         # secret pattern scanner for JSON files
    └── ui/              # colorized output helpers and table renderer
```
