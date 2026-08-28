package store

import (
	"bytes"
	"io"
	"sync/atomic"

	"github.com/yourpwnguy/refine/internal/core"
)

// MergedPart is one input file's deduplicated, still-sorted spans together with the
// buffer they reference. WriteMerged combines many such parts into one globally sorted,
// globally unique stream.
//
// Release tears down the part's backing buffer (an mmap view). Callers must invoke it
// once the part is no longer needed so the kernel mapping is reclaimed; the bytes
// referenced by Spans become invalid after Release.
type MergedPart struct {
	Buf     []byte
	Spans   []core.Span
	Total   int // line count before dedup, for reporting
	Release func() error
}

// WriteMerged k-way merges already-sorted-and-deduplicated parts into w, emitting a
// single globally sorted, globally unique stream.
//
// Because every part is internally sorted, the merge is a heap merge: pop the smallest
// current span, write it (skipping a global duplicate of the previously written line),
// advance that part's cursor. Comparisons are cache-friendly — each part is scanned
// sequentially — and the byte-level fallback only fires when two lines share their first
// 8 bytes.
//
// The CLI caps merges at two inputs, so the common case is a 2-way merge. Handle it with
// a direct two-pointer walk instead of a heap: when both sides present the same line we
// advance both cursors at once, which halves the work for the frequent "combine two
// overlapping wordlists" pattern.
func WriteMerged(parts []MergedPart, w io.Writer, bytesWritten, linesWritten *int64) error {
	if len(parts) == 2 {
		return mergeTwo(parts, w, bytesWritten, linesWritten)
	}

	// Upper bound on output size: every line of every part, plus a newline each.
	upper := 0
	for _, p := range parts {
		for _, s := range p.Spans {
			upper += int(s.Len) + 1
		}
	}
	out := make([]byte, 0, upper)
	nl := byte('\n')

	h := &mergeHeap{parts: parts}
	for i, p := range parts {
		if len(p.Spans) > 0 {
			h.push(mCursor{pi: i, pos: 0})
		}
	}

	var last []byte
	first := true
	for h.Len() > 0 {
		c := h.pop()
		span := parts[c.pi].Spans[c.pos]
		line := span.Bytes(parts[c.pi].Buf)

		if first || !bytes.Equal(line, last) {
			out = append(out, line...)
			out = append(out, nl)
			if linesWritten != nil {
				atomic.AddInt64(linesWritten, 1)
			}
			last = line
			first = false
		}
		c.pos++
		if c.pos < len(parts[c.pi].Spans) {
			h.push(c)
		}
	}
	if bytesWritten != nil {
		atomic.AddInt64(bytesWritten, int64(len(out)))
	}
	_, err := w.Write(out)
	return err
}

// mergeTwo is the 2-way special case of WriteMerged: a single two-pointer walk over the
// two sorted runs. It is both simpler and faster than a heap, and when the same line
// appears in both inputs it is emitted once and both cursors advance, so overlapping
// lists cost ~half the comparisons of a generic merge.
func mergeTwo(parts []MergedPart, w io.Writer, bytesWritten, linesWritten *int64) error {
	a, b := parts[0], parts[1]
	upper := 0
	for _, s := range a.Spans {
		upper += int(s.Len) + 1
	}
	for _, s := range b.Spans {
		upper += int(s.Len) + 1
	}
	out := make([]byte, 0, upper)
	nl := byte('\n')

	i, j := 0, 0
	na, nb := len(a.Spans), len(b.Spans)
	var last []byte
	first := true
	emit := func(line []byte) {
		out = append(out, line...)
		out = append(out, nl)
		if linesWritten != nil {
			atomic.AddInt64(linesWritten, 1)
		}
		last = line
		first = false
	}
	for i < na && j < nb {
		sa, sb := a.Spans[i], b.Spans[j]
		cmp := compareSpans(a.Buf, sa, b.Buf, sb)
		if cmp <= 0 {
			line := sa.Bytes(a.Buf)
			if first || !bytes.Equal(line, last) {
				emit(line)
			}
			i++
			if cmp == 0 {
				j++ // identical line already emitted from the other side
			}
		} else {
			line := sb.Bytes(b.Buf)
			if first || !bytes.Equal(line, last) {
				emit(line)
			}
			j++
		}
	}
	for ; i < na; i++ {
		line := a.Spans[i].Bytes(a.Buf)
		if first || !bytes.Equal(line, last) {
			emit(line)
		}
	}
	for ; j < nb; j++ {
		line := b.Spans[j].Bytes(b.Buf)
		if first || !bytes.Equal(line, last) {
			emit(line)
		}
	}
	if bytesWritten != nil {
		atomic.AddInt64(bytesWritten, int64(len(out)))
	}
	_, err := w.Write(out)
	return err
}

// compareSpans orders two spans by prefix (cheap, cache-hot) and only falls back to a
// full byte comparison when the first 8 bytes match.
func compareSpans(ba []byte, sa core.Span, bb []byte, sb core.Span) int {
	if sa.Prefix != sb.Prefix {
		if sa.Prefix < sb.Prefix {
			return -1
		}
		return 1
	}
	return bytes.Compare(sa.Bytes(ba), sb.Bytes(bb))
}
