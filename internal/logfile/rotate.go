// Package logfile provides a minimal, size-based rotating file writer. It lets
// us capture Hub stdout/stderr to a bounded set of log files without taking on
// an external logging dependency (this is a trust repo — stdlib only here).
package logfile

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	// DefaultMaxSize is the per-file rotation threshold (5 MiB).
	DefaultMaxSize = 5 << 20
	// DefaultMaxFiles is how many files to keep (current + N-1 rotated).
	DefaultMaxFiles = 3
)

// Writer is an io.WriteCloser that rotates the backing file once it would
// exceed maxSize. Rotation keeps maxFiles files total: path, path.1 … path.(N-1).
// It is safe for concurrent use (os/exec writes stdout and stderr from separate
// goroutines).
type Writer struct {
	path     string
	maxSize  int64
	maxFiles int

	mu   sync.Mutex
	f    *os.File
	size int64
}

// Open creates (or appends to) the log file at path. maxSize/maxFiles default
// to DefaultMaxSize/DefaultMaxFiles when non-positive.
func Open(path string, maxSize int64, maxFiles int) (*Writer, error) {
	if maxSize <= 0 {
		maxSize = DefaultMaxSize
	}
	if maxFiles <= 0 {
		maxFiles = DefaultMaxFiles
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	w := &Writer{path: path, maxSize: maxSize, maxFiles: maxFiles}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.openLocked(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Writer) openLocked() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	w.f = f
	w.size = info.Size()
	return nil
}

// Write appends p, rotating first if the file would exceed maxSize.
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		if err := w.openLocked(); err != nil {
			return 0, err
		}
	}
	if w.size > 0 && w.size+int64(len(p)) > w.maxSize {
		if err := w.rotateLocked(); err != nil {
			return 0, err
		}
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

// rotateLocked shifts path→path.1→…→path.(N-1), dropping the oldest, then opens
// a fresh empty path.
func (w *Writer) rotateLocked() error {
	if w.f != nil {
		_ = w.f.Close()
		w.f = nil
	}
	// Drop the oldest rotated file.
	_ = os.Remove(fmt.Sprintf("%s.%d", w.path, w.maxFiles-1))
	// Shift the rest up by one (high index first).
	for i := w.maxFiles - 1; i >= 1; i-- {
		src := w.path
		if i > 1 {
			src = fmt.Sprintf("%s.%d", w.path, i-1)
		}
		dst := fmt.Sprintf("%s.%d", w.path, i)
		_ = os.Rename(src, dst) // ignore: source may not exist yet
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	w.f = f
	w.size = 0
	return nil
}

// Close closes the backing file.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}
