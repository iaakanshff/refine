package engine

import (
	"fmt"
	"os"
	"time"

	"github.com/yourpwnguy/refine/internal/cli"
	"github.com/yourpwnguy/refine/internal/core"
	"github.com/yourpwnguy/refine/internal/report"
	"github.com/yourpwnguy/refine/internal/store"
	"github.com/yourpwnguy/refine/internal/style"
)

// dedupeAndWrite is the shared core-to-disk pipeline used by the stdin and wildcard modes. It
// parses buf into spans, dedupes them, and streams the result to out.
//
// label is the human-readable name shown in the report (usually a base file name or "stdin").
// commit, when non-nil, is invoked after a successful flush to atomically replace the destination
// (nil for stdout). attachCounter opts into live byte-throughput reporting; it is only safe with a
// single in-flight write, so the concurrent wildcard loop passes false and reports file completion
// via progress.Add instead.
func dedupeAndWrite(cfg cli.Config, buf []byte, label string, out *os.File, commit func() error, progress *report.Progress, attachCounter bool) (report.Result, error) {
	t0 := time.Now()

	spans := core.ExtractSpans(buf, cfg.TrimSpace)
	total := int64(len(spans))
	progress.Set("dedupe "+label, 0)

	uniq := core.Deduplicate(buf, spans, cfg.Workers)
	unique := int64(len(uniq))

	// Total bytes we're about to emit: every span plus a trailing newline.
	var totalBytes int64
	for _, s := range uniq {
		totalBytes += int64(s.Len) + 1
	}

	var bytesWritten int64
	if attachCounter && progress != nil {
		progress.UseCounter(&bytesWritten)
	}
	progress.Set("write "+label, totalBytes)

	werr := store.WriteSpans(buf, uniq, out, &bytesWritten)
	if commit != nil {
		if werr == nil {
			werr = commit()
		} else {
			// A failed write must not leave a half-written temp file behind.
			_ = os.Remove(out.Name())
		}
	}
	if werr != nil {
		return report.Result{}, fmt.Errorf("write %q: %w", label, werr)
	}

	return report.Result{
		Target:         label,
		TotalLines:     total,
		UniqueLines:    unique,
		DuplicateLines: total - unique,
		BytesWritten:   bytesWritten,
		Duration:       time.Since(t0),
	}, nil
}

// failf prints a per-file error to stderr using the same red coloring as the rest of the UI.
// It is the shared error-reporting helper for the concurrent modes.
func failf(format string, args ...any) {
	fmt.Fprintln(os.Stderr, style.Red("!")+" "+fmt.Sprintf(format, args...))
}
