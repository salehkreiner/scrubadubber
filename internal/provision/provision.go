// Package provision is the shared, idempotent install engine. It downloads the
// Hub and bridge (and, for the Windows installer, the app binary), verifies
// their checksums, places them under the per-user data dir, wires up PATH and
// the shell profile, writes default settings, and registers start-on-login.
//
// On Windows it is driven by cmd/installer (the setup.exe); on macOS the app
// itself calls Install on first run (the .app is the installer). Platform
// specifics (PATH editing) live in provision_windows.go / provision_darwin.go.
package provision

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/salehkreiner/scrubadubber/internal/config"
	"github.com/salehkreiner/scrubadubber/internal/settings"
	"github.com/salehkreiner/scrubadubber/internal/startup"
	"github.com/salehkreiner/scrubadubber/internal/updater"
)

// Options controls an install/uninstall run.
type Options struct {
	// AppVersion, when non-empty, downloads the app binary into bin (the
	// Windows installer sets this; on macOS the app is already in the .app).
	AppVersion string
	// HubVersion / BridgeVersion are release tags, or "latest"/"" for newest.
	HubVersion    string
	BridgeVersion string
	// StartupTarget is the executable to register for start-on-login ("" skips).
	StartupTarget string
	// LaunchTarget is an executable to launch when done ("" skips).
	LaunchTarget string
	// Log receives human-readable progress (may be nil).
	Log func(string, ...any)
}

func (o Options) logf(format string, args ...any) {
	if o.Log != nil {
		o.Log(format, args...)
	}
}

func orLatest(v string) string {
	if v == "" {
		return "latest"
	}
	return v
}

// Install performs (or repairs) the installation. It is safe to re-run.
func Install(ctx context.Context, opts Options) error {
	binDir, err := config.BinDir()
	if err != nil {
		return err
	}
	configDir, _ := config.ConfigDir()
	logDir, _ := config.LogDir()
	for _, d := range []string{binDir, configDir, logDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", d, err)
		}
	}

	client := updater.NewClient()

	if opts.AppVersion != "" {
		opts.logf("Downloading %s app…", config.AppName)
		if err := downloadBinary(ctx, client, config.AppRepo, opts.AppVersion, config.AppAssetName(), filepath.Join(binDir, config.AppBinaryName())); err != nil {
			return fmt.Errorf("download app: %w", err)
		}
	}

	opts.logf("Downloading Hub…")
	hubRel, err := client.Resolve(ctx, config.GitHubOwner, config.HubRepo, orLatest(opts.HubVersion))
	if err != nil {
		return fmt.Errorf("resolve hub release: %w", err)
	}
	if err := client.UpdateBinary(ctx, hubRel, config.HubAssetName(), filepath.Join(binDir, config.HubBinaryName())); err != nil {
		return fmt.Errorf("download hub: %w", err)
	}
	// Hub releases ship only binaries; seed config.yaml from the repo's example
	// so `hub serve -config <path>` has something to read.
	if cfgPath, err := config.HubConfigPath(); err == nil {
		if _, statErr := os.Stat(cfgPath); os.IsNotExist(statErr) {
			opts.logf("Fetching default Hub config…")
			if err := downloadConfig(ctx, client.HTTP, config.HubExampleConfigURL(hubRel.TagName), cfgPath); err != nil {
				opts.logf("warning: fetch example config: %v", err)
			}
		}
	}

	opts.logf("Downloading bridge…")
	if err := downloadBinary(ctx, client, config.BridgeRepo, orLatest(opts.BridgeVersion), config.ScrubClaudeAssetName(), filepath.Join(binDir, config.ScrubClaudeBinaryName())); err != nil {
		return fmt.Errorf("download scrub-claude: %w", err)
	}
	if err := downloadBinary(ctx, client, config.BridgeRepo, orLatest(opts.BridgeVersion), config.ScrubSetupAssetName(), filepath.Join(binDir, config.ScrubSetupBinaryName())); err != nil {
		return fmt.Errorf("download scrub-setup: %w", err)
	}

	opts.logf("Adding %s to PATH…", binDir)
	if err := addToUserPath(binDir); err != nil {
		opts.logf("warning: could not update PATH: %v", err)
	}

	opts.logf("Configuring shell profile (scrub-setup)…")
	if err := runScrubSetup(ctx, filepath.Join(binDir, config.ScrubSetupBinaryName())); err != nil {
		opts.logf("warning: scrub-setup failed: %v", err)
	}

	if sp, err := config.SettingsPath(); err == nil {
		if _, statErr := os.Stat(sp); os.IsNotExist(statErr) {
			if err := settings.Save(sp, settings.Default()); err != nil {
				opts.logf("warning: write default settings: %v", err)
			}
		}
	}

	if opts.StartupTarget != "" {
		opts.logf("Registering start-on-login…")
		if err := startup.New(opts.StartupTarget, "--startup").Enable(); err != nil {
			opts.logf("warning: startup registration: %v", err)
		}
	}

	if opts.LaunchTarget != "" {
		opts.logf("Launching %s…", config.AppName)
		if err := launch(opts.LaunchTarget); err != nil {
			opts.logf("warning: launch failed: %v", err)
		}
	}

	opts.logf("Done.")
	return nil
}

// Uninstall removes the installation on a best-effort basis. On Windows the
// running process cannot delete its own binary, so bin may partially remain;
// the caller (cmd/installer) handles registry/shortcut cleanup.
func Uninstall(opts Options) error {
	var firstErr error
	note := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	binDir, _ := config.BinDir()

	if p := filepath.Join(binDir, config.ScrubSetupBinaryName()); fileExists(p) {
		_ = exec.Command(p, "--uninstall").Run() // best effort
	}
	if opts.StartupTarget != "" {
		_ = startup.New(opts.StartupTarget).Disable()
	}
	note(removeFromUserPath(binDir))

	if d, err := config.ConfigDir(); err == nil {
		note(os.RemoveAll(d))
	}
	if d, err := config.LogDir(); err == nil {
		note(os.RemoveAll(d))
	}
	note(os.RemoveAll(binDir))
	return firstErr
}

func downloadBinary(ctx context.Context, client *updater.Client, repo, version, asset, dest string) error {
	rel, err := client.Resolve(ctx, config.GitHubOwner, repo, version)
	if err != nil {
		return err
	}
	return client.UpdateBinary(ctx, rel, asset, dest)
}

// downloadConfig fetches a raw config file (e.g. the Hub's example config) to
// dest. It is not checksum-verified — it's a YAML template the user can edit.
func downloadConfig(ctx context.Context, client *http.Client, url, dest string) error {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch config: status %d", resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o644)
}

func runScrubSetup(ctx context.Context, scrubSetupPath string) error {
	if !fileExists(scrubSetupPath) {
		return fmt.Errorf("scrub-setup not found at %s", scrubSetupPath)
	}
	return exec.CommandContext(ctx, scrubSetupPath, "--yes").Run()
}

func launch(exePath string) error {
	return exec.Command(exePath).Start()
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// CopyFile copies src to dst (used by the Windows installer to keep a copy of
// itself for uninstall). The destination is made executable.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
