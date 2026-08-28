// Package term provides terminal introspection used to decide whether it is
// safe to emit ANSI escapes and live progress.
//
// refine writes its human report to stderr, which is usually a terminal but may
// be a pipe, a redirected file, or /dev/null. The UI must behave differently in
// each case: color and carriage-return tricks are great on a TTY but corrupt
// captured logs, so we gate them on this single check.
package term

import "os"

// IsTerminal reports whether f is an interactive character device (a TTY).
//
// The check is a stat + mode test with no side effects, so it is cheap to call
// once at startup. A false result tells the UI to stay monochrome and silent so
// redirected output stays clean.
func IsTerminal(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}
