//go:build darwin

package startup

import (
	"strings"
	"testing"
)

func TestPlistContents(t *testing.T) {
	d := &darwinManager{
		execPath: "/Applications/Scrubadubber.app/Contents/MacOS/scrubadubber",
		args:     []string{"--startup"},
	}
	out := d.plistContents()
	for _, want := range []string{
		"<string>com.scrubadubber.app</string>",
		"<string>/Applications/Scrubadubber.app/Contents/MacOS/scrubadubber</string>",
		"<string>--startup</string>",
		"<key>RunAtLoad</key>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("plist missing %q\n---\n%s", want, out)
		}
	}
}

func TestXMLEscape(t *testing.T) {
	if got := xmlEscape("a&b<c>d"); got != "a&amp;b&lt;c&gt;d" {
		t.Errorf("xmlEscape = %q", got)
	}
}
