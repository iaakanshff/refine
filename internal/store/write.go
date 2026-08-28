package store

import (
	"io"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/yourpwnguy/refine/internal/core"
)

// WriteSpans streams the given spans (referencing buf) to w, one line per span
// separated by '\n'. The whole output is assembled in a single contiguous buffer and
// written with one Write call.
//
// Why one buffer + one write: the old implementation built the entire output as a
// strings.Join, doubling peak memory; earlier still we wrote line-by-line, which made
// syscalls dominate at 14M lines. A single contiguous buffer keeps peak memory at ~1x
// the output size and amortizes the syscall cost to one call.
//
// The per-line copy is parallelized across cores: each worker fills its own block of
// the output using a block-local prefix sum, so we avoid both the serial per-line
// append overhead (which dominated at 14M lines) and a single giant prefix-sum pass
// that would leave most cores idle.
//
// If bytesWritten is non-nil it is incremented atomically by the number of bytes
// emitted, letting the live progress display sample throughput without the writer ever
// blocking on a lock or a UI tick.
func WriteSpans(buf []byte, spans []core.Span, w io.Writer, bytesWritten *int64) error {
	n := len(spans)
	if n == 0 {
		_, err := w.Write(nil)
		return err
	}
	workers := runtime.GOMAXPROCS(0)
	if workers > n {
		workers = n
	}
	if workers < 1 {
		workers = 1
	}
	// sizes holds each span's emitted size in bytes (line length + 1 for the trailing
	// newline). It is phase-1 scratch used only to sum per-block totals.
	sizes := make([]int, n)
	// block is the number of spans assigned to each worker (ceil(n/workers)); the last
	// block may be shorter.
	block := (n + workers - 1) / workers
	// blockBase[c] is the starting offset of block c inside out, filled in by the
	// prefix-sum in phase 1.
	blockBase := make([]int, workers)
	var wg sync.WaitGroup

	// Phase 1: per-block local prefix sums + per-block totals.
	for c := 0; c < workers; c++ {
		start, end := c*block, c*block+block
		if end > n {
			end = n
		}
		if start >= end {
			blockBase[c] = 0
			continue
		}
		wg.Add(1)
		go func(c, start, end int) {
			defer wg.Done()
			off := 0
			for k := start; k < end; k++ {
				sz := int(spans[k].Len) + 1
				sizes[k] = sz
				off += sz
			}
			blockBase[c] = off
		}(c, start, end)
	}
	wg.Wait()

	// Prefix-sum the block totals into absolute block bases.
	base := 0
	for c := 0; c < workers; c++ {
		t := blockBase[c]
		blockBase[c] = base
		base += t
	}
	// out is the single contiguous output buffer; every worker writes only into its own
	// precomputed region, so no locking is needed during the copy.
	out := make([]byte, base)
	nl := byte('\n') // separator appended after every line

	// Phase 2: parallel copy of each block into its region of out.
	for c := 0; c < workers; c++ {
		start, end := c*block, c*block+block
		if end > n {
			end = n
		}
		if start >= end {
			continue
		}
		wg.Add(1)
		go func(c, start, end int) {
			defer wg.Done()
			off := blockBase[c]
			for k := start; k < end; k++ {
				line := spans[k].Bytes(buf)
				copy(out[off:], line)
				off += len(line)
				out[off] = nl
				off++
			}
		}(c, start, end)
	}
	wg.Wait()

	if bytesWritten != nil {
		atomic.AddInt64(bytesWritten, int64(len(out)))
	}
	_, err := w.Write(out)
	return err
}
