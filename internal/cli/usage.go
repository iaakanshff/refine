package cli

import (
	"fmt"
	"os"
)

// version is stamped into the binary at build time via ldflags so
// `refine -v` reports the exact version without recompiling.
var version string

// usage is the help text printed for -h/--help and on flag errors. It documents every mode and
// flag in one place so users never have to guess the invocation.
const usage = `refine - fast line deduplication and sorting

Usage:
  refine [options] <file>                 dedupe & sort a file in place
  refine [options] <in> <out>            if out exists: merge in+out into out;
                                         otherwise: dedupe in, write result to out
  refine -o <out> <in>                   dedupe in, write result to out
  cat <file> | refine [options]           dedupe stdin, print to stdout
  cat <file> | refine [options] <out>    dedupe stdin, write to out
  refine -w <dir> [-e a,b]               dedupe & sort every file in dir

Options:
  -w, --wildcard <dir>        process every regular file in a directory
  -e, --exclude <a,b>         comma-separated base names to skip (with -w)
  -o, --output <file>         write output to this file (overrides default)
  -t, --trim-space            strip leading/trailing whitespace from lines
      --workers <n>           parallelism for sorting (default: all cores)
      --json                  emit machine-readable stats on stderr
      --no-color              disable colored output
  -q, --quiet                 disable the live progress display
  -v, --version               print version and exit
  -h, --help                  show this help and exit

Notes:
  Files are rewritten atomically, so an interrupted run never corrupts the
  original. Lines are compared verbatim; use -t to normalize whitespace.
`

// PrintHelp writes the usage text. Kept here so main stays a thin wiring layer.
func PrintHelp() { fmt.Fprint(os.Stderr, usage) }

// PrintVersion prints the version line.
func PrintVersion() { fmt.Fprintln(os.Stderr, "refine "+version) }
