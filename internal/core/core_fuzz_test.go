package core

import (
	"bytes"
	"testing"
)

// FuzzDeduplicate hammers the parser + dedup pipeline with arbitrary bytes.
// The invariant we assert on every input (including the empty string, all
// newlines, or a single unterminated line) is that the output is always
// sorted ascending and contains no duplicate spans, and that the function
// never panics. This is the kind of test that would have caught the
// crash-on-malformed-input bugs in earlier versions.
func FuzzDeduplicate(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("\n\n\n"))
	f.Add([]byte("single"))
	f.Add([]byte("a\nb\na\n"))
	f.Add([]byte("foo\r\nbar\r\nfoo\r\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		spans := ExtractSpans(data, false)
		uniq := Deduplicate(data, spans, 4)

		for i := 1; i < len(uniq); i++ {
			prev := uniq[i-1].Bytes(data)
			cur := uniq[i].Bytes(data)
			if bytes.Compare(prev, cur) > 0 {
				t.Fatalf("output not sorted: %q > %q", prev, cur)
			}
			if bytes.Equal(prev, cur) {
				t.Fatalf("duplicate remained: %q", cur)
			}
		}
	})
}
