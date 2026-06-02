// Package bridgecheck reports (and can undo) whether the bridge has configured
// the shell to route LLM traffic through the Hub.
//
// scrub-setup writes a marked block to the user's shell profile (setting
// SCRUBADUBBER_HUB_URL and defining a `claude` wrapper). We inspect that block
// rather than the live environment, because a tray/menubar agent doesn't
// inherit interactive-shell variables. The candidate profile paths are
// platform-specific (see bridgecheck_windows.go / bridgecheck_darwin.go).
package bridgecheck

import (
	"os"
	"strings"
)

const (
	// marker is the variable scrub-setup exports in the shell profile.
	marker = "SCRUBADUBBER_HUB_URL"
	// blockStart / blockEnd delimit the block scrub-setup writes.
	blockStart = "# >>> scrubadubber bridge >>>"
	blockEnd   = "# <<< scrubadubber bridge <<<"
)

// Configured reports whether any of the platform's shell profiles carry the
// bridge configuration.
func Configured() bool {
	for _, p := range profilePaths() {
		if data, err := os.ReadFile(p); err == nil && strings.Contains(string(data), marker) {
			return true
		}
	}
	return false
}

// RemoveProfileBlock strips the scrub-setup block from every profile that has
// it. Uninstall calls this because scrub-setup --uninstall doesn't reliably
// remove the block itself.
func RemoveProfileBlock() error {
	var firstErr error
	for _, p := range profilePaths() {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		stripped, changed := stripBlock(string(data))
		if !changed {
			continue
		}
		if err := os.WriteFile(p, []byte(stripped), 0o644); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// stripBlock removes the inclusive blockStart..blockEnd region(s), plus any
// stray marker line, returning the new content and whether anything changed.
func stripBlock(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	inBlock, changed := false, false
	for _, ln := range lines {
		switch t := strings.TrimSpace(ln); {
		case t == blockStart:
			inBlock, changed = true, true
		case t == blockEnd:
			inBlock = false
		case inBlock:
			// drop lines inside the block
		case strings.Contains(ln, marker):
			changed = true // defensive: stray marker outside a block
		default:
			out = append(out, ln)
		}
	}
	return strings.Join(out, "\n"), changed
}
