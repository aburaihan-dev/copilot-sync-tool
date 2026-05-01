# v1.8.0 Implementation Plan — Claude & Claude Desktop Support

## Goal

Extend `copilot-sync-tool` to manage **GitHub Copilot CLI**, **Claude Desktop**, and
**Claude CLI (Claude Code)** configs — all in one dotfiles repo — with the ability to
sync a shared MCP server list to all three tools at once.

## Config paths per tool

| Tool | macOS config path | Windows | Linux | MCP key |
|---|---|---|---|---|
| Copilot CLI | `~/Library/Application Support/GitHub Copilot/mcp-config.json` | `%APPDATA%\GitHub Copilot\mcp-config.json` | `~/.config/github-copilot/mcp-config.json` | `mcpServers` |
| Claude Desktop | `~/Library/Application Support/Claude/claude_desktop_config.json` | `%APPDATA%\Claude\claude_desktop_config.json` | `~/.config/Claude/claude_desktop_config.json` | `mcpServers` |
| Claude CLI | `~/.claude.json` (global settings, merge-write) | same | same | `mcpServers` (nested) |
| Claude instructions | `~/.claude/CLAUDE.md` | same | same | n/a |
| Claude skills | `~/.claude/skills/` | same | same | n/a |

All three tools use the same `mcpServers` JSON format — the MCP config is fully portable.

## New dotfiles structure

```
copilot-dotfiles/
  copilot/                                  # existing
    mcp-config.{macos,linux,windows}.json
    settings.json
    agents/
    copilot-instructions.md
  claude-desktop/                           # NEW
    mcp-config.macos.json
    mcp-config.linux.json
    mcp-config.windows.json
  claude/                                   # NEW
    mcp-config.json                         # cross-platform → ~/.claude.json (mcpServers only)
    CLAUDE.md                               # global instructions → ~/.claude/CLAUDE.md
    skills/                                 # skills dir → ~/.claude/skills/
  shared/                                   # NEW
    mcp-servers.json                        # MCP servers deployed to ALL tools
```

## New `--tool` flag

Added to: `capture`, `install`, `sync`, `status`, `diff`, `validate`

| Value | Meaning |
|---|---|
| `copilot` | GitHub Copilot CLI only (default for capture/install — backward compat) |
| `claude-desktop` | Claude Desktop only |
| `claude` | Claude CLI only |
| `all` | All three tools (default for `sync`) |

```bash
copilot-sync-tool status --tool all
copilot-sync-tool capture --tool claude-desktop
copilot-sync-tool install --tool all
copilot-sync-tool sync                        # implies --tool all
```

## New `deploy` command

```bash
copilot-sync-tool deploy [--from shared] [--to copilot,claude-desktop,claude]
```

Merges `shared/mcp-servers.json` entries into each target tool's MCP config in dotfiles,
then commits and pushes. Additive merge only — never removes existing servers.

---

## Implementation Phases

### Phase 1 — Platform paths (`internal/platform/platform.go`)
- [ ] `ClaudeDesktopDir()` — platform-specific Claude Desktop config dir
- [ ] `ClaudeDesktopMCPConfigFile()` → `claude_desktop_config.json`
- [ ] `ClaudeDir()` → `~/.claude/`
- [ ] `ClaudeMCPConfigFile()` → `~/.claude.json`
- [ ] `ClaudeInstructionsFile()` → `~/.claude/CLAUDE.md`
- [ ] `ClaudeSkillsDir()` → `~/.claude/skills/`
- [ ] Dotfiles helpers: `DotfilesClaudeDesktopMCPConfig()`, `DotfilesClaudeMCPConfig()`,
      `DotfilesClaudeInstructions()`, `DotfilesClaudeSkillsDir()`, `DotfilesSharedMCPConfig()`

**Depends on:** nothing

---

### Phase 2 — MCP package (`internal/mcp/mcp.go`)
- [ ] `LoadClaudeCLI(path)` — reads `~/.claude.json`, extracts only `mcpServers` into `*Config`
- [ ] `SaveClaudeCLI(path, cfg *Config)` — writes `mcpServers` back into `~/.claude.json`
      without touching other keys (read → patch → write)
- [ ] `MergeInto(base, additions *Config) *Config` — additive merge for shared → target

**Depends on:** Phase 1

---

### Phase 3 — `--tool` flag (`cmd/root.go` + helpers)
- [ ] Define `ToolTarget` type: `ToolCopilot`, `ToolClaudeDesktop`, `ToolClaude`, `ToolAll`
- [ ] `ParseTools(s string) ([]ToolTarget, error)` — parse comma-separated tool list
- [ ] Add `--tool` persistent flag to root command
- [ ] Wire `--tool` into `capture`, `install`, `sync`, `status`, `diff`, `validate`

**Depends on:** Phase 1

---

### Phase 4 — `status` command (`cmd/status.go`)
- [ ] Add **Claude Desktop** section: symlink check for `claude_desktop_config.json`, MCP count
- [ ] Add **Claude CLI** section: check `~/.claude.json`, `CLAUDE.md`, `skills/`; MCP count
- [ ] Show tool sections only when `--tool` includes that tool (default: show all)

**Depends on:** Phase 3

---

### Phase 5 — `capture` command (`cmd/capture.go`)
- [ ] `--tool claude-desktop`: copy `claude_desktop_config.json` → dotfiles `claude-desktop/`
- [ ] `--tool claude`: copy `mcpServers` from `~/.claude.json` + `CLAUDE.md` + `skills/` → dotfiles `claude/`
- [ ] `--tool all`: capture all three tools

**Depends on:** Phase 3

---

### Phase 6 — `install` command (`cmd/install.go`)
- [ ] `--tool claude-desktop`: symlink/copy `claude_desktop_config.json` from dotfiles
- [ ] `--tool claude`: install `mcpServers` into `~/.claude.json` (merge-write) + symlink `CLAUDE.md` + `skills/`
- [ ] Interactive picker (`-i`) lists components from all selected tools

**Depends on:** Phase 3

---

### Phase 7 — `deploy` command (`cmd/deploy.go`) — NEW
- [ ] New `deploy` subcommand
- [ ] `--from shared` (default): read `shared/mcp-servers.json`
- [ ] `--to copilot,claude-desktop,claude` (default: all): merge into each tool's dotfiles MCP config
- [ ] Additive merge only — never deletes existing servers
- [ ] Commit + push after merge (with `--no-push` escape hatch)

**Depends on:** Phase 2

---

### Phase 8 — `sync`, `diff`, `validate` (`cmd/sync.go`, `cmd/diff.go`, `cmd/validate.go`)
- [ ] `sync`: default to `--tool all`
- [ ] `diff`: show per-tool diff when multiple tools selected
- [ ] `validate`: validate Claude Desktop and Claude CLI configs in dotfiles

**Depends on:** Phase 3

---

### Phase 9 — Dotfiles repo updates (`copilot-dotfiles`)
- [ ] Add `claude-desktop/mcp-config.{macos,linux,windows}.json` (empty `{"mcpServers":{}}`)
- [ ] Add `claude/mcp-config.json`, `claude/CLAUDE.md`, `claude/skills/` placeholder
- [ ] Add `shared/mcp-servers.json` (empty `{"mcpServers":{}}`)
- [ ] Update `install.sh` / `install.ps1` for new dirs
- [ ] Update `copilot/copilot-instructions.md` to document new tool support

**Depends on:** Phase 6

---

### Phase 10 — Build and release
- [ ] Build local binary and verify `copilot-sync-tool status --tool all` works
- [ ] `make build-all` for all 5 platform binaries
- [ ] Tag `v1.8.0` and push — GitHub Actions auto-publishes release
- [ ] Pull submodule update in `copilot-dotfiles`

**Depends on:** Phase 9

---

## Key decisions

| Decision | Rationale |
|---|---|
| `~/.claude.json` uses **merge-write** not symlink | File contains non-MCP keys (auth, preferences); symlinking would expose them to git |
| `~/.claude/CLAUDE.md` and `skills/` **can be symlinked** | Like Copilot agents — pure content, safe to track |
| `claude_desktop_config.json` **can be symlinked** | It's purely MCP config, nothing sensitive |
| `--tool` defaults to `copilot` on capture/install | Backward compatibility — existing users unaffected |
| `sync` defaults to `--tool all` | New behavior, clearly documented |
| Shared MCP merge is **additive only** | Prevent accidental deletion of tool-specific servers |
