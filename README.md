# copilot-sync-tool

[![Release](https://img.shields.io/github/v/release/aburaihan-dev/copilot-sync-tool)](https://github.com/aburaihan-dev/copilot-sync-tool/releases/latest)
[![Go](https://img.shields.io/badge/Go-1.22+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/github/license/aburaihan-dev/copilot-sync-tool)](LICENSE)

Sync your GitHub Copilot CLI config — agents, MCP servers, settings, and instructions — across macOS, Linux, and Windows using a Git dotfiles repo.

---

## Contents

- [Quick Start](#quick-start)
- [Features](#features)
- [Installation](#installation)
- [Commands](#commands)
- [Directory Layout](#directory-layout)
- [Cross-Platform Notes](#cross-platform-notes)
- [Development](#development)

---

## Quick Start

### New machine — no dotfiles repo yet

```bash
copilot-sync-tool init
```

Walks you through scaffolding a repo, capturing existing config, and pushing to Git.

### New machine — dotfiles repo already exists

```bash
copilot-sync-tool setup
# enter your repo path or clone URL → saved for all future commands
```

Then apply config:

```bash
copilot-sync-tool install      # interactive picker
copilot-sync-tool install --all  # install everything at once
```

### Daily driver (keep all devices current)

```bash
copilot-sync-tool sync   # git pull + install --all in one shot
```

### Capture changes from this machine

```bash
copilot-sync-tool capture   # copy local config → dotfiles → commit + push
```

---

## Features

### Setup & onboarding

| Feature | Command | Description |
|---|---|---|
| Bootstrap from scratch | `init` | Scaffolds `copilot/` structure, captures existing config, runs `git init` and pushes |
| Connect on a new machine | `setup` | Register a local path or clone URL; saves it so `--dotfiles` is never needed again |
| Apply config to this machine | `install` | Interactive picker or `--all`; creates symlinks (or copies on Windows) with `.bak` safety |
| Switch dotfiles repos | `profile switch` | Named profiles for personal / work repos; all commands use the active profile |

### Daily sync

| Feature | Command | Description |
|---|---|---|
| Pull + install in one shot | `sync` | `git pull` → `install --all` — the only command most days |
| Save local changes | `capture` | Copies local config into dotfiles, scans for secrets, commits and pushes |
| Check sync state | `status` | Symlink state per file, agent + MCP counts, git branch, ahead/behind, untracked items |
| Line-level drift | `diff` | Shows exactly what has changed between local config and dotfiles |

### Conflict resolution & safety

| Feature | Command | Description |
|---|---|---|
| Resolve MCP conflicts | `merge` | Per-server interactive resolution; `--ai` generates a merge prompt for Copilot |
| Validate dotfiles | `validate` | Checks JSON validity, directory structure, agent frontmatter — exits 1 for CI |
| Undo an install | `restore` | Interactive or `--all`; restores from `.bak` files created during install |
| Secrets scan | _(automatic)_ | Detects API key patterns in MCP JSON before any commit |
| Safe backups | _(automatic)_ | Existing files renamed to `.bak` before any install step |

### Agent & MCP management

| Feature | Command | Description |
|---|---|---|
| Scaffold a new agent | `agent new` | Creates a properly formatted `.agent.md`; `--capture` also installs it locally |
| List MCP servers | `mcp list` | Shows all servers across platform configs |
| Add from registry | `mcp add` | Interactive picker or `mcp add github`; names include `github`, `filesystem`, `fetch`, `memory` |
| Add to all platforms | `mcp add --all-platforms` | Writes to macOS, Linux, and Windows configs in one step |
| Remove a server | `mcp remove` | Interactive picker to remove from any platform config |

### Utilities

| Feature | Command | Description |
|---|---|---|
| Update the binary | `self-update` | Downloads latest release and replaces binary in-place; `--check` prints version only |
| Shell tab completion | `completion install` | Auto-detects shell; supports bash, zsh, fish, PowerShell |
| Cross-platform | _(automatic)_ | macOS/Linux use symlinks; Windows auto-falls back to copy mode if symlinks unavailable |

---

## Installation

### Download pre-built binary (recommended)

Download from the [Releases page](https://github.com/aburaihan-dev/copilot-sync-tool/releases/latest):

| Platform | File |
|---|---|
| macOS (Apple Silicon) | `copilot-sync-tool-macos-arm64` |
| macOS (Intel) | `copilot-sync-tool-macos-amd64` |
| Linux (x86_64) | `copilot-sync-tool-linux-amd64` |
| Linux (ARM64) | `copilot-sync-tool-linux-arm64` |
| Windows | `copilot-sync-tool-windows-amd64.exe` |

```bash
# macOS / Linux — make executable and move to PATH
chmod +x copilot-sync-tool-macos-arm64
sudo mv copilot-sync-tool-macos-arm64 /usr/local/bin/copilot-sync-tool
```

### Build from source

```bash
git clone https://github.com/aburaihan-dev/copilot-sync-tool
cd copilot-sync-tool
make build        # current platform → ./copilot-sync-tool
make build-all    # all 5 platforms → dist/
```

### Keep it updated

```bash
copilot-sync-tool self-update
```

---

## Commands

**Global flag:** `--dotfiles <path>` — override dotfiles repo location.
Auto-detected in order: `--dotfiles` → `$COPILOT_DOTFILES_DIR` → saved config → `~/copilot-dotfiles`.

---

### `init` — Bootstrap a new dotfiles repo

```bash
copilot-sync-tool init
```

Scaffolds `copilot/` structure, runs `git init`, captures existing config, and pushes.

---

### `setup` — Configure tool on existing machine

```bash
copilot-sync-tool setup
```

Enter a local path or clone URL. Saves the path so `--dotfiles` is never needed again. Optionally installs the binary to PATH and runs `install`.

---

### `status` — See what's in sync

```bash
copilot-sync-tool status
```

| Status | Meaning |
|---|---|
| `✓ symlinked` | Managed via symlink — always in sync |
| `✓ copied (in sync)` | File copy matches dotfiles |
| `⚠ copied (out of sync)` | Drifted — run `capture` |
| `⚠ not installed` | Exists in dotfiles but not on this machine — run `install` |
| `- missing in dotfiles` | Not captured yet — run `capture` |

Also shows: agent counts, MCP server counts, git branch, ahead/behind, untracked agents.

---

### `install` — Apply dotfiles to this machine

```bash
copilot-sync-tool install          # interactive picker (default)
copilot-sync-tool install --all    # install everything silently
copilot-sync-tool install --mcp --agents  # specific components
copilot-sync-tool install --copy   # copy instead of symlinking
copilot-sync-tool install --no-pull  # skip git pull
```

The interactive picker lets you choose individual MCP servers and agent files.

---

### `capture` — Save local config to dotfiles

```bash
copilot-sync-tool capture              # capture all, commit + push
copilot-sync-tool capture --agents     # agents only
copilot-sync-tool capture --mcp --no-push  # MCP only, skip push
copilot-sync-tool capture --message "feat: add new server"
```

Skips already-symlinked files. Scans for API key patterns before committing.

---

### `sync` — Pull + install in one shot

```bash
copilot-sync-tool sync           # git pull → install --all
copilot-sync-tool sync --no-pull # re-install from current dotfiles
```

---

### `merge` — Resolve MCP config conflicts

```bash
copilot-sync-tool merge       # interactive per-server resolution
copilot-sync-tool merge --ai  # generate AI-assisted merge prompt
```

---

### `diff` — See what has drifted

```bash
copilot-sync-tool diff              # diff all managed files
copilot-sync-tool diff --mcp --settings
```

Most useful on Windows copy-mode to find drift before running `capture`.

---

### `validate` — Check dotfiles health (CI-friendly)

```bash
copilot-sync-tool validate
```

Checks: directory structure, JSON validity, agent frontmatter. Exits 1 on errors.

---

### `restore` — Undo an install

```bash
copilot-sync-tool restore        # interactive picker
copilot-sync-tool restore --all  # restore all .bak files
```

---

### `agent` — Manage agent files

```bash
copilot-sync-tool agent new            # scaffold a new .agent.md interactively
copilot-sync-tool agent new --capture  # create and install to local Copilot
```

---

### `mcp` — Manage MCP servers

```bash
copilot-sync-tool mcp list              # list servers in dotfiles
copilot-sync-tool mcp add               # interactive picker from registry
copilot-sync-tool mcp add github        # add by name (github, filesystem, fetch, memory…)
copilot-sync-tool mcp add github --all-platforms  # add to all three platform configs
copilot-sync-tool mcp remove            # interactive remove
```

---

### `profile` — Multiple dotfiles repos

```bash
copilot-sync-tool profile list
copilot-sync-tool profile new work --dotfiles ~/work-dotfiles
copilot-sync-tool profile switch        # interactive
copilot-sync-tool profile switch work
copilot-sync-tool profile delete work
```

All commands use the active profile's dotfiles directory automatically.

---

### `self-update` — Update the binary

```bash
copilot-sync-tool self-update          # download and replace binary
copilot-sync-tool self-update --check  # print latest version only
```

---

### `completion` — Shell tab completion

```bash
copilot-sync-tool completion install           # auto-detect shell
copilot-sync-tool completion install --shell zsh
```

---

## Directory Layout

```
~/copilot-dotfiles/               ← your dotfiles repo
└── copilot/
    ├── agents/
    │   └── my-agent.agent.md
    ├── mcp-config.macos.json
    ├── mcp-config.linux.json
    ├── mcp-config.windows.json
    ├── settings.json
    └── copilot-instructions.md
```

**Managed symlinks (macOS example):**

```
~/Library/Application Support/GitHub Copilot/
├── agents/                  → copilot-dotfiles/copilot/agents/
├── mcp-config.json          → copilot-dotfiles/copilot/mcp-config.macos.json
├── settings.json            → copilot-dotfiles/copilot/settings.json
└── copilot-instructions.md  → copilot-dotfiles/copilot/copilot-instructions.md
```

Linux uses `~/.config/github-copilot/` (respects `$XDG_CONFIG_HOME`).
Windows uses `%APPDATA%\GitHub Copilot\` with file copies instead of symlinks.

---

## Cross-Platform Notes

### macOS / Linux

Symlinks work out of the box. `git pull` in the dotfiles repo instantly reflects on the machine.

### Windows

Symlinks require **Developer Mode** (Settings → Privacy & Security → For developers) or running as Administrator. Without either, the tool auto-falls back to copy mode — `status` will show `copied (in sync/out of sync)`.

---

## Development

```bash
make build      # build for current platform
make build-all  # build all 5 platforms into dist/
make tidy       # go mod tidy
make clean      # remove build artifacts
```

**Package layout:**

```
cmd/          # cobra commands (one file per command)
internal/
  platform/   # OS-specific config paths
  mcp/        # MCP config load / save / diff / merge
  agents/     # agent file listing and capture
  symlink/    # cross-platform symlink helpers
  git/        # git operations via exec
  config/     # saved tool config and profiles
  secrets/    # API key pattern scanner
  ui/         # colorized output, tables, prompts
```

---

> **Roadmap:** v1.8.0 will add [Claude Desktop + Claude CLI support](https://github.com/aburaihan-dev/copilot-sync-tool/issues/1) — sync MCP servers across Copilot, Claude Desktop, and Claude CLI from one dotfiles repo.
