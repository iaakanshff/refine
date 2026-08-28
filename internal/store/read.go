// Package store owns all byte movement: reading inputs, writing outputs
// atomically, and listing directories for wildcard mode. It has no knowledge of
// dedup logic or presentation, which keeps the I/O concerns isolated and easy to
// reason about.
//
// The core idea is that the dedup pipeline works on a single contiguous []byte per
// input; store is responsible for getting bytes in (mmap or stdin) and getting
// bytes out (a single buffered write) without ever materializing more than one copy
// beyond what the OS already caches.
package store

import (
	"io"
	"os"
)

// ReadStdin reads everything piped on standard input. Unlike files we cannot seek or
// know the size up front, but io.ReadAll still yields one contiguous buffer suitable
// for the core package.
func ReadStdin() ([]byte, error) {
	return io.ReadAll(os.Stdin)
}
