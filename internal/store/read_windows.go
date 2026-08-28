//go:build windows

package store

import "os"

// ReadFileMmap on Windows cannot use syscall.Mmap (it is Unix-only), so we fall back to
// reading the whole file into memory. The behavior is identical to the Unix mmap path from
// the caller's point of view (one contiguous []byte), at the cost of an extra in-process copy
// and losing the shared page-cache benefit. Keeping this dependency-free (no x/sys/windows)
// is more important than shaving that copy on a platform the hot path isn't tuned for.
//
// The release function is a no-op because there is no mapping to unmap; the garbage collector
// reclaims the buffer once the spans built from it are no longer referenced.
func ReadFileMmap(path string) (data []byte, release func() error, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	return b, func() error { return nil }, nil
}
