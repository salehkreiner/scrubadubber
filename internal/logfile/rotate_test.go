package logfile

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRotationKeepsBoundedFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hub.log")

	w, err := Open(path, 100, 3) // rotate at 100 bytes, keep 3 files
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	chunk := bytes.Repeat([]byte("a"), 60)
	for i := 0; i < 12; i++ {
		if _, err := w.Write(chunk); err != nil {
			t.Fatalf("Write %d: %v", i, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	mustExist := []string{path, path + ".1", path + ".2"}
	for _, p := range mustExist {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s to exist: %v", filepath.Base(p), err)
		}
	}
	// Only maxFiles (3) should be kept; the 4th must never appear.
	if _, err := os.Stat(path + ".3"); !os.IsNotExist(err) {
		t.Errorf("expected %s.3 to NOT exist", filepath.Base(path))
	}
}

func TestReopenAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hub.log")

	w, _ := Open(path, 1<<20, 3)
	_, _ = w.Write([]byte("first\n"))
	_ = w.Close()

	w2, _ := Open(path, 1<<20, 3)
	_, _ = w2.Write([]byte("second\n"))
	_ = w2.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got := string(data); got != "first\nsecond\n" {
		t.Errorf("expected appended content, got %q", got)
	}
}
