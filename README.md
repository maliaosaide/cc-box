<div align="center">
  <img src="./gui/assets/icons/generated/appicon.png" alt="CC-Box" width="96" height="96" />

  <h1>CC-Box</h1>

  <p><strong>Sync, encrypt, version, and rollback your Claude Code setup.</strong></p>

  <p>
    <img src="https://img.shields.io/badge/version-v0.4.0-ok?style=flat-square&color=c4704e" alt="Version" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-blue?style=flat-square" alt="Platform" />
    <img src="https://img.shields.io/badge/built_with-Wails-09f?style=flat-square&color=df2929" alt="Built with Wails" />
    <img src="https://img.shields.io/github/downloads/maliaosaide/cc-box/total?style=flat-square&color=6b9080" alt="Downloads" />
  </p>

  <p>
    <b>English</b> | <a href="./readme/README_CN.md">中文</a>
  </p>
</div>

---

## What is CC-Box?

CC-Box puts your entire Claude Code environment — config files, skills, agents, MCP setups, project `.claude.json`, and even the Claude binary itself — into a **Git-like snapshot system** backed by your own **WebDAV storage** with **end-to-end encryption**.

Switch computers? Spin up a new VM? CC-Box restores your Claude Code exactly how you left it. No more dragging `~/.claude/` around on a USB stick or praying a cloud sync doesn't corrupt your settings.

## Why You'll Love It

- **One-click sync across all your machines.** Desktop, laptop, server — push from one, pull on another.
- **Snapshots, not overwrites.** Every sync creates a versioned snapshot. Browse history, view diffs, rollback anytime.
- **End-to-end encrypted.** Argon2id + AES-256-GCM. Your WebDAV provider sees encrypted blobs, not your configs.
- **Conflict resolution that makes sense.** Side-by-side view with Git-style inline markers. Pick local, remote, or merge.
- **Claude binary management built in.** Install official releases, GitHub versions, or your own WebDAV backups. Switch versions in one click.
- **Project `.claude.json` sync.** Keep per-project MCP configs in sync across devices.
- **CLI for scripting, GUI for daily use.** Both share the same core — choose what fits your workflow.

## Get Started in 30 Seconds

### Download a Release

Grab the latest from [Releases](https://github.com/maliaosaide/cc-box/releases):

| File | What |
|------|------|
| `cc-box.exe` | CLI tool (command-line) |
| `cc-box-gui.exe` | Desktop app (Windows) |

### Or Build from Source

**Prerequisites:** Go 1.25+, Node.js, Wails CLI v2.x

```bash
# CLI
go -C cli build -o cc-box.exe ./cmd/cc-box/

# GUI (Windows/macOS/Linux)
cd gui && wails build
```

## How It Works

```
Your Machine A                              Your WebDAV Server                    Your Machine B
─────────────                              ──────────────────                    ─────────────
~/.claude/          ── encrypt ──→        snapshots/*.enc                       ~/.claude/
  settings.json                           objects/{hash}          ── decrypt ──→   settings.json
  CLAUDE.md                               HEAD                                      CLAUDE.md
  skills/                                 salt.bin                                  skills/
  agents/                                                                            agents/
~/.claude.json       ── encrypt ──→       (included in snapshot)    ── decrypt ──→ ~/.claude.json
```

1. **Scan** — walks `~/.claude/` and `~/.claude.json`, hashes every file.
2. **Encrypt** — derives key from your password, encrypts with AES-256-GCM.
3. **Upload** — pushes objects and snapshot to your WebDAV.
4. **HEAD** — atomic pointer to the latest snapshot, protected by ETag-based CAS.

## Features

### Config File Sync & Browsing

Browse your synced config tree with status badges (modified, added, conflict, etc.). View file content with full scrolling support, inspect line-by-line diffs, and resolve conflicts with Git-style inline markers.

<p align="center"><i>Sync status, diffs, and conflict resolution — all from the Files page.</i></p>

### Claude Binary Management

Install, backup, switch, and rollback Claude binaries without leaving the app.

| Source | Use Case |
|--------|----------|
| **Official Installer** | Latest stable release, one click. |
| **GitHub Releases** | Pin a specific version from any release. SHA256 verified. |
| **WebDAV Backup** | Restore a version you previously uploaded. Works offline from GitHub. |

All installations target the official Claude path (`~/.local/bin/claude` / `~/.local/bin/claude.exe`).

### Dashboard with Live Status

At-a-glance sync health: connection status, pending changes, conflict count, last sync time, Claude version, and device list.

### Snapshot History & Rollback

Full timeline of every sync. Click to inspect file lists, revert to a previous state, or restore a specific version.

### Project `.claude.json` Sync

Discover projects with `.claude.json` via git remotes. Push, pull, and merge MCP configs across devices.

### System Tray (GUI)

Minimize to tray. Real-time sync state and conflict indicators. Single-instance — re-opening the app brings the existing window to front.

## CLI Quick Reference

```bash
cc-box init                # First-time setup: WebDAV, encryption, device
cc-box status              # Check sync status
cc-box push                # Upload local changes
cc-box pull                # Download remote changes
cc-box sync                # Pull then push
cc-box diff path/to/file   # View file differences
cc-box conflicts           # List and resolve conflicts
cc-box log                 # Snapshot history
cc-box revert <snap-id>    # Rollback to a snapshot

cc-box binary list         # List available Claude versions
cc-box binary push         # Backup current Claude binary
cc-box binary pull <ver>   # Restore a backed-up version
cc-box binary install --source github --version 1.2.3

cc-box project list        # List discovered project configs
cc-box project push        # Push project .claude.json files
```

## Tech Stack

| Layer | Technology |
|-------|------------|
| Language | Go 1.25+ |
| CLI Framework | Cobra + Viper |
| GUI Framework | [Wails v2](https://wails.io) |
| Frontend | Svelte + Vite + Tailwind CSS |
| Encryption | Argon2id → AES-256-GCM |
| Storage | WebDAV (any provider) |
| File Watching | fsnotify |
| System Tray | systray |

## WebDAV Providers

Works with any standard WebDAV service:

- **Alist** · **NextCloud** · **坚果云 (JianGuoYun)** · **Synology** · **Self-hosted**

Concurrent multi-device sync relies on proper ETag / If-Match / If-None-Match support.

## Build

```bash
# CLI
go -C cli build -o cc-box.exe ./cmd/cc-box/

# GUI (requires Wails CLI v2.x + Node.js)
cd gui && wails build

# Output:
#   cli/cc-box.exe
#   gui/build/bin/cc-box-gui.exe
```

Run tests:
```bash
go -C core test ./...
go -C cli test ./...
go -C gui test ./...
npm --prefix gui/frontend run build
```

## Project Structure

```
cc-box/
├── core/           # Shared: config, crypto, snapshot, sync, WebDAV, binary
├── cli/            # CLI app (Cobra commands)
├── gui/            # Wails desktop app (Svelte frontend)
│   ├── frontend/   # Svelte + Vite + Tailwind
│   └── internal/   # Desktop adapters, project management
└── readme/         # Translated READMEs
```

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `CC_BOX_WEBDAV_PASSWORD` | WebDAV password for non-interactive CLI use |

## License

MIT

---

## Star History

<p align="center">
  <a href="https://star-history.com/#maliaosaide/cc-box&Date">
    <img src="https://api.star-history.com/svg?repos=maliaosaide/cc-box&type=Date" alt="Star History Chart" width="600" />
  </a>
</p>
