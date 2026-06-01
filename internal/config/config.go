// Package config holds application-wide identifiers, release/network
// coordinates, and helpers that resolve the per-platform data directories used
// to store the managed Hub and bridge binaries, configuration, and logs.
//
// This repo never contains scrubbing logic; it only knows where the Hub and
// bridge binaries live and where to download them from.
package config

import (
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
)

const (
	// AppName is the user-facing product name.
	AppName = "Scrubadubber"
	// AppID is the reverse-DNS identifier (LaunchAgent label, etc.).
	AppID = "com.scrubadubber.app"

	// GitHubOwner owns all three Scrubadubber repositories.
	GitHubOwner = "salehkreiner"
	// AppRepo is this repo; the updater polls it for new app releases.
	AppRepo = "scrubadubber"
	// HubRepo is the PUBLIC release mirror for the Hub binaries. The Hub source
	// stays private (scrubadubber-hub); its CI publishes the compiled binaries +
	// SHA256SUMS (and an example config) here so the installer can fetch them
	// unauthenticated.
	HubRepo = "scrubadubber-hub-releases"
	// BridgeRepo holds the (public) bridge binaries.
	BridgeRepo = "bridge-claude-code"

	// HubPort is where the Hub proxies LLM traffic.
	HubPort = 8383
	// HealthPort is the Hub's health endpoint port.
	HealthPort = 8384
	// HealthPath is the Hub health-check path.
	HealthPath = "/healthz"

	// LockPort is a fixed loopback port used as a single-instance lock for the
	// tray app: binding it fails if another instance is already running.
	LockPort = 8385

	// PinnedHubVersion / PinnedBridgeVersion select which release to download.
	// "latest" resolves to the newest GitHub release at install/update time; a
	// concrete tag pins a known-good build. The Hub is pinned because its public
	// mirror tracks the private source's versioning and /releases/latest skips
	// prereleases.
	PinnedHubVersion    = "v0.1.3"
	PinnedBridgeVersion = "latest"
)

// exeSuffix is ".exe" on Windows and "" elsewhere.
func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// AppBinaryName is the on-disk name of the tray/menubar app for this platform.
func AppBinaryName() string { return "scrubadubber" + exeSuffix() }

// AppAssetName is the release-asset filename for the app binary, e.g.
// "scrubadubber_windows_amd64.exe". The updater downloads this to self-update.
func AppAssetName() string { return assetName("scrubadubber") }

// HubBinaryName is the on-disk name of the Hub binary for this platform.
func HubBinaryName() string { return "hub" + exeSuffix() }

// ChecksumAssetNames lists the checksum-file asset names we recognize across the
// app, Hub, and bridge releases — different repos use different conventions
// (scrubadubber-hub publishes SHA256SUMS; this repo's releases use
// checksums.txt). The updater verifies downloads against the first one present
// in a release.
var ChecksumAssetNames = []string{"SHA256SUMS", "SHA256SUMS.txt", "checksums.txt", "sha256sums.txt"}

// ScrubClaudeBinaryName is the on-disk name of the scrub-claude bridge binary.
func ScrubClaudeBinaryName() string { return "scrub-claude" + exeSuffix() }

// ScrubSetupBinaryName is the on-disk name of the scrub-setup bridge binary.
func ScrubSetupBinaryName() string { return "scrub-setup" + exeSuffix() }

// HubAssetName is the Hub's release-asset filename, e.g. "hub_windows_amd64.exe".
func HubAssetName() string { return assetName("hub") }

// ScrubClaudeAssetName is the scrub-claude release-asset filename.
func ScrubClaudeAssetName() string { return assetName("scrub-claude") }

// ScrubSetupAssetName is the scrub-setup release-asset filename.
func ScrubSetupAssetName() string { return assetName("scrub-setup") }

const releaseDownloadBase = "https://github.com/%s/%s/releases/download/%s/%s"

// assetName builds the release asset filename, e.g. "hub_windows_amd64.exe".
func assetName(prefix string) string {
	return fmt.Sprintf("%s_%s_%s%s", prefix, runtime.GOOS, runtime.GOARCH, exeSuffix())
}

// HubDownloadURL returns the Hub binary download URL for a concrete release tag.
func HubDownloadURL(tag string) string {
	return fmt.Sprintf(releaseDownloadBase, GitHubOwner, HubRepo, tag, assetName("hub"))
}

// ScrubClaudeDownloadURL returns the scrub-claude download URL for a tag.
func ScrubClaudeDownloadURL(tag string) string {
	return fmt.Sprintf(releaseDownloadBase, GitHubOwner, BridgeRepo, tag, assetName("scrub-claude"))
}

// ScrubSetupDownloadURL returns the scrub-setup download URL for a tag.
func ScrubSetupDownloadURL(tag string) string {
	return fmt.Sprintf(releaseDownloadBase, GitHubOwner, BridgeRepo, tag, assetName("scrub-setup"))
}

// HubConfigAssetNames are the candidate filenames for the Hub's example config
// published as a release asset. The Hub is launched as `hub serve -config
// <path>`; on first install the installer seeds that file from the first of
// these present in the Hub release. (The public mirror is releases-only, so the
// config ships as a release asset rather than coming from a source tree.)
var HubConfigAssetNames = []string{"config.example.yaml", "config.yaml"}

// DefaultHubURL is the locally-managed Hub's traffic endpoint.
func DefaultHubURL() string { return fmt.Sprintf("http://127.0.0.1:%d", HubPort) }

// DefaultHealthURL is the locally-managed Hub's health endpoint.
func DefaultHealthURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", HealthPort, HealthPath)
}

// HealthURLFor derives the Hub health endpoint from a configured Hub URL by
// swapping in the health port. It falls back to the default local health URL
// for an unparseable input.
func HealthURLFor(hubURL string) string {
	u, err := url.Parse(hubURL)
	if err != nil || u.Hostname() == "" {
		return DefaultHealthURL()
	}
	return fmt.Sprintf("http://%s:%d%s", u.Hostname(), HealthPort, HealthPath)
}

// --- per-platform path helpers --------------------------------------------
// dataDir() is implemented in paths_windows.go and paths_darwin.go.

// DataDir is the root data directory for the app on this platform.
func DataDir() (string, error) { return dataDir() }

func sub(name string) (string, error) {
	d, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, name), nil
}

// BinDir holds the managed binaries (hub, scrub-claude, scrub-setup).
func BinDir() (string, error) { return sub("bin") }

// ConfigDir holds app + Hub configuration files.
func ConfigDir() (string, error) { return sub("config") }

// LogDir holds rotated log files.
func LogDir() (string, error) { return sub("logs") }

func binPath(name string) (string, error) {
	b, err := BinDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(b, name), nil
}

// HubPath is the absolute path to the managed Hub binary.
func HubPath() (string, error) { return binPath(HubBinaryName()) }

// ScrubClaudePath is the absolute path to the managed scrub-claude binary.
func ScrubClaudePath() (string, error) { return binPath(ScrubClaudeBinaryName()) }

// ScrubSetupPath is the absolute path to the managed scrub-setup binary.
func ScrubSetupPath() (string, error) { return binPath(ScrubSetupBinaryName()) }

// SettingsPath is the absolute path to the app's settings.json.
func SettingsPath() (string, error) {
	c, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(c, "settings.json"), nil
}

// HubConfigPath is the local Hub config file. The Hub is launched with
// `hub serve -config <this path>`; on first install it is seeded from the Hub
// release's example-config asset (see HubConfigAssetNames).
func HubConfigPath() (string, error) {
	c, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(c, "config.yaml"), nil
}

// HubLogPath is the absolute path to the rotating Hub log file.
func HubLogPath() (string, error) {
	l, err := LogDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(l, "hub.log"), nil
}
