package core

import "sync"

// classifyOrder reports whether spans are non-decreasing (sorted, equal neighbors
// allowed) and, if so, whether they are strictly increasing (no equal neighbors).
//
// Both checks run in parallel across the worker pool by examining each chunk
// independently and then reconciling the chunk boundaries. The chunk-local scan is a
// tight loop with no allocation; the boundary reconciliation is O(workers). Failed
// order checks abort as soon as a chunk reports unsorted, so genuinely unsorted
// input pays almost nothing.
//
// The two booleans are returned together because the caller needs both: a strictly
// increasing run needs no sort and no compaction, a merely non-decreasing run still
// needs compaction, and an unsorted run needs both sort and compaction.
func classifyOrder(buf []byte, spans []Span, workers int) (sorted, strict bool) {
	n := len(spans)
	if n < 2 {
		return true, true
	}
	if workers < 1 {
		workers = 1
	}
	if workers > n {
		workers = n
	}
	block := (n + workers - 1) / workers

	chunkSorted := make([]bool, workers)
	chunkStrict := make([]bool, workers)
	chunkFirst := make([]Span, workers)
	chunkLast := make([]Span, workers)

	var wg sync.WaitGroup
	for c := 0; c < workers; c++ {
		start, end := c*block, c*block+block
		if end > n {
			end = n
		}
		if start >= end {
			chunkSorted[c] = true
			chunkStrict[c] = true
			continue
		}
		wg.Add(1)
		go func(c, start, end int) {
			defer wg.Done()
			ok := true
			st := true
			prev := spans[start]
			for k := start + 1; k < end; k++ {
				cur := spans[k]
				if spanLess(buf, cur, prev) {
					ok = false
					break
				}
				if spanEqual(buf, cur, prev) {
					st = false
				}
				prev = cur
			}
			chunkSorted[c] = ok
			chunkStrict[c] = st
			chunkFirst[c] = spans[start]
			chunkLast[c] = spans[end-1]
		}(c, start, end)
	}
	wg.Wait()

	sorted = true
	strict = true
	for c := 0; c < workers; c++ {
		if !chunkSorted[c] {
			return false, false
		}
		if !chunkStrict[c] {
			strict = false
		}
		if c > 0 {
			// First of this chunk must be >= last of previous chunk for the whole
			// slice to be non-decreasing.
			if spanLess(buf, chunkFirst[c], chunkLast[c-1]) {
				return false, false
			}
			if spanEqual(buf, chunkFirst[c], chunkLast[c-1]) {
				strict = false
			}
		}
	}
	return sorted, strict
}
