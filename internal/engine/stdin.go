package engine

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/yourpwnguy/refine/internal/cli"
	"github.com/yourpwnguy/refine/internal/report"
	"github.com/yourpwnguy/refine/internal/store"
)

// runStdin handles `cat file | refine [out]`. Data comes from stdin; the optional positional file
// (cfg.StdinToFile) is the destination. With no destination the result goes to stdout.
func runStdin(cfg cli.Config, progress *report.Progress) ([]report.Result, error) {
	progress.Set("read stdin", 0)
	buf, err := store.ReadStdin()
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}

	var label string
	var out *os.File
	var commit func() error
	if cfg.StdinToFile != "" {
		label = filepath.Base(cfg.StdinToFile)
		f, c, err := store.OpenAtomic(cfg.StdinToFile)
		if err != nil {
			return nil, fmt.Errorf("open %q: %w", cfg.StdinToFile, err)
		}
		out, commit = f, c
	} else {
		label = "stdin"
		out = os.Stdout
	}

	res, err := dedupeAndWrite(cfg, buf, label, out, commit, progress, true)
	if err != nil {
		return nil, err
	}
	return []report.Result{res}, nil
}
