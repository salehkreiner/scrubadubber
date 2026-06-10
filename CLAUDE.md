# scrubadubber — Project Context

## What This Repo Is

The user-facing product. A native system tray app (Windows) and menubar app (macOS)
that silently manages the Scrubadubber Hub and bridge in the background, giving
developers one-click access to **on-device pseudonymization and agent egress
control** — sensitive values are replaced with reversible pseudonyms before traffic
leaves the machine, with the re-identification key held locally — at zero ongoing
friction. Positioning: **free for individuals, enforceable and validated for
organizations.**

This is the repo users interact with. It downloads, installs, and manages the
Hub and bridge binaries — it does not contain pseudonymization logic itself.

## Architecture Position

```
scrubadubber (this repo — public)
    ├── downloads + manages → Hub binary (public mirror: scrubadubber-hub-releases)
    └── downloads + manages → bridge-claude-code binary (from public releases)
```

The Hub *source* stays private (scrubadubber-hub); its release workflow
cross-publishes the compiled binaries (+ SHA256SUMS + config.example.yaml) to the
public scrubadubber-hub-releases mirror so the installer can fetch them
unauthenticated. This repo only knows the Hub's binary download URL; the
pseudonymization logic is never exposed here.

## Target Users

* Individual developers using Claude Code, Aider, or other LLM CLI tools
* Teams where an admin installs once and developers use it transparently
* First 10 testers include Windows and Mac users — both platforms are required

## Strict Scope

Build ONLY the tray/menubar app and its installer. Do not re-implement any
pseudonymization, masking, or detection logic. Do not vendor the Hub or bridge source.

## User Experience Goal

1. User downloads scrubadubber-setup.exe (Windows) or scrubadubber.dmg (Mac)
2. Double-clicks, clicks through a minimal installer (elevation expected here — once)
3. Tray/menubar icon appears — green = protected, red = Hub unreachable
4. User types `claude` exactly as before — nothing changes in their workflow
5. That's it. No terminals, no Go commands, no manual PATH editing, ever.

## Platform Targets

* Windows (primary): amd64. System tray icon via systray library.
* macOS (required for v1.0): amd64 + arm64 (Apple Silicon). Menubar icon via systray.

## Core Components

### 1\. Tray/Menubar App (cmd/scrubadubber/main.go)

The always-on background process. Uses github.com/getlantern/systray (or
fyne.io/systray — evaluate and justify choice) for cross-platform tray support.

Menu structure:

```
● Scrubadubber                    ← title, not clickable
  ─────────────────
  ✓ Protected — Claude Code       ← green when Hub reachable + ANTHROPIC\_BASE\_URL set
  ─────────────────
  Start Hub                       ← shown when Hub not running
  Stop Hub                        ← shown when Hub running
  Restart Hub                     ← always shown
  ─────────────────
  View Logs...                    ← opens log file in default text viewer
  Settings...                     ← opens a minimal settings window (see §4)
  ─────────────────
  About Scrubadubber v1.0.0
  Check for Updates
  ─────────────────
  Quit
```

Tray icon states:

* Green circle: Hub running, health check passing, bridge env set
* Yellow circle: Hub running but health check degraded
* Red circle: Hub not running or unreachable
* Grey circle: Scrubadubber itself is starting up

### 2\. Hub Process Manager (internal/hubmanager/manager.go)

Manages the Hub binary as a child process:

* Start/stop/restart the Hub process
* Watch for unexpected exits and auto-restart (with backoff)
* Health-check loop: GET :8384/healthz every 10 seconds
* Update tray icon state based on health
* Pipe Hub logs to a rotating log file

Hub binary location: platform data directory

* Windows: %LOCALAPPDATA%\\scrubadubber\\bin\\hub.exe
* macOS: \~/Library/Application Support/scrubadubber/bin/hub

### 3\. Startup Registration (internal/startup/)

* startup\_windows.go: registry key HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run
* startup\_macos.go: LaunchAgent plist at \~/Library/LaunchAgents/com.scrubadubber.hub.plist
Registers the tray app to start on login. Enabled by default, toggleable in Settings.

### 4\. Settings Window (internal/settings/)

A minimal native window (use fyne.io/fyne/v2 for cross-platform native UI, or
embed a small HTML/JS page in a webview — evaluate and justify):

* Hub URL (default: http://127.0.0.1:8383) — for enterprise pointing at shared Hub
* Scrubbing mode: mask | redact | off
* Start on login toggle
* Open config file button (opens Hub config.yaml in default editor)
* Protected tools checklist (Claude Code ✓, more coming)

### 5\. Installer / Updater (cmd/installer/)

**Windows**: NSIS or a pure-Go installer that:

* Requests elevation once (expected for system-level install)
* Downloads Hub binary from scrubadubber-hub GitHub Releases
* Downloads scrub-claude + scrub-setup from bridge-claude-code releases
* Places binaries in %LOCALAPPDATA%\\scrubadubber\\bin\\
* Adds that directory to user PATH (no system PATH — no admin needed after install)
* Runs scrub-setup --yes to configure shell profile
* Launches the tray app
* Creates Start Menu shortcut + uninstaller

**macOS**: .pkg or .dmg that:

* Downloads Hub binary (darwin/amd64 or arm64 auto-detected)
* Downloads scrub-claude + scrub-setup binaries
* Places in \~/Library/Application Support/scrubadubber/bin/
* Adds to PATH via \~/.zshrc (most Mac developers use zsh)
* Runs scrub-setup --yes
* Launches menubar app
* Code-signed with an ad-hoc signature (full notarization as a later upgrade)

### 6\. Auto-Updater (internal/updater/)

* Polls GitHub Releases API on startup and daily
* Compares current version against latest tag
* Shows "Update available" in tray menu
* Downloads + verifies SHA256 checksum before applying
* Restarts itself after update

## Binary Download URLs (Hub contract)

The installer fetches Hub binaries from the PUBLIC scrubadubber-hub-releases
mirror (the source repo scrubadubber-hub stays private; its release workflow
cross-publishes the binaries + SHA256SUMS + config.example.yaml to the mirror).
These URLs are the ONLY coupling between this repo and the Hub:

```
https://github.com/salehkreiner/scrubadubber-hub-releases/releases/download/{tag}/hub\_{os}\_{arch}\[.exe]
```

Bridge binaries from public releases:

```
https://github.com/salehkreiner/bridge-claude-code/releases/download/{tag}/scrub-claude\_{os}\_{arch}\[.exe]
https://github.com/salehkreiner/bridge-claude-code/releases/download/{tag}/scrub-setup\_{os}\_{arch}\[.exe]
```

## Key Design Principles

* Zero terminals after install — if a user ever needs to open a terminal for
normal operation, that is a bug
* Elevation once, never again — installer requests admin, tray app never does
* Fail visibly — red icon is better than silent failure
* Works on both platforms from day one — no "Windows only for now"
* Enterprise ready — HUB\_URL in settings lets teams point at a shared Hub

## Dependencies (justify each — this is a trust repo)

* github.com/getlantern/systray OR fyne.io/systray — tray icon (evaluate both)
* fyne.io/fyne/v2 — settings window (native widgets, no webview dependency)
OR a lightweight webview — evaluate and justify
* Standard library for everything else

## Directory Structure

```
scrubadubber/
├── cmd/
│   ├── scrubadubber/
│   │   └── main.go              # tray app entrypoint
│   └── installer/
│       └── main.go              # standalone installer binary
├── internal/
│   ├── hubmanager/
│   │   ├── manager.go           # Hub process lifecycle
│   │   └── manager\_test.go
│   ├── startup/
│   │   ├── startup\_windows.go   # registry-based startup
│   │   └── startup\_macos.go     # LaunchAgent plist
│   ├── settings/
│   │   ├── settings.go          # settings window + persistence
│   │   └── settings\_test.go
│   ├── updater/
│   │   ├── updater.go           # GitHub Releases polling + download
│   │   └── updater\_test.go
│   └── config/
│       └── config.go            # app config (install paths, URLs, defaults)
├── assets/
│   ├── icon\_green.png           # 32x32 tray icons (all states)
│   ├── icon\_red.png
│   ├── icon\_yellow.png
│   └── icon\_grey.png
├── .github/workflows/
│   ├── ci.yml                   # build + test on windows + macos
│   └── release.yml              # tag → build installers → publish
├── go.mod
├── Makefile
└── README.md
```

## Build Order (once plan approved)

1. internal/config + assets (icons)
2. internal/hubmanager (process lifecycle + health check loop)
3. internal/startup (Windows + macOS)
4. cmd/scrubadubber (tray app, all icon states, menu)
5. internal/settings (settings window)
6. internal/updater
7. cmd/installer (Windows .exe installer + macOS .dmg)
8. CI + Release workflows
9. README

## Hub Binary Release Requirement

COMPLETE — scrubadubber-hub v0.1.3 is published with all 5 platform binaries

(linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64) plus

SHA256SUMS. All Hub binary download URLs are live and ready for the installer.Notes from Smoke Test

* Use PowerShell, not cmd.exe, in all user-facing documentation and instructions
* scrub-setup elevation fix (UAC manifest) is in bridge-claude-code v0.1.1
* Hub starts with: go run ./cmd/hub serve (binary install needed for production)
* Traffic confirmed flowing through Hub end-to-end on Windows

