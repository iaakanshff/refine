// Package engine wires the pure core (dedup), the store (I/O) and the report (presentation)
// layers together into the run modes: stdin, single/multi-file, and wildcard. It is the only
// package that knows about all three at once; keeping that orchestration in one place lets main
// stay a thin wiring layer.
package engine

import (
	"os"
	"time"

	"github.com/yourpwnguy/refine/internal/cli"
	"github.com/yourpwnguy/refine/internal/report"
	"github.com/yourpwnguy/refine/internal/term"
)

// Run executes refine according to cfg and returns an aggregated Summary.
//
// Progress is drawn to stderr and is enabled only for interactive terminals, so redirected or
// logged output stays clean (and never interleaves escape codes with piped data). stdout is
// reserved for the deduped line data in pipe mode.
func Run(cfg cli.Config) (*report.Summary, error) {
	progressOn := !cfg.Quiet && !cfg.JSON && term.IsTerminal(os.Stderr)
	progress := report.NewProgress(os.Stderr, progressOn)
	defer progress.Done()

	start := time.Now()
	var (
		results []report.Result
		err     error
	)
	switch {
	case cfg.IsStdin:
		results, err = runStdin(cfg, progress)
	case cfg.WildcardDir != "":
		results, err = runWildcard(cfg, progress)
	default:
		if len(cfg.Inputs) == 1 {
			results, err = runFiles(cfg, progress)
		} else {
			// Two or more inputs: merge them into one sorted, deduplicated output. Each input is
			// deduplicated independently (and those deduplications run in parallel across the worker
			// pool), then the sorted results are k-way merged. This parallelizes the sort work across
			// files instead of concatenating everything into one giant buffer and sorting it serially.
			results, err = runMerge(cfg, progress)
		}
	}
	if err != nil {
		return nil, err
	}

	summary := &report.Summary{Results: results, Duration: time.Since(start)}
	for _, r := range results {
		summary.FilesProcessed++
		summary.TotalLines += r.TotalLines
		summary.UniqueLines += r.UniqueLines
		summary.DuplicateLines += r.DuplicateLines
		summary.BytesWritten += r.BytesWritten
	}
	return summary, nil
}
