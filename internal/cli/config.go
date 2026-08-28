// Package cli parses command-line arguments into a validated Config. It owns no behavior beyond
// understanding the user's intent: which inputs, what output, which modes, and how much
// parallelism/verbosity they asked for. Every other package consumes the Config; none of them
// re-parse flags, which keeps argument handling in exactly one place.
package cli

// Config is the fully resolved, mode-agnostic description of what refine should do. It is the
// single hand-off type between argument parsing and the engine.
type Config struct {
	Inputs      []string // file paths to read (file mode)
	Output      string   // destination path; empty means stdout (pipe mode)
	WildcardDir string   // when set, deduplicate every file in this directory
	Exclude     []string // base names to skip in wildcard mode
	TrimSpace   bool     // strip surrounding ASCII whitespace per line
	Workers     int      // parallelism budget for sorting / directory fan-out
	JSON        bool     // machine-readable stats on stderr
	NoColor     bool     // force monochrome output
	Quiet       bool     // suppress the live progress display
	Version     bool     // print version and exit
	Help        bool     // print help and exit
	IsStdin     bool     // input is being piped on stdin
	StdinToFile string   // when stdin and a path is given, write there
}
