package updater

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// SwapBinary atomically replaces dst with src, keeping the previous dst as
// dst+".old". This works while dst is the currently-running executable:
// Windows forbids deleting a running .exe but allows renaming it, and Unix
// replaces the inode while the running process keeps its open file. The ".old"
// backup is cleaned up on the next launch (see CleanupOldBinary).
//
// src and dst should live on the same volume so the renames are atomic.
func SwapBinary(dst, src string) (backup string, err error) {
	backup = dst + ".old"
	_ = os.Remove(backup) // discard any stale backup

	if _, statErr := os.Stat(dst); statErr == nil {
		if err := os.Rename(dst, backup); err != nil {
			return "", fmt.Errorf("back up current binary: %w", err)
		}
	}
	if err := os.Rename(src, dst); err != nil {
		// Roll back so we never leave dst missing.
		_ = os.Rename(backup, dst)
		return "", fmt.Errorf("install new binary: %w", err)
	}
	return backup, nil
}

// CleanupOldBinary removes a leftover ".old" backup next to path. Call it on
// startup; errors are ignored (the file may briefly remain locked).
func CleanupOldBinary(path string) {
	_ = os.Remove(path + ".old")
}

// UpdateBinary downloads the named asset from rel into destPath (verifying its
// checksum when the release publishes one) and atomically swaps it into place.
// Used for the app's own binary as well as the Hub/bridge binaries.
func (c *Client) UpdateBinary(ctx context.Context, rel Release, assetName, destPath string) error {
	asset, ok := rel.Asset(assetName)
	if !ok {
		return fmt.Errorf("release %s has no asset %q", rel.TagName, assetName)
	}
	sums, err := c.Checksums(ctx, rel)
	if err != nil {
		return err
	}

	tmp := destPath + ".new"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	dlErr := DownloadAndVerify(ctx, c.httpClient(), asset.URL, sums[assetName], f)
	closeErr := f.Close()
	if dlErr != nil {
		_ = os.Remove(tmp)
		return dlErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	_ = os.Chmod(tmp, 0o755) // ensure the executable bit on Unix

	if _, err := SwapBinary(destPath, tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// SelfUpdate replaces the running executable with assetName from rel. The
// caller should Relaunch and then exit. (On macOS this replaces the binary
// inside the .app bundle; relaunching that binary is sufficient for a menubar
// agent.)
func (c *Client) SelfUpdate(ctx context.Context, rel Release, assetName string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return c.UpdateBinary(ctx, rel, assetName, exe)
}

// Relaunch starts a fresh copy of the current executable with the same
// arguments. The caller should exit immediately afterwards.
func Relaunch() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Start()
}
