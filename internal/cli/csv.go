package cli

import "os"

// splitCSV splits a comma-separated list, trimming spaces and dropping empty entries so
// "-e a,,b" yields exactly {"a","b"}. It is intentionally minimal: refine's exclude list is a
// small set of file names, not a mini-DSL.
func splitCSV(s string) []string {
	var out []string
	cur := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			tok := trimSpaceASCII(s[cur:i])
			if tok != "" {
				out = append(out, tok)
			}
			cur = i + 1
		}
	}
	return out
}

// trimSpaceASCII trims leading/trailing ASCII whitespace. We only need ASCII here because the
// values are file base names, which are ASCII in every case refine cares about.
func trimSpaceASCII(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// isStdin reports whether standard input is a pipe (not a terminal). When stdin is a TTY we treat
// it as "no input provided" rather than blocking on interactive input.
func isStdin() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) == 0
}
