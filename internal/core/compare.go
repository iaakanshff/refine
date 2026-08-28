package core

import "bytes"

// spanLess compares two spans lexicographically. This is the single most important
// function in the hot loop, so it is worth understanding precisely.
//
// The fast path compares the cached 8-byte prefix, which lives contiguously inside
// the spans slice and is therefore cache-hot. The full byte comparison into the cold
// backing buffer only runs when two lines genuinely share their first 8 bytes,
// which is rare for real wordlists (where prefixes are effectively random). This
// turns most comparisons from random walks into a 280 MB+ buffer into a cheap
// uint64 compare.
func spanLess(buf []byte, a, b Span) bool {
	if a.Prefix != b.Prefix {
		return a.Prefix < b.Prefix
	}
	return bytes.Compare(a.Bytes(buf), b.Bytes(buf)) < 0
}

// spanEqual reports whether two spans hold identical bytes. It mirrors spanLess's
// prefix shortcut for the same performance reason.
func spanEqual(buf []byte, a, b Span) bool {
	return bytes.Equal(a.Bytes(buf), b.Bytes(buf))
}
