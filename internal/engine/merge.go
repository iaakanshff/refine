package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/yourpwnguy/refine/internal/cli"
	"github.com/yourpwnguy/refine/internal/core"
	"github.com/yourpwnguy/refine/internal/report"
	"github.com/yourpwnguy/refine/internal/store"
)

// runMerge handles `refine a b ... out`: every input is deduplicated independently (and those
// deduplications run concurrently across the worker pool), then the now-sorted unique results are
// k-way merged into one globally sorted, globally unique output written to cfg.Output.
//
// This is strictly better than concatenating all inputs and sorting once:
//   - the sort work is parallelized across files instead of done serially on a buffer that is the sum
//     of every input (which thrashes the cache and the GC);
//   - the merge is a linear, cache-friendly scan of each already-sorted part.
func runMerge(cfg cli.Config, progress *report.Progress) ([]report.Result, error) {
	t0 := time.Now()
	parts := make([]store.MergedPart, len(cfg.Inputs))
	errs := make([]error, len(cfg.Inputs))

	// sem bounds how many files we scan/dedupe concurrently. Without it we would mmap and
	// sort every input at once, which thrashes the page cache and the scheduler once the file
	// count exceeds the core count. The pool size is the user's --workers budget.
	sem := make(chan struct{}, cfg.Workers)
	var wg sync.WaitGroup
	for i, path := range cfg.Inputs {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, path string) {
			defer wg.Done()
			defer func() { <-sem }()

			label := filepath.Base(path)
			progress.Set("dedupe "+label, 0)
			b, release, rerr := store.ReadFileMmap(path)
			if rerr != nil {
				errs[i] = rerr
				return
			}
			// Each file's scan runs in parallel across a share of the pool; the per-file sort/dedup stays
			// serial but is bounded and merged afterwards via the k-way merge.
			perFile := max(1, cfg.Workers/len(cfg.Inputs))
			spans := core.ExtractSpans(b, cfg.TrimSpace)
			uniq := core.Deduplicate(b, spans, perFile)
			parts[i] = store.MergedPart{Buf: b, Spans: uniq, Total: len(spans), Release: release}
		}(i, path)
	}
	wg.Wait()

	var totalLines int64
	for i, p := range parts {
		if errs[i] != nil {
			return nil, fmt.Errorf("read %q: %w", cfg.Inputs[i], errs[i])
		}
		totalLines += int64(p.Total)
	}

	label := filepath.Base(cfg.Output)
	f, commit, err := store.OpenAtomic(cfg.Output)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", cfg.Output, err)
	}

	progress.Set("merge", 0)
	var bytesWritten, linesWritten int64
	werr := store.WriteMerged(parts, f, &bytesWritten, &linesWritten)
	if werr == nil {
		werr = commit()
	} else {
		_ = os.Remove(f.Name())
	}
	if werr != nil {
		return nil, fmt.Errorf("write %q: %w", cfg.Output, werr)
	}

	for i := range parts {
		if parts[i].Release != nil {
			// Release unmaps the merged part; the error is non-actionable after a successful write.
			_ = parts[i].Release()
		}
	}

	return []report.Result{{
		Target:         label,
		TotalLines:     totalLines,
		UniqueLines:    linesWritten,
		DuplicateLines: totalLines - linesWritten,
		BytesWritten:   bytesWritten,
		Duration:       time.Since(t0),
	}}, nil
}
