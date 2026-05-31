// Package updater checks GitHub Releases for newer versions of the app (and of
// the managed Hub/bridge binaries), downloads them, verifies their SHA256
// checksum, and swaps them into place.
//
// Three things can be updated, kept distinct: the tray app itself
// (self-replace + relaunch), the Hub binary, and the bridge binaries.
package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/mod/semver"

	"github.com/salehkreiner/scrubadubber/internal/config"
)

const defaultAPIBase = "https://api.github.com"

// Asset is one downloadable file attached to a release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// Release is the subset of the GitHub Releases API we use.
type Release struct {
	TagName string  `json:"tag_name"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

// Asset returns the named asset, if present.
func (r Release) Asset(name string) (Asset, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

// Client talks to the GitHub Releases API. The zero value is usable; APIBase
// and HTTP can be overridden (tests point APIBase at a fake server).
type Client struct {
	HTTP    *http.Client
	APIBase string
}

// NewClient returns a Client with sensible defaults.
func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 30 * time.Second}, APIBase: defaultAPIBase}
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *Client) apiBase() string {
	if c.APIBase != "" {
		return c.APIBase
	}
	return defaultAPIBase
}

func (c *Client) getRelease(ctx context.Context, url string) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("github release %s: status %d", url, resp.StatusCode)
	}
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return Release{}, err
	}
	return rel, nil
}

// LatestRelease fetches the most recent published release for owner/repo.
func (c *Client) LatestRelease(ctx context.Context, owner, repo string) (Release, error) {
	return c.getRelease(ctx, fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.apiBase(), owner, repo))
}

// ReleaseByTag fetches a specific release by its tag.
func (c *Client) ReleaseByTag(ctx context.Context, owner, repo, tag string) (Release, error) {
	return c.getRelease(ctx, fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", c.apiBase(), owner, repo, tag))
}

// Resolve returns the latest release when version is "" or "latest", otherwise
// the release for that exact tag.
func (c *Client) Resolve(ctx context.Context, owner, repo, version string) (Release, error) {
	if version == "" || version == "latest" {
		return c.LatestRelease(ctx, owner, repo)
	}
	return c.ReleaseByTag(ctx, owner, repo, version)
}

// Update describes the result of an app version check.
type Update struct {
	Available bool
	Version   string
	Release   Release
}

// CheckApp checks whether a newer version of the app has been released.
func (c *Client) CheckApp(ctx context.Context, currentVersion string) (Update, error) {
	rel, err := c.LatestRelease(ctx, config.GitHubOwner, config.AppRepo)
	if err != nil {
		return Update{}, err
	}
	return Update{
		Available: IsNewer(currentVersion, rel.TagName),
		Version:   rel.TagName,
		Release:   rel,
	}, nil
}

func ensureV(v string) string {
	if v == "" || v[0] == 'v' {
		return v
	}
	return "v" + v
}

// IsNewer reports whether latest is a strictly greater semantic version than
// current. Invalid versions (e.g. the "dev" build marker) yield false, so dev
// builds never nag about updates.
func IsNewer(current, latest string) bool {
	cur, lat := ensureV(current), ensureV(latest)
	if !semver.IsValid(cur) || !semver.IsValid(lat) {
		return false
	}
	return semver.Compare(lat, cur) > 0
}

// Poller checks for app updates on startup and then on a fixed interval,
// invoking OnUpdate whenever a newer version is available.
type Poller struct {
	Client   *Client
	Current  string
	Interval time.Duration // default 24h
	OnUpdate func(Update)
}

// Run starts the polling loop in a goroutine; it stops when ctx is cancelled.
func (p *Poller) Run(ctx context.Context) {
	interval := p.Interval
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	check := func() {
		cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		u, err := p.Client.CheckApp(cctx, p.Current)
		if err == nil && u.Available && p.OnUpdate != nil {
			p.OnUpdate(u)
		}
	}
	go func() {
		check()
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				check()
			}
		}
	}()
}
