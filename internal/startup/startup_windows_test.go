//go:build windows

package startup

import "testing"

func TestCommandRendering(t *testing.T) {
	w := &windowsManager{execPath: `C:\Users\me\AppData\Local\scrubadubber\bin\scrubadubber.exe`, args: []string{"--startup"}}
	got := w.command()
	want := `"C:\Users\me\AppData\Local\scrubadubber\bin\scrubadubber.exe" --startup`
	if got != want {
		t.Errorf("command() = %q, want %q", got, want)
	}
}

func TestCommandNoArgs(t *testing.T) {
	w := &windowsManager{execPath: `C:\app.exe`}
	if got := w.command(); got != `"C:\app.exe"` {
		t.Errorf("command() = %q", got)
	}
}
