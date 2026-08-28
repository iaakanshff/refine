package engine

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/yourpwnguy/refine/internal/cli"
	"github.com/yourpwnguy/refine/internal/report"
	"github.com/yourpwnguy/refine/internal/store"
)

// runWildcard handles `refine -w dir [-e a,b]`. Every regular file in the directory is deduplicated
// independently and rewritten in place, in parallel across a bounded worker pool. This is the headline
// concurrency win for the directory use case: files are completely independent, so we can saturate all
// cores with zero locking on shared data.
func runWildcard(cfg cli.Config, progress *report.Progress) ([]report.Result, error) {
	exclude := make(map[string]bool, len(cfg.Exclude))
	for _, e := range cfg.Exclude {
		exclude[e] = true
	}
	files, err := store.ListFiles(cfg.WildcardDir, exclude)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no files to process in %q", cfg.WildcardDir)
	}

	progress.Set("processing", int64(len(files)))
	results := make([]report.Result, len(files))
	errs := make([]error, len(files))

	// sem bounds concurrent file processing to cfg.Workers so a directory of thousands of
	// files doesn't open them all at once (which would exhaust mmap slots / memory).
	sem := make(chan struct{}, cfg.Workers)
	var wg sync.WaitGroup
	for i, path := range files {
		wg.Add(1)
		sem <- struct{}{} // acquire a worker slot
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
			f, commit, oerr := store.OpenAtomic(path)
			if oerr != nil {
				// Release unmaps the just-read input on the failed open; ignore the error.
				_ = release()
				errs[i] = oerr
				return
			}
			// No per-file byte counter here: the shared progress counter is single-slot, so we report
			// file completion via Add instead.
			res, derr := dedupeAndWrite(cfg, b, label, f, commit, progress, false)
			// release unmaps the input buffer now that it has been consumed; ignore the error.
			_ = release()
			if derr != nil {
				errs[i] = derr
				return
			}
			results[i] = res
			progress.Add(1)
		}(i, path)
	}
	wg.Wait()

	// Surface per-file failures to the user without aborting the whole run. We compact the
	// successful results in place (reusing the results slice) and keep the first error so the
	// caller can still see that something failed.
	var firstErr error
	clean := results[:0]
	for i, r := range results {
		if errs[i] != nil {
			if firstErr == nil {
				firstErr = errs[i]
			}
			failf("%v", errs[i])
			continue
		}
		clean = append(clean, r)
	}
	if len(clean) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return clean, nil
}
