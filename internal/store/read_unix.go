//go:build !windows

package store

import (
	"os"
	"syscall"
)

// ReadFileMmap memory-maps path read-only. The returned slice shares the kernel
// page cache, so it avoids the extra copy that os.ReadFile performs — a direct,
// measurable win for inputs that are already cached (the common case when refining
// the same wordlist repeatedly). The release function must be called once the buffer
// is no longer referenced; it is safe to call after the spans built from the buffer
// have been consumed.
//
// The empty file is handled explicitly because mmap rejects a zero-length mapping;
// we return a zero-length slice and a no-op release so callers need no special case.
//
// This Unix implementation uses syscall.Mmap, which is unavailable on Windows; that
// platform uses the memory-copy fallback in read_windows.go.
func ReadFileMmap(path string) (data []byte, release func() error, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	// The mmap keeps the data alive after the fd is closed, so closing here is safe and the
	// error is non-actionable.
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	size := fi.Size()
	if size == 0 {
		return []byte{}, func() error { return nil }, nil
	}
	m, err := syscall.Mmap(int(f.Fd()), 0, int(size), syscall.PROT_READ, syscall.MAP_PRIVATE)
	if err != nil {
		return nil, nil, err
	}
	return m, func() error { return syscall.Munmap(m) }, nil
}
