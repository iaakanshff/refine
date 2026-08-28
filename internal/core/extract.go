package core

// ExtractSpans scans buf for newline-delimited lines and returns one Span per
// line. It is fully zero-copy: spans borrow bytes from buf.
//
// Trimming: when trim is true, leading/trailing ASCII whitespace is stripped from
// each line. When trim is false the raw bytes between newlines are kept verbatim,
// except that a single trailing carriage return is always removed so CRLF files
// behave like LF files.
//
// Blank lines are preserved exactly (a zero-length Span). The caller decides
// whether to drop them; we do not silently discard user data.
func ExtractSpans(buf []byte, trim bool) []Span {
	// Capacity guess ~ lines in buf (avg line is ~10 bytes), so we avoid most
	// reallocations without a second pass.
	spans := make([]Span, 0, len(buf)/8)
	start := 0
	n := len(buf)
	for i := 0; i < n; i++ {
		if buf[i] == '\n' {
			s, e := trimRange(buf, start, i, trim)
			spans = append(spans, makeSpan(buf, s, e))
			start = i + 1
		}
	}
	if start < n {
		s, e := trimRange(buf, start, n, trim)
		spans = append(spans, makeSpan(buf, s, e))
	}
	return spans
}

// trimRange returns the [start, end) range of a raw line, optionally trimming
// ASCII whitespace and always stripping a single trailing '\r'.
func trimRange(buf []byte, start, end int, trim bool) (int, int) {
	if trim {
		for start < end && isSpace(buf[start]) {
			start++
		}
		for end > start && isSpace(buf[end-1]) {
			end--
		}
	}
	if end > start && buf[end-1] == '\r' {
		end--
	}
	return start, end
}

// isSpace reports whether b is ASCII whitespace. We only need the ASCII set:
// refine compares raw bytes and non-ASCII whitespace would be a surprising thing
// to strip from a wordlist.
func isSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	default:
		return false
	}
}
