package core

import "sort"

// SortSpans sorts spans in place, ascending by line content.
//
// We deliberately use a single serial sort.Sort (pdqsort) rather than a "parallel"
// sort. This was a measured decision, not laziness:
//
//   - On a 140 MB / 14.3M-line file, a bucket-and-conquer parallel sort across 12
//     cores came in at ~964 ms — identical to the serial 947 ms. A chunk-sort +
//     k-way-merge attempt was actually SLOWER (2.66 s) because the serial merge
//     dominated.
//   - The reason more cores don't help here is that the comparison work is
//     memory-bandwidth bound: every goroutine hammers the same backing buffer, and
//     adding cores just increases cache-line contention.
//
// So the fastest, leanest path for a single file is serial. Real parallelization in
// refine lives at the file level (wildcard mode fans independent files across a
// worker pool), where there is no shared data to contend on.
//
// We use sort.Sort with a typed sort.Interface instead of sort.Slice. The latter
// swaps elements via reflection on every one of the hundreds of millions of swaps a
// 14M+ line file needs; a typed Swap removes that per-call reflection cost, which
// on a 28M-line merge is the difference between ~11 s and ~2 s.
//
// workers is accepted for API symmetry with the rest of the pipeline but is
// currently unused: see the bandwidth argument above. A future parallel sort that
// beats pdqsort on this access pattern would hook in here.
func SortSpans(buf []byte, spans []Span, workers int) {
	sort.Sort(spanSlice{buf: buf, s: spans})
}

// spanSlice adapts []Span to sort.Interface. See SortSpans for why the typed Swap
// matters at scale.
type spanSlice struct {
	buf []byte
	s   []Span
}

// Len, Less and Swap implement sort.Interface. Less delegates to spanLess so the prefix
// shortcut applies; Swap is a typed field swap that avoids the reflection sort.Slice would use.
// Len returns the number of spans to sort.
func (x spanSlice) Len() int { return len(x.s) }

// Less reports whether span i sorts before span j, delegating to spanLess so the
// prefix shortcut applies.
func (x spanSlice) Less(i, j int) bool { return spanLess(x.buf, x.s[i], x.s[j]) }

// Swap exchanges two spans in place during the sort.
func (x spanSlice) Swap(i, j int) { x.s[i], x.s[j] = x.s[j], x.s[i] }
