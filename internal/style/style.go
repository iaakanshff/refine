// Package style provides a tiny, dependency-free ANSI color helper.
//
// We deliberately avoid pulling in a third-party styling library: refine does not
// need rich terminal markup, and every external dependency is attack surface we
// would have to audit and keep patched for no real gain. A handful of ANSI
// escapes cover everything the CLI needs.
//
// Coloring is opt-in and respects the NO_COLOR convention (https://no-color.org)
// as well as non-TTY outputs, so refine stays safe to use in pipes and scripts.
package style

import "os"

// enabled is toggled once during startup based on the user's flags and the
// environment. All paint functions become no-ops when it is false, which keeps
// call sites free of branching noise.
var enabled bool

// Configure decides whether colors should be emitted.
//
//   - if noColor is true the caller explicitly opted out.
//   - if toStdErr is false (stderr is not a terminal) we stay monochrome so
//     that redirected output is not littered with escape codes.
//   - the NO_COLOR environment variable, when set, forces monochrome.
func Configure(noColor, toStdErr bool) {
	enabled = !noColor && toStdErr && os.Getenv("NO_COLOR") == ""
}

// paint wraps s in the ANSI escape sequence for the given SGR code, e.g. "31" for red. It
// is a no-op when coloring is disabled or s is empty, so callers never need to branch on
// enabled themselves.
func paint(code, s string) string {
	if !enabled || s == "" {
		return s
	}
	return "\033[" + code + "m" + s + "\033[0m"
}

// Red wraps s in ANSI red (SGR 31).
func Red(s string) string { return paint("31", s) }

// Green wraps s in ANSI green (SGR 32).
func Green(s string) string { return paint("32", s) }

// Yellow wraps s in ANSI yellow (SGR 33).
func Yellow(s string) string { return paint("33", s) }

// Cyan wraps s in ANSI cyan (SGR 36); used for accents like the spinner and brand mark.
func Cyan(s string) string { return paint("36", s) }

// Bold wraps s in ANSI bold (SGR 1).
func Bold(s string) string { return paint("1", s) }

// Dim wraps s in ANSI dim (SGR 2); used for separators and secondary labels.
func Dim(s string) string { return paint("2", s) }
