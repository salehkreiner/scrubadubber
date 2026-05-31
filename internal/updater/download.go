package updater

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/salehkreiner/scrubadubber/internal/config"
)

// DownloadAndVerify streams the URL into dst while computing its SHA256. If
// expectedSHA256 is non-empty and does not match, it returns an error (and the
// caller must discard whatever was written).
func DownloadAndVerify(ctx context.Context, client *http.Client, url, expectedSHA256 string, dst io.Writer) error {
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
		return fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(dst, h), resp.Body); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if expectedSHA256 != "" && !strings.EqualFold(got, expectedSHA256) {
		return fmt.Errorf("checksum mismatch: got %s, want %s", got, expectedSHA256)
	}
	return nil
}

// ParseChecksums parses "sha256␠␠filename" lines (the standard sha256sum
// format) into a filename→sha map (sha lowercased).
func ParseChecksums(data []byte) map[string]string {
	out := make(map[string]string)
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[len(fields)-1], "*") // sha256sum binary marker
		if i := strings.LastIndexAny(name, `/\`); i >= 0 {
			name = name[i+1:] // tolerate dir-prefixed entries; key by base name
		}
		out[name] = strings.ToLower(fields[0])
	}
	return out
}

// findChecksumAsset returns the first release asset whose name matches one of
// the recognized checksum filenames (case-insensitive).
func findChecksumAsset(rel Release) (Asset, bool) {
	for _, name := range config.ChecksumAssetNames {
		for _, a := range rel.Assets {
			if strings.EqualFold(a.Name, name) {
				return a, true
			}
		}
	}
	return Asset{}, false
}

// Checksums downloads and parses the release's checksums file. A release
// without one yields an empty map (downloads then proceed unverified — the
// caller decides whether that's acceptable).
func (c *Client) Checksums(ctx context.Context, rel Release) (map[string]string, error) {
	a, ok := findChecksumAsset(rel)
	if !ok {
		return map[string]string{}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download checksums: status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	return ParseChecksums(data), nil
}
