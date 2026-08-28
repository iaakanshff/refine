# refine — Architecture

`refine` is a fast, concurrent command-line tool for deduplicating and lexicographically
sorting line-oriented text (wordlists, logs, CSV-ish row dumps, etc.). Given `N` input
lines it produces the same result as `LC_ALL=C sort -u`, typically several times faster,
while keeping stdout free for piped data and writing reports to stderr.

This document explains the final architecture: the package boundaries, the data and control
flow, the core algorithms, the deliberate performance decisions, and the operational caveats.

---

## 1. Design principles

These constraints shaped every package boundary below:

- **Zero external dependencies.** The module imports only the standard library. Every third-party
  package is attack surface to audit and keep patched; for a focused CLI the stdlib is enough.
- **In-memory, single-buffer model.** `refine` reads each input into one contiguous `[]byte` and
  reasons about lines as *offsets into that buffer*. This is what makes the hot loops cache-friendly
  and allocation-free. It implies a practical ceiling of ~2 GB per input file (see §7).
- **Presentation is isolated from logic.** The dedup engine never formats text or touches a terminal;
  the reporter never reads a file. This lets the UI change without touching the pipeline, and lets the
  core be fuzz-tested in isolation.
- **Parallelize where there is no shared state; serialize where there is.** File-level work (scanning,
  per-file dedup, directory fan-out) is parallel across a worker pool. Within a single file the sort is
  serial by measurement, not by omission (see §5).
- **Atomic writes, always.** A run that is interrupted, SIGKILLed, or hits a full disk must never leave
  a truncated original behind.

---

## 2. Package map and dependency direction

```
              main  (cmd/refine)
                |
        +-------+-------+-------+
        |       |       |       |
      cli    engine   report   style
        |       |       |        |
        |    +--+--+    |        |
        |    |     |    |        |
        |  core  store  |        |
        |    |     |    |        |
        +----+-----+----+--------+
             |
            term
```

- `main` is a thin wiring layer: parse → configure style → run → render. No logic.
- `cli` turns `os.Args` into a `Config`. It knows nothing about I/O or dedup.
- `engine` is the only package that knows about `cli`, `core`, `store`, and `report` at once. It
  orchestrates the run modes.
- `core` is pure: spans, comparison, sort, dedup. No I/O, no color, no globals.
- `store` owns bytes: mmap reads, atomic writes, the parallel stream writer, the k-way merge, directory
  listing. It imports `core` (for the `Span` type) but nothing higher.
- `report` renders a `Summary` to a human one-liner or JSON, and owns the live progress display. Imports
  `style`.
- `style` is a tiny ANSI helper, and `term` is a tiny TTY probe. Both are leaves.

There are no import cycles. Dependency direction is strictly downward.

---

## 3. Data model: `core.Span`

A line is represented as:

```go
type Span struct {
    Start  int32
    Len    int32
    Prefix uint64 // big-endian uint64 of the line's first 8 bytes (0-padded)
}
```

**Why offsets instead of `[]byte`/`string`:** a `[]byte` header is 24 bytes; this 16-byte record with a
cached prefix is ~33% smaller. At 14 M lines that is the difference between ~336 MB and ~224 MB of
bookkeeping. More importantly, sorting a slice of small, fixed-size, contiguous records is cache-friendly;
the only cold access during a comparison is the actual byte payload, and the prefix removes that access in
the common case.

**Why a prefix:** `Prefix` is the first 8 bytes of the line as a big-endian `uint64`. Because it is
big-endian, the integer's numeric order equals the lexicographic order of those bytes, so comparing two
prefixes is a correct proxy for comparing the lines whenever they differ early. The full byte comparison
(`bytes.Compare`) only runs when two lines genuinely share their first 8 bytes, which is rare for real
wordlists. This turns the dominant comparison in the sort from a random walk into a ~280 MB buffer into a
cheap `uint64` compare — the single biggest hot-loop win.

**Invariant:** a `Span` is valid only while its backing buffer is alive and unmodified. The caller owns the
buffer (typically an mmap view that is released after use).

---

## 4. Control & data flow

```
input file ──▶ store.ReadFileMmap ──▶ []byte (shared page cache)
                                       │
                          core.ExtractSpans (parallel scan)
                                       │  []core.Span   (zero-copy offsets)
                          core.Deduplicate
                            ├─ classifyOrder  (parallel already-sorted check)
                            ├─ SortSpans      (serial pdqsort, prefix shortcut)
                            └─ compactUnique  (in-place linear compaction)
                                       │  unique []core.Span
                  store.WriteSpans / store.WriteMerged
                            ├─ parallel per-block copy into one output buffer
                            └─ single Write to os.Stdout or an atomic temp file
                                       │
                              store.OpenAtomic commit (fsync + rename)
```

The engine turns the resulting `report.Result`(s) into a `report.Summary`, which `report` renders.

---

## 5. Core algorithms

### 5.1 Extraction (`ExtractSpans`, `internal/core/extract.go`)
A single forward scan splits the buffer at `\n`, building one `Span` per line. The capacity is guessed from
buffer size to avoid most reallocation. When trimming is off, a single trailing `\r` is still stripped so
CRLF files behave like LF. Blank lines are preserved as zero-length spans (user data is never silently
dropped). Extraction is parallelized inside `core.ExtractSpans`, which partitions the buffer at newline
boundaries and runs the per-chunk scan across the worker pool; offsets are rebased by the chunk start so the
results concatenate into one valid slice.

### 5.2 The already-sorted fast path (`classifyOrder`, `internal/core/order.go`)
Many real inputs (e.g. `rockyou.txt`) are already sorted. `classifyOrder` checks this in parallel across the
worker pool, examining each chunk independently and reconciling chunk boundaries. If the spans are already
**strictly increasing**, the O(N log N) sort and the compaction are both skipped. For genuinely unsorted
input the check fails within a handful of comparisons, so it costs almost nothing on the general case.

### 5.3 Sort + compact (`SortSpans`, `compactUnique`, `internal/core/sort.go`, `dedup.go`)
When sorting is needed we use a single serial `sort.Sort` (pdqsort) over a typed `sort.Interface`
(`spanSlice`). Two deliberate decisions here:

- **Serial, not parallel sort.** A parallel bucket-sort across 12 cores was measured at ~964 ms vs ~947 ms
  serial on a 140 MB / 14.3 M-line file; a chunk-sort + k-way-merge was *slower* (2.66 s) because the serial
  merge dominated. The work is memory-bandwidth bound — every goroutine hammers the same backing buffer, so
  more cores just add cache-line contention. Real parallelism in `refine` lives at the file level (§6), where
  there is no shared buffer to contend on.
- **Typed `Swap`, not `sort.Slice`.** `sort.Slice` swaps via reflection on every one of the hundreds of
  millions of swaps a large merge needs; the typed `Swap` removes that per-call cost (the difference between
  ~11 s and ~2 s on a 28 M-line merge).

Deduplication is then a single linear `compactUnique` pass: because the slice is sorted, duplicates are
contiguous, so no hashing or map is required. (A map was rejected: hashing 140 MB per run, bucket overhead,
and GC churn from map entries all lose to sort+compact here.)

### 5.4 Merge (`WriteMerged` / `mergeTwo`, `internal/core/...` → `internal/store/merge.go`)
When several already-sorted, already-unique parts must be combined, `WriteMerged` does a k-way heap merge.
Because the CLI caps merges at two inputs, the common case is handled by `mergeTwo`, a direct two-pointer
walk that advances *both* cursors when the same line appears in both inputs — halving the work for the frequent
"combine two overlapping wordlists" pattern. The heap itself (`mergeHeap`, `internal/store/mergeheap.go`) is
hand-rolled rather than `container/heap` to avoid per-operation interface dispatch, which dominates once the
merge runs over tens of millions of spans. Comparisons reuse the cached `Prefix` exactly as the sort does.

---

## 6. Engine: run modes and concurrency

`engine.Run` dispatches on the `Config`:

| Invocation | Mode | Behavior |
|---|---|---|
| `cat f \| refine` | stdin | read stdin → dedup → stdout |
| `cat f \| refine out` | stdin→file | read stdin → dedup → atomic `out` |
| `refine f` | in place | mmap `f` → dedup → atomic `f` |
| `refine in out` (out missing) | **output** | mmap `in` → dedup → atomic `out`; `in` untouched |
| `refine in out` (out exists) | **merge** | dedup both → merge → atomic `out` |
| `refine -o out in` | output (explicit) | dedup `in` → atomic `out`; `in` untouched |
| `refine -w dir` | wildcard | dedup every file in `dir` in parallel, in place |

The `in out` disambiguation (output vs merge) is decided in `cli.Parse` by checking whether `out` already
exists, which matches user intent: "write the result somewhere new" vs "combine two existing files".

**Concurrency model.** `runWildcard` and `runMerge` fan independent files across a bounded worker pool
(`cfg.Workers` semaphore). Within a file, scanning is parallel (`core.ExtractSpans`) but the per-file
sort/dedup stays serial (§5.3). The merge phase is a single linear, cache-friendly scan of each
already-sorted part. The shared atomic-write progress counter is single-slot, so the wildcard loop reports
per-file completion via `Progress.Add` rather than a per-file byte counter.

---

## 7. Store: I/O and performance

- **Zero-copy reads (`ReadFileMmap`).** On Unix, files are `mmap`ed read-only, sharing the kernel page
  cache. This avoids the extra copy `os.ReadFile` would make — a direct, measurable win for inputs that are
  already cached (the common case when refining the same wordlist repeatedly). The empty file is handled
  explicitly because `mmap` rejects a zero-length mapping. The returned `release` (`Munmap`) is called once
  the spans are consumed. On Windows, where `syscall.Mmap` is unavailable, `ReadFileMmap` falls back to a
  full in-memory read (`os.ReadFile`); the API and release contract are identical, so callers are unchanged.
- **Parallel stream writer (`WriteSpans`).** The output is assembled in one contiguous buffer and written with
  a single `Write`. The per-line copy is parallelized across cores: each worker fills its own block using a
  block-local prefix sum, avoiding both the serial per-line append overhead (which dominated at 14 M lines)
  and a single giant prefix-sum pass. `bytesWritten` is bumped with one atomic add, so the live progress
  display can sample throughput without the writer ever blocking.
- **Atomic writes (`OpenAtomic`).** A temp file is created in the *same directory* as the destination (so the
  final rename is on the same filesystem and atomic), written, `fsync`ed, then renamed over the target. The
  original is untouched until the new content is fully durable; readers never observe a half-written file.
- **Directory listing (`ListFiles`).** Returns regular files sorted by name for deterministic ordering,
  skipping excluded base names and non-regular entries (directories, symlinks). A race with another process
  removing a file is tolerated by skipping it rather than failing.

---

## 8. Reporting and UI (`internal/report`)

- **One-line summary (`RenderHuman`).** The final report is a single, colorized line:
  `✦ rockyou.txt · 14.34M lines · 133.46 MB · 0 dup · 473ms`, prefixed with a blank line so it is visually
  separated from any prior progress output. Counts use a compact form (`14.34M`); precise figures remain in
  the JSON view. stdout is never touched, so piped line data is unaffected.
- **Live progress (`Progress`).** A single goroutine driven by a 50 ms ticker draws to stderr with a carriage
  return + clear-to-end-of-line, so it overwrites itself and never smears into piped data. It shows a braille
  spinner for indeterminate phases (`scan`, `dedupe`, `merge`) and a block bar + percentage + adaptive
  throughput (`MB/s`) for the `write` phase. All cross-goroutine state is a couple of atomics plus a
  mutex-guarded metadata struct; the writer hot path never waits on the UI. The display is disabled for
  non-TTY, `--quiet`, and `--json` destinations.
- **JSON (`RenderJSON`).** A machine-readable view on stderr; durations are integer milliseconds for easy
  parsing.

Color is gated by `style.Configure`, which respects `--no-color`, non-TTY output, and the `NO_COLOR`
environment variable (https://no-color.org), so `refine` is safe in pipes and scripts.

---

## 9. Tradeoffs and operational caveats

- **In-memory model.** Each input is held whole in RAM. Inputs larger than available memory (or larger than
  ~2 GB per file, the `int32` offset limit) are out of scope; such cases call for an external merge sort,
  which would be a different tool.
- **Single-buffer sort is bandwidth bound.** Throwing more cores at one file's sort does not help (§5.3);
  parallelism is at the file level instead.
- **Atomicity.** Output goes through a temp file + rename, so an interrupted run never corrupts the original.
  The temp file lives in the destination's directory; a full disk mid-write fails the commit but leaves the
  original intact.
- **Merge semantics.** `refine in out` merges when `out` exists and overwrites/outputs otherwise; use `-o` for
  an explicit, unambiguous destination.
- **Lines are compared verbatim** (byte order, `LC_ALL=C` semantics implied by `bytes.Compare`); pass `-t` to
  normalize ASCII whitespace first.
- **Determinism.** Output is a stable sort of unique lines; for fully-unique input the order equals the
  canonical sorted order, which is why it matches `LC_ALL=C sort -u`.

---

## 10. Testing

- `internal/core/core_test.go` — extraction, trimming, CRLF, correctness, and a serial-vs-parallel
  equivalence check.
- `internal/core/core_fuzz_test.go` — `FuzzDeduplicate` asserts the output is always sorted and duplicate-free
  for arbitrary input (including empty, all-newline, and unterminated-last-line cases). This is the regression
  net for "crashes on malformed input" bugs.
- `internal/engine/*_test.go` — in-place, merge, output-mode, and wildcard runs against temp files, asserting
  no leftover temp files. `internal/cli/*_test.go` asserts the `in out` vs `-o` disambiguation.
- `internal/report/*_test.go` — the one-line summary shape and the progress renderer (read only after the UI
  goroutine is stopped, to avoid a data race on the shared buffer).
- `internal/core/real_bench_test.go` — benchmarks gated on the `ROCKYOU` env var for profiling on real data.
  Run with `go test -race ./...` in CI; fuzzing is run separately via `go test -fuzz`.
