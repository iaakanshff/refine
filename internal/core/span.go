package core

// Span is a half-open byte range [Start, Start+Len) inside an external buffer,
// plus a cached prefix used to make sorting cache-friendly.
//
// Why offsets instead of []byte or string:
//
//   - A []byte header is 24 bytes; an int pair (plus prefix) is 16. For 14M
//     lines that is the difference between ~336 MB and ~224 MB of bookkeeping.
//   - More importantly, sorting a slice of small, fixed-size, contiguous records
//     is cache-friendly. The only cold-memory access during a comparison is the
//     actual byte payload, and the prefix below removes that access in the
//     common case.
//
// Invariant: a Span is only valid while its backing buffer is alive and
// unmodified. The caller owns the buffer.
//
// Start/Len are int32, so an individual input file is limited to 2 GB. That is
// intentional: refine is a wordlist/line tool, and keeping spans at 16 bytes
// (vs 24 for int64) matters when there are tens of millions of them.
type Span struct {
	Start  int32  // byte offset of the line's first byte within the backing buffer
	Len    int32  // byte length of the line, excluding the trailing newline
	Prefix uint64 // big-endian uint64 of the line's first 8 bytes (0-padded)
}

// Bytes returns the line payload referenced by the span. Centralizing the int32
// offset arithmetic keeps it in one place and documents the [Start, Start+Len)
// invariant.
func (s Span) Bytes(buf []byte) []byte {
	return buf[int(s.Start) : int(s.Start)+int(s.Len)]
}

// makeSpan builds a Span and precomputes its 8-byte big-endian prefix. The prefix
// lets comparisons short-circuit without touching the backing buffer unless two
// lines genuinely share their first 8 bytes.
func makeSpan(buf []byte, start, end int) Span {
	return Span{
		Start:  int32(start),
		Len:    int32(end - start),
		Prefix: prefixOf(buf, start, end),
	}
}

// prefixOf returns the first up to 8 bytes of buf[start:end] as a big-endian
// uint64, zero-padded on the right. Big-endian means the uint64's numeric order
// equals the lexicographic order of the prefix, so comparing prefixes is a correct
// stand-in for comparing the lines whenever they differ early.
//
// The shift-or loop is intentionally a simple serial chain: it runs once per line
// during scanning, where it is dominated by memory bandwidth, not by this tiny
// dependency chain. (An unaligned 8-byte load is an alternative, but it needs an
// explicit length guard and is not clearly faster here.)
func prefixOf(buf []byte, start, end int) uint64 {
	var p uint64
	m := end - start
	if m > 8 {
		m = 8
	}
	for i := 0; i < m; i++ {
		p = (p << 8) | uint64(buf[start+i])
	}
	// Pad remaining high-order bytes with zeros so shorter lines sort before
	// their longer extensions (e.g. "ab" < "abc").
	return p << uint(8*(8-m))
}
