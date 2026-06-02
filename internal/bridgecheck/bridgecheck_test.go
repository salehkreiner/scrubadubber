package bridgecheck

import (
	"strings"
	"testing"
)

func TestStripBlockRemovesMarkedRegion(t *testing.T) {
	in := "echo before\n" +
		blockStart + "\n" +
		`$env:SCRUBADUBBER_HUB_URL = "http://127.0.0.1:8383"` + "\n" +
		"function claude { scrub-claude @args }\n" +
		blockEnd + "\n" +
		"echo after\n"

	out, changed := stripBlock(in)
	if !changed {
		t.Fatal("expected changed=true")
	}
	if strings.Contains(out, marker) || strings.Contains(out, blockStart) || strings.Contains(out, blockEnd) {
		t.Errorf("block not fully removed:\n%q", out)
	}
	if !strings.Contains(out, "echo before") || !strings.Contains(out, "echo after") {
		t.Errorf("surrounding content was lost:\n%q", out)
	}
}

func TestStripBlockNoOpWhenAbsent(t *testing.T) {
	in := "echo hi\nSet-Alias foo bar\n"
	out, changed := stripBlock(in)
	if changed || out != in {
		t.Errorf("expected no change; changed=%v out=%q", changed, out)
	}
}
