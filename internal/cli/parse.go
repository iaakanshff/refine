package cli

import (
	"flag"
	"fmt"
	"os"
	"runtime"
)

// Parse builds a Config from raw arguments. args should be everything after the program name
// (i.e. os.Args[1:]).
//
// Mode selection is intentionally simple and driven by what kind of input was given:
//
//   - piped stdin          -> read stdin, optional single output file
//   - -w <dir>             -> deduplicate every file in a directory, in parallel
//   - one file             -> deduplicate in place
//   - <in> <out> (out new) -> deduplicate in, write the result to out (output mode)
//   - <in> <out> (out old) -> merge both into out (merge mode)
//   - -o <out> <in>        -> explicit output mode, regardless of whether out exists
func Parse(args []string) (Config, error) {
	var cfg Config

	fs := flag.NewFlagSet("refine", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }

	var wildcard, excludeCSV string
	fs.StringVar(&wildcard, "wildcard", "", "process every file in a directory")
	fs.StringVar(&wildcard, "w", "", "process every file in a directory")
	fs.StringVar(&excludeCSV, "exclude", "", "comma-separated file names to skip")
	fs.StringVar(&excludeCSV, "e", "", "comma-separated file names to skip")
	fs.StringVar(&cfg.Output, "output", "", "write output to this file")
	fs.StringVar(&cfg.Output, "o", "", "write output to this file")
	fs.BoolVar(&cfg.TrimSpace, "trim-space", false, "trim whitespace from lines")
	fs.BoolVar(&cfg.TrimSpace, "t", false, "trim whitespace from lines")
	fs.IntVar(&cfg.Workers, "workers", runtime.GOMAXPROCS(0), "parallelism for wildcard/directory mode")
	fs.BoolVar(&cfg.JSON, "json", false, "machine-readable stats")
	fs.BoolVar(&cfg.NoColor, "no-color", false, "disable color")
	fs.BoolVar(&cfg.Quiet, "quiet", false, "disable progress")
	fs.BoolVar(&cfg.Quiet, "q", false, "disable progress")
	fs.BoolVar(&cfg.Version, "version", false, "print version")
	fs.BoolVar(&cfg.Version, "v", false, "print version")
	fs.BoolVar(&cfg.Help, "help", false, "show help")
	fs.BoolVar(&cfg.Help, "h", false, "show help")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

	cfg.WildcardDir = wildcard
	if excludeCSV != "" {
		cfg.Exclude = splitCSV(excludeCSV)
	}
	if cfg.Workers < 1 {
		cfg.Workers = 1
	}

	// Help/version short-circuit everything else.
	if cfg.Help || cfg.Version {
		return cfg, nil
	}

	positional := fs.Args()

	// Stdin mode: data is piped in. An optional single positional argument is the output file;
	// anything else is a usage error.
	if isStdin() {
		cfg.IsStdin = true
		switch len(positional) {
		case 0:
			cfg.StdinToFile = ""
		case 1:
			cfg.StdinToFile = positional[0]
		default:
			return cfg, fmt.Errorf("stdin mode takes at most one file argument")
		}
		return cfg, nil
	}

	// Wildcard mode ignores positional files; the directory is the input.
	if cfg.WildcardDir != "" {
		if len(positional) != 0 {
			return cfg, fmt.Errorf("wildcard mode takes no positional files")
		}
		return cfg, nil
	}

	// File mode.
	switch len(positional) {
	case 0:
		return cfg, fmt.Errorf("no input file given")
	case 1:
		cfg.Inputs = positional
		if cfg.Output == "" {
			cfg.Output = positional[0] // in place
		}
	case 2:
		if cfg.Output != "" {
			// Explicit -o: merge both inputs into the named output file.
			cfg.Inputs = positional
		} else if !fileExists(positional[1]) {
			// The destination doesn't exist yet: read the first file and write the
			// deduped/sorted result into it, leaving the source untouched.
			cfg.Inputs = positional[:1]
			cfg.Output = positional[1]
		} else {
			// Both files exist: merge them into the second (current behavior).
			cfg.Inputs = positional
			cfg.Output = positional[1]
		}
	default:
		return cfg, fmt.Errorf("too many arguments: expected at most 2 files")
	}
	return cfg, nil
}

// fileExists reports whether path is accessible (used to decide whether a second positional
// argument is an existing merge target or a fresh output).
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
