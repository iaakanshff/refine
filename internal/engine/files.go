package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yourpwnguy/refine/internal/cli"
	"github.com/yourpwnguy/refine/internal/core"
	"github.com/yourpwnguy/refine/internal/report"
	"github.com/yourpwnguy/refine/internal/store"
)

// runFiles handles `refine a.txt [out]`: deduplicate one input and write the result. The buffer is
// memory-mapped for zero-copy reads, scanned in parallel across the worker pool (the dominant cost),
// then deduplicated. Because a single file is one logical input, the spans are kept in one slice and
// sorted once — this parallelizes the scan without paying for a serial k-way merge.
//
// When cfg.Output differs from the input it is treated as output mode: the source file is left
// untouched and the deduped result is written to the (possibly new) output path.
func runFiles(cfg cli.Config, progress *report.Progress) ([]report.Result, error) {
	t0 := time.Now()
	if len(cfg.Inputs) != 1 {
		return nil, fmt.Errorf("internal: runFiles expects exactly one input")
	}
	path := cfg.Inputs[0]
	outPath := cfg.Output

	progress.Set("scan "+filepath.Base(path), 0)
	buf, release, err := store.ReadFileMmap(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	// release unmaps the input buffer. It runs again on the error path below, so ignore
	// the deferred result here (idempotent close).
	defer func() { _ = release() }()

	spans := core.ExtractSpans(buf, cfg.TrimSpace)
	uniq := core.Deduplicate(buf, spans, cfg.Workers)

	label := filepath.Base(outPath)
	f, commit, err := store.OpenAtomic(outPath)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", outPath, err)
	}

	progress.Set("dedupe", 0)
	var bytesWritten int64
	werr := store.WriteSpans(buf, uniq, f, &bytesWritten)
	if werr == nil {
		werr = commit()
	} else {
		_ = os.Remove(f.Name())
	}
	if werr != nil {
		return nil, fmt.Errorf("write %q: %w", outPath, werr)
	}

	return []report.Result{{
		Target:         label,
		TotalLines:     int64(len(spans)),
		UniqueLines:    int64(len(uniq)),
		DuplicateLines: int64(len(spans)) - int64(len(uniq)),
		BytesWritten:   bytesWritten,
		Duration:       time.Since(t0),
	}}, nil
}
