package core

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
)

// spanText reconstructs the line text for each span from the backing buffer.
func spanText(buf []byte, spans []Span) []string {
	out := make([]string, len(spans))
	for i, s := range spans {
		out[i] = string(s.Bytes(buf))
	}
	return out
}

func TestExtractSpansBasic(t *testing.T) {
	buf := []byte("alpha\nbeta\ngamma\n")
	spans := ExtractSpans(buf, false)
	got := spanText(buf, spans)
	want := []string{"alpha", "beta", "gamma"}
	if len(got) != len(want) {
		t.Fatalf("got %d spans, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("span %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExtractSpansTrailingLine(t *testing.T) {
	// No final newline: the last line must still be captured.
	buf := []byte("one\ntwo")
	spans := ExtractSpans(buf, false)
	if len(spans) != 2 {
		t.Fatalf("got %d spans, want 2", len(spans))
	}
	if string(spans[1].Bytes(buf)) != "two" {
		t.Errorf("last span = %q, want %q", spans[1].Bytes(buf), "two")
	}
}

func TestExtractSpansCRLF(t *testing.T) {
	// A trailing carriage return must be stripped even when trim is off.
	buf := []byte("foo\r\nbar\r\n")
	spans := ExtractSpans(buf, false)
	got := spanText(buf, spans)
	if len(got) != 2 || got[0] != "foo" || got[1] != "bar" {
		t.Errorf("CRLF handling wrong: %q", got)
	}
}

func TestExtractSpansTrim(t *testing.T) {
	buf := []byte("  spaced  \t\n\tindented\t\n")
	spans := ExtractSpans(buf, true)
	got := spanText(buf, spans)
	want := []string{"spaced", "indented"}
	if len(got) != len(want) {
		t.Fatalf("got %d spans, want %d (%q)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("span %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDeduplicateCorrectness(t *testing.T) {
	buf := []byte("delta\nbeta\nalpha\nbeta\ngamma\nalpha\n")
	spans := ExtractSpans(buf, false)
	uniq := Deduplicate(buf, spans, 1)
	got := spanText(buf, uniq)
	want := []string{"alpha", "beta", "delta", "gamma"}
	if len(got) != len(want) {
		t.Fatalf("got %d unique, want %d (%q)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDeduplicateParallelMatchesSerial(t *testing.T) {
	// All-unique lines so the expected order is fully deterministic. The
	// parallel path (chunk sort + heap merge) must produce byte-identical
	// output to the serial path.
	buf := make([]byte, 0, 1<<20)
	var lines []string
	r := rand.New(rand.NewSource(42))
	for i := 0; i < 200000; i++ {
		lines = append(lines, fmt.Sprintf("line-%08d-%c%c%c", i, byte('a'+r.Intn(26)), byte('a'+r.Intn(26)), byte('a'+r.Intn(26))))
	}
	for i, l := range lines {
		if i > 0 {
			buf = append(buf, '\n')
		}
		buf = append(buf, l...)
	}

	serial := Deduplicate(buf, ExtractSpans(buf, false), 1)
	parallel := Deduplicate(buf, ExtractSpans(buf, false), 8)

	if len(serial) != len(parallel) {
		t.Fatalf("length mismatch: serial=%d parallel=%d", len(serial), len(parallel))
	}
	for i := range serial {
		a := serial[i].Bytes(buf)
		b := parallel[i].Bytes(buf)
		if !bytes.Equal(a, b) {
			t.Fatalf("position %d differs: %q vs %q", i, a, b)
		}
	}
}

func BenchmarkExtractSpans(b *testing.B) {
	buf := genBuffer(500000, 0.3)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExtractSpans(buf, false)
	}
}

func BenchmarkDeduplicate(b *testing.B) {
	buf := genBuffer(500000, 0.3)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		spans := ExtractSpans(buf, false)
		_ = Deduplicate(buf, spans, 8)
	}
}

// genBuffer builds a buffer of n lines with a configurable duplicate ratio.
// The duplicates share prefixes (like real wordlists) so the comparison work
// resembles production input.
func genBuffer(n int, dupRatio float64) []byte {
	r := rand.New(rand.NewSource(1))
	var buf []byte
	prefixes := []string{"admin", "password", "test", "user", "root", "guest", "dev", "qa"}
	for i := 0; i < n; i++ {
		if i > 0 {
			buf = append(buf, '\n')
		}
		p := prefixes[r.Intn(len(prefixes))]
		if r.Float64() < dupRatio {
			// Force a duplicate by reusing a small id space.
			buf = append(buf, fmt.Sprintf("%s%d", p, r.Intn(n/10))...)
		} else {
			buf = append(buf, fmt.Sprintf("%s%d", p, i)...)
		}
	}
	return buf
}
