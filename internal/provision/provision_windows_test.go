//go:build windows

package provision

import "testing"

func TestPathHas(t *testing.T) {
	list := `C:\Windows;C:\Users\me\AppData\Local\scrubadubber\bin;C:\foo`
	if !pathHas(list, `c:\users\me\appdata\local\scrubadubber\bin`) {
		t.Error("expected case-insensitive match")
	}
	if pathHas(list, `C:\bar`) {
		t.Error("unexpected match for absent dir")
	}
	if pathHas("", `C:\x`) {
		t.Error("empty PATH should not match")
	}
}

func TestOrLatest(t *testing.T) {
	if orLatest("") != "latest" {
		t.Error(`orLatest("") should be "latest"`)
	}
	if orLatest("v1.2.3") != "v1.2.3" {
		t.Error("orLatest should pass through explicit tags")
	}
}
