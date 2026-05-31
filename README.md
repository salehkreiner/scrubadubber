# Scrubadubber

A native **system-tray app (Windows)** and **menubar app (macOS)** that silently
installs and supervises the Scrubadubber **Hub** and **bridge** in the
background — giving developers one-click, zero-friction protection for their LLM
API traffic. You type `claude` exactly as before; nothing changes in your
workflow.

This repo is the **user-facing product**. It downloads, installs, and manages the
Hub and bridge binaries — it contains **no scrubbing logic** itself.

```
scrubadubber (this repo — public)
    ├── downloads + manages → scrubadubber-hub binary   (private releases)
    └── downloads + manages → bridge-claude-code binary (public releases)
```

## Install

> **Note:** end-to-end install requires published Hub/bridge release binaries.
> See [Status](#status).

### Windows
1. Download **`scrubadubber-setup.exe`** from the latest [release](https://github.com/salehkreiner/scrubadubber/releases).
2. Double-click it. It installs per-user — **no admin prompt** — into
   `%LOCALAPPDATA%\scrubadubber\`, adds that folder to your PATH, configures your
   shell, and launches the tray app.
3. Open a **new PowerShell window** and run `claude` as usual.

### macOS
1. Download **`scrubadubber.dmg`**, open it, and drag **Scrubadubber** to
   Applications.
2. Launch Scrubadubber. On first run it provisions the Hub/bridge into
   `~/Library/Application Support/scrubadubber/`, updates `~/.zshrc`, and adds the
   menubar icon.
3. Open a **new terminal** and run `claude` as usual.

## Using it

A status icon lives in your tray / menubar:

| Icon | Meaning |
|------|---------|
| 🟢 Green | Protected — Hub healthy and the bridge is configured |
| 🟡 Yellow | Hub running but degraded, or the bridge isn't configured yet |
| 🔴 Red | Hub not running / unreachable |
| ⚪ Grey | Starting up |

The menu lets you Start/Stop/Restart the Hub, **View Logs**, open **Settings**,
and **Check for Updates**.

### Settings
**Settings…** opens a small page in your default browser, served locally on
`127.0.0.1` (token-gated, never exposed off-machine):

- **Hub URL** — point at a shared/enterprise Hub
- **Scrubbing mode** — `mask` / `redact` / `off`
- **Start on login** — toggle
- **Open config file** — edit the Hub's `config.yaml`
- **Protected tools** — Claude Code (more coming)

## Build from source

Requires Go 1.26+.

```powershell
# Windows (PowerShell) — the tray app is pure Go, no C compiler needed
go test ./...
go build -o dist\scrubadubber.exe .\cmd\scrubadubber
go build -o dist\scrubadubber-setup.exe .\cmd\installer

# Regenerate the embedded status icons
go run .\assets\gen
```

```bash
# macOS — systray needs cgo (Cocoa); produces a universal .app + .dmg
make mac-dmg VERSION=v1.0.0
```

Releases are built by `.github/workflows/release.yml` on a `v*` tag: Windows
`setup.exe` + tray binary, a universal macOS `.dmg` + per-arch binaries, and a
`checksums.txt`. Downloads are SHA256-verified before they're applied.

## Architecture

| Package | Responsibility |
|---------|----------------|
| `cmd/scrubadubber` | Tray/menubar app — icon states, menu, wiring |
| `cmd/installer` | Windows `setup.exe` (install / `--uninstall`) |
| `internal/hubmanager` | Hub process lifecycle, health loop, auto-restart |
| `internal/provision` | Shared install engine (download + verify + wire up) |
| `internal/settings` | Settings JSON + loopback web UI |
| `internal/startup` | Login registration (HKCU Run key / LaunchAgent) |
| `internal/updater` | GitHub Releases polling, download, verify, self-replace |
| `internal/config` | Paths, URLs, release coordinates, defaults |
| `internal/logfile` | Size-based rotating log writer |
| `internal/bridgecheck` | Detects whether the bridge configured the shell |
| `internal/sysopen` | Open URLs/files with the OS default handler |
| `assets` | Generated status icons (`go run ./assets/gen`) |

### Design principles
- **Zero terminals after install** — needing a terminal for normal use is a bug.
- **No elevation** — everything is per-user; the installer requests no UAC.
- **Fail visibly** — a red icon beats silent failure.
- **Both platforms from day one.**
- **Minimal, justified dependencies** (trust repo): `fyne.io/systray` (tray),
  `golang.org/x/sys` (registry), `golang.org/x/mod` (semver). Everything else is
  the standard library; the settings UI uses no GUI framework.

## Status

This app is complete and unit-tested.

The **Hub contract is confirmed** against `scrubadubber-hub` **v0.1.3** (5 platform
binaries + `SHA256SUMS`):

- binary asset `hub_windows_amd64.exe` (and `hub_{darwin,linux}_{amd64,arm64}`),
- launched as `hub serve -config <path>`,
- health at `GET :8384/healthz` → `{"status":"ok"}` (a non-`ok` status shows as
  degraded/yellow),
- config seeded from the repo's `configs/config.example.yaml` on first install,
- downloads SHA256-verified against the release's `SHA256SUMS`.

Remaining to confirm: the **bridge** (`bridge-claude-code`) release asset names and
`scrub-setup`'s `--yes` / `--uninstall` flags, and how the scrubbing **mode** maps
into the Hub's `config.yaml` (today the mode is persisted in settings and the
Hub's config is edited via **Open config file**).
