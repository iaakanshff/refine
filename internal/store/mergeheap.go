package store

import "bytes"

// mCursor points at the next unmerged span of one part. It is the atom of the
// merge heap.
type mCursor struct {
	pi  int // index into the parts slice (which input file this cursor walks)
	pos int // next span index within that part's Spans slice
}

// mergeHeap is a binary min-heap of mCursors ordered by the referenced line's prefix
// (falling back to a full byte comparison only when two lines share their first 8 bytes).
//
// It is hand-rolled rather than container/heap to avoid the per-operation interface
// dispatch, which dominates once the merge runs over tens of millions of spans. The heap
// stores cursors, not spans, so each entry is tiny and the comparisons reuse the cached
// prefixes in the spans slice.
type mergeHeap struct {
	parts []MergedPart // the inputs being merged; the heap only borrows spans/buffers from here
	cur   []mCursor    // the live heap entries: one per part that still has an unconsumed span
}

// Len returns the number of parts that still have spans to emit.
func (h *mergeHeap) Len() int { return len(h.cur) }

// less reports whether the line at heap index i should sort before the line at j. It
// reuses the cached Prefix first and only touches the cold backing buffer when two lines
// share their first 8 bytes.
func (h *mergeHeap) less(i, j int) bool {
	a := h.parts[h.cur[i].pi].Spans[h.cur[i].pos]
	b := h.parts[h.cur[j].pi].Spans[h.cur[j].pos]
	if a.Prefix != b.Prefix {
		return a.Prefix < b.Prefix
	}
	return bytes.Compare(a.Bytes(h.parts[h.cur[i].pi].Buf), b.Bytes(h.parts[h.cur[j].pi].Buf)) < 0
}

// push appends c and restores the heap invariant by sifting it up from the tail.
func (h *mergeHeap) push(c mCursor) {
	h.cur = append(h.cur, c)
	h.siftUp(len(h.cur) - 1)
}

// pop removes and returns the smallest entry (the next line to emit). It swaps the root
// with the tail, sifts the new root down, then shrinks the slice — the classic O(log n)
// extract-min.
func (h *mergeHeap) pop() mCursor {
	n := len(h.cur) - 1
	h.cur[0], h.cur[n] = h.cur[n], h.cur[0]
	h.siftDown(0, n)
	c := h.cur[n]
	h.cur = h.cur[:n]
	return c
}

// siftUp restores the heap invariant above index i by swapping with the parent while the
// parent is larger.
func (h *mergeHeap) siftUp(i int) {
	for i > 0 {
		p := (i - 1) / 2
		if !h.less(i, p) {
			return
		}
		h.cur[i], h.cur[p] = h.cur[p], h.cur[i]
		i = p
	}
}

// siftDown restores the heap invariant below index i by swapping with the smaller child
// while a child is smaller. n is the current heap size, passed so a single pop can shrink
// the effective range before calling it.
func (h *mergeHeap) siftDown(i, n int) {
	for {
		l := 2*i + 1
		if l >= n {
			return
		}
		j := l
		if r := l + 1; r < n && h.less(r, l) {
			j = r
		}
		if !h.less(j, i) {
			return
		}
		h.cur[i], h.cur[j] = h.cur[j], h.cur[i]
		i = j
	}
}
