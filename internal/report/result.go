// Package report turns processing results into output: a polished human view on
// stderr (keeping stdout free for piped data) and a machine-readable JSON view for
// scripts. It also owns the live progress display.
//
// The package deliberately knows nothing about how the data was produced; it receives a
// Summary (built by the engine) and renders it. This keeps presentation fully isolated
// from dedup and I/O logic, so the UI can change without touching the pipeline.
package report

import "time"

// Result is the outcome of deduplicating a single input (a file or stdin).
type Result struct {
	Target         string        // base name of the file (or "stdin") that was processed
	TotalLines     int64         // lines read, before deduplication
	UniqueLines    int64         // lines written, after deduplication
	DuplicateLines int64         // TotalLines - UniqueLines
	BytesWritten   int64         // bytes emitted to the destination
	Duration       time.Duration // wall time spent on this input
}

// Summary aggregates every Result plus run-wide totals. Duration is the whole-run
// wall clock (not a sum of per-file durations, which would double-count parallel work).
type Summary struct {
	Results        []Result      // one per processed input
	Duration       time.Duration // total run wall time
	FilesProcessed int           // number of inputs that produced a result
	TotalLines     int64         // sum of per-result TotalLines
	UniqueLines    int64         // sum of per-result UniqueLines
	DuplicateLines int64         // sum of per-result DuplicateLines
	BytesWritten   int64         // sum of per-result BytesWritten
}
