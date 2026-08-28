package core

// Deduplicate returns the sorted, unique spans from the input.
//
// Pipeline:
//  1. Detect whether the spans are already in order (common for curated wordlists
//     such as rockyou.txt). If so, skip the O(N log N) sort entirely.
//  2. Otherwise sort the spans (sort+compact beats a map: no per-line hashing of
//     140 MB of input, no bucket overhead, no GC churn from map entries).
//  3. Compact adjacent equal spans in place. Because the slice is sorted, duplicates
//     are always contiguous, so a single linear scan removes them with no hashing or
//     map lookups.
//
// workers is the parallelism budget for the order check; it is clamped internally.
func Deduplicate(buf []byte, spans []Span, workers int) []Span {
	if len(spans) == 0 {
		return spans
	}
	// Fast path: already-ordered input needs no sort. The order check runs in parallel
	// across the pool and fails within a handful of comparisons for genuinely
	// unsorted input, so it costs almost nothing on the general case.
	sorted, strict := classifyOrder(buf, spans, workers)
	if !sorted {
		SortSpans(buf, spans, workers)
		return compactUnique(buf, spans)
	}
	if strict {
		// Already strictly increasing: nothing to remove.
		return spans
	}
	return compactUnique(buf, spans)
}

// compactUnique keeps the first span of each run of equal spans. Because the slice
// is sorted (or already order-preserving), duplicates are always contiguous, so a
// single linear scan removes them with no hashing. The slice is compacted in place;
// the returned sub-slice shares the underlying array.
func compactUnique(buf []byte, spans []Span) []Span {
	w := 1
	for i := 1; i < len(spans); i++ {
		if !spanEqual(buf, spans[i], spans[w-1]) {
			spans[w] = spans[i]
			w++
		}
	}
	return spans[:w]
}
