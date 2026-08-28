<div align="center">

![GoStyle LOGO](https://i.imgur.com/wveX8z8.png)

</div>
<h4 align="center">Simple, Ultra-fast file handling utility for text deduplication.</h4>
<p align="center">
<img src="https://img.shields.io/github/go-mod/go-version/yourpwnguy/refine">
<!-- <a href="https://github.com/iaakanshff/crtfinder/releases"><img src="https://img.shields.io/github/downloads/iaakanshff/crtfinder/total"> -->
<a href="https://github.com/yourpwnguy/refine/graphs/contributors"><img src="https://img.shields.io/github/contributors-anon/yourpwnguy/refine">
<!-- <a href="https://github.com/iaakanshff/crtfinder/releases/"><img src="https://img.shields.io/github/release/iaakanshff/crtfinder"> -->
<a href="https://github.com/yourpwnguy/refine/issues"><img src="https://img.shields.io/github/issues-raw/yourpwnguy/refine">
<a href="https://github.com/yourpwnguy/refine/stars"><img src="https://img.shields.io/github/stars/yourpwnguy/refine">
<!-- <a href="https://github.com/iaakanshff/crtfinder/discussions"><img src="https://img.shields.io/github/discussions/iaakanshff/crtfinder"> -->
</p>

---

Meet `refine`, a powerful and user-friendly tool for the process of removing duplicate lines from files. `refine` is designed with efficiency in mind, making file manipulation a seamless experience for users. Below is a comprehensive guide on installing and using Seek to enhance your file processing tasks.

## Features 💡

- Efficient file deduplication and management.
- Includes inbuilt sorting.
- Support for diverse input methods.
- Advanced wildcard sorting with exception handling.

## Installation 🛠️
To install the refine tool, you can simply use the following command.
````bash
go install -v "github.com/yourpwnguy/refine/cmd/refine@latest"
cp ~/go/bin/refine /usr/local/bin/
````

## Usage 📘
```yaml
refine - fast line deduplication and sorting

Usage:
  refine [options] <file>                 dedupe & sort a file in place
  refine [options] <in> <out>            if out exists: merge in+out into out;
                                         otherwise: dedupe in, write result to out
  refine -o <out> <in>                   dedupe in, write result to out
  cat <file> | refine [options]           dedupe stdin, print to stdout
  cat <file> | refine [options] <out>    dedupe stdin, write to out
  refine -w <dir> [-e a,b]               dedupe & sort every file in dir

Options:
  -w, --wildcard <dir>        process every regular file in a directory
  -e, --exclude <a,b>         comma-separated base names to skip (with -w)
  -o, --output <file>         write output to this file (overrides default)
  -t, --trim-space            strip leading/trailing whitespace from lines
      --workers <n>           parallelism for sorting (default: all cores)
      --json                  emit machine-readable stats on stderr
      --no-color              disable colored output
  -q, --quiet                 disable the live progress display
  -v, --version               print version and exit
  -h, --help                  show this help and exit

Notes:
  Files are rewritten atomically, so an interrupted run never corrupts the
  original. Lines are compared verbatim; use -t to normalize whitespace.
```

### DIRECT MODE:

Using `refine` to read and write the deduplicated ouptut to the same file:

![refine-direct-mode](https://i.imgur.com/S28T7En.png)


Using `refine` to read from file1 and write the deduplicated output to file2:

![refine-direct-mode](https://i.imgur.com/F6BeuEI.png)


Using `refine` for wildcard sorting (-w), which sorts all files in a directory. This feature is limited to direct mode, as during the tool's development, no use case for the pipeline mode was found.

![refine-direct-mode](https://i.imgur.com/olQvVCD.png)


Using `refine` for wildcard sorting (-w), which sorts all files in a directory except for the specified exceptions. The exceptions, meaning the files to be skipped, can be provided through the -we (wildcard exception) flag with filenames comma-separated.

![refine-direct-mode](https://i.imgur.com/zwUt90r.png)

---

### STDIN MODE:

Using `refine` for sorting the lines from the standard input (stdin). The deduplicated output is displayed on the terminal without modifying the original file. This method is ideal for viewing results without altering the source file.

![refine-direct-mode](https://i.imgur.com/k9Svi6Q.png)


Using the `refine` for sorting the lines from the standard input (stdin), and writes the deduplicated output to a new file specified as an argument. This allows users to create a new file with cleaned data while preserving the original content. `Note:` If the specified file already exists and contains data, it will also be sorted.

![refine-direct-mode](https://i.imgur.com/Oxrbq1K.png)

---

# The long version, and why it is built this way

This part is for people who like to know what is actually happening under the hood. The
quick start above is enough to use the tool. Everything below is the reasoning, the data
structures, and the tradeoffs that shaped version 0.2.0.

I want to be upfront about the goal. `refine` is not trying to be the most feature packed
tool. It is trying to be the fastest honest way to turn a messy text file into a clean,
sorted, duplicate free one, while never corrupting your data and never making you think
about how it works. That single sentence drove most of the decisions in here.

## The mental model: a file is one big buffer

Most line tools think in terms of lines as strings. They read a line, hold it in memory,
compare it, maybe put it in a set, write it back. That model feels natural but it is
expensive. Every line becomes a heap allocation, and sorting a list of strings means
touches for comparison that walk the bytes anyway.

`refine` thinks differently. Once a file is in memory it is just a flat array of bytes.
A line is not an object. A line is a tiny record that says "start at byte 4096, length 11".
That is it. We call that record a `Span`, and the whole pipeline works on spans, never on
copied strings.

```
file bytes:  [ h e l l o \n w o r l d \n ... ]
spans:       [ {start:0, len:5}, {start:6, len:5}, ... ]
```

Because a span is just two small numbers, a file with 14 million lines is just 14 million
of these records. That is a few dozen megabytes of bookkeeping for a 140 megabyte file,
and it lives in one contiguous slice. No per line garbage, no map of strings, no pointers
to chase.

## Why we never move a line

The headline trick is zero copy. On Unix we memory map the input file read only. The byte
buffer the program sees is the kernel page cache itself. There is no `read()` loop copying
the file into a second buffer. We just point at what the OS already has in memory.

This matters more than it sounds. If you refine the same wordlist twice, the second run is
served straight from cache and the only real work left is the logic, not shuttling bytes
around. Even on a cold run, mmap lets the kernel page in only what we touch.

The span records point into that mapped buffer. We never copy a line out to compare it or
to sort it. Sorting reorders the spans, not the bytes. Writing copies each line exactly
once, straight from the map into the output. So a line physically moves from input to
output a single time, at the very end.

On Windows `syscall.Mmap` does not exist, so there we fall back to reading the file fully
into memory. The public behavior is identical. We chose a plain `os.ReadFile` on purpose
so the project keeps zero dependencies. The tradeoff is one extra in process copy and the
loss of the shared cache benefit, which we accept because the hot path was never tuned for
Windows anyway.

## The Span and the eight byte trick

A span is three fields:

```
type Span struct {
    Start  int32  // byte offset of the line's first byte in the backing buffer
    Len    int32  // byte length of the line, excluding the trailing newline
    Prefix uint64 // big-endian uint64 of the line's first 8 bytes (0-padded)
}
```

The interesting field is `Prefix`. While we scan the file we already read the first eight
bytes of every line to find its length, so caching them costs nothing extra. From then on,
comparing two lines almost always means comparing two 64 bit integers instead of walking
the bytes.

Lines that share a prefix still need a real comparison, and there we fall back to a full
byte compare. But the common case, and especially the tie breaking in a sort, collapses to
integer math. When you are doing tens of millions of comparisons, turning most of them into
a single integer compare is the difference between "fast" and "fast enough".

The prefix is stored big endian on little endian machines so that the integer compare
mirrors byte order. That detail is what lets the cached prefix exactly match what a byte by
byte comparison would decide.

## The pipeline, and why only some of it is parallel

The work splits into three stages:

```
extract spans  ->  sort + dedupe  ->  write output
   (parallel)        (serial)          (parallel)
```

Extraction is embarrassingly parallel. We cut the buffer into chunks at newline boundaries
and build spans for each chunk on its own goroutine. The chunks never overlap, so there is
no sharing and no locking. The results just get concatenated because each chunk already
knows its own byte offsets.

Writing is parallel for a different reason. We know the final size up front, so we allocate
one output buffer and hand each worker a disjoint block of lines. Each worker copies its
lines into its own region using a block local prefix sum for the offsets, which means no
locks and no single giant pass over the data. The only shared write is a single atomic
counter that tracks how many bytes have been written, and that exists only so the progress
display can show a speed.

Sorting is deliberately serial. This surprises people, so here is the honest reasoning. A
comparison based sort on spans is already cheap because of the prefix cache, and the real
cost is memory bandwidth and cache misses, not the compare itself. Parallelizing the sort
would mean coordinating many goroutines over a shared array for a win that, in practice, is
smaller than the overhead of the coordination. We use a tuned pattern sort
(`pdqsort` style introsort) on one core and let the stages that actually scale, extract and
write, eat the cores instead. This is the kind of choice you only make after measuring, and
we measured.

## The already sorted fast path

Here is a fact that shaped the whole design. A lot of real inputs, wordlists like
`rockyou.txt` included, are already sorted. If the data is already in order, sorting it
again is pure waste.

So before we sort, we ask a question: is this already sorted? `classifyOrder` checks this
in parallel, one chunk per worker, and then reconciles the chunk boundaries. If every chunk
is sorted and the boundaries line up, we skip the sort entirely and run a single linear
unique pass. The result is byte for byte identical to sorting, but the work drops to a scan
plus a compare against the previous kept line.

On `rockyou.txt` this is the difference between doing real sorting work and basically just
streaming the file. It is the kind of optimization that does nothing for a random file and
everything for the files people actually hand it.

## Merging files without losing your mind

When you give `refine` two or more inputs and ask it to write a combined result, those
inputs are each already sorted by the time we are done reading them. Concatenating and
sorting again would throw away that property. Instead we merge.

A merge takes the already sorted parts and walks them together, always emitting the
smallest next line. We keep one cursor per part in a binary min heap, ordered by the next
line's prefix (with a full byte compare as the tiebreaker). Pop the smallest, write it,
advance that part's cursor, push it back. Repeat until empty.

We wrote the heap by hand instead of using the standard `container/heap` package. That
sounds like the kind of choice that gets you in trouble, but here it is justified: the heap
holds cursors, not spans, so each entry is tiny, and the comparisons happen tens of millions
of times during a big merge. The interface dispatch in `container/heap` shows up in profiles
at that scale, while a direct slice based heap does not. The code is a little more verbose
and that is a price we happily pay for the throughput.

There is also a two part special case. Merging exactly two sorted runs is so common and so
simple that we handle it with a dedicated routine that needs no heap at all, just two
pointers and a shared output buffer. It is shorter, it is faster, and it covers the
majority of real merges.

## Writing the output without corrupting anything

The single scariest thing a tool like this can do is halfway write a file and then crash,
leaving you with neither the original nor a valid result. We refuse to do that.

Every write goes to a temporary file created in the same directory as the destination. We
fill it, flush it, and only then rename it over the target. The rename is atomic on a
single filesystem, which means anything watching the file path either sees the old content
or the new content, never a partial write. If we die mid way, the original is completely
untouched because we had not replaced it yet.

The temp file lives in the same directory on purpose. An atomic rename only works within
one filesystem, so a temp file on a different mount could not be renamed into place. Keeping
it local is what makes the guarantee real.

For in place mode the source becomes the destination only at that final rename. For output
mode we write a brand new file and never touch the source. For merge the combined output is
also a fresh file until the rename.

## Modes: in place, output, and merge

The command line tries to match what a person means, not what a parser finds convenient.
One argument means refine the file in place. Two arguments is where it gets interesting,
and `cli.Parse` decides between two readings based on whether the second path already
exists:

- If `out` does not exist yet, you clearly want to produce a new file. We read `in`, refine
  it, and write `out`. The source stays exactly as it was.
- If `out` does exist, you most likely want to combine `in` and `out` into `out`. That is
  the merge path.
- The `-o` flag makes the intent explicit and always means output mode.

This small existence check encodes a surprising amount of intent. It means `refine a b`
does what you expect whether `b` is a destination you are creating or a sibling you are
folding in. We could have forced separate flags for everything, but matching intuition
keeps the common cases one command long.

Stdin mode follows the same spirit. `cat file | refine` prints to the terminal. `cat file |
refine out.txt` writes to a file instead, without modifying the piped source.

## Wildcard mode and bounding the chaos

`refine -w dir` processes every file in a directory, each refined in place. It is the kind
of feature that is trivial to describe and easy to do badly. The trap is opening and mapping
every file at once, which thrashes the page cache and the scheduler the moment a directory
has thousands of entries.

So wildcard mode is bounded. A semaphore sized to your worker count limits how many files
are in flight. Each file is still scanned and written with the same fast internal path, and
exceptions let you skip named files. The key point is that the per file work stays correct
and isolated while the concurrency stays under control. We would rather finish a huge
directory a little slower than fall over trying to do all of it at once.

## The UI: one line, honest numbers

The output is a single line, and that is on purpose.

```
✦ target.txt · 14.34M lines · 133.46 MB · 0 dup · 473ms
```

A blank line sits above it so it does not get buried in scrollback. The line tells you the
thing that mattered: what file, how many lines survived, how big it was, how many duplicates
we dropped, and how long it took.

During the run there is a live status line on stderr. It has two faces. For the scan,
dedupe, and merge phases we show a spinner, because those phases are memory bound and do not
have a stable denominator to count toward. A progress bar that says 40 percent of an unknown
amount is a lie, so we do not show one. For the write phase we show a bar with a percentage
and a speed, because now we know the exact byte total and the speed is the one number worth
watching.

The speed is computed from elapsed time since the run started and smoothed a little so it
does not jump around. We deliberately show throughput only where it is real. On a non
terminal, under `--quiet`, or in `--json` mode, the whole live display is disabled and you
get either silence or structured data.

## How fast, and why

On a cached run with `rockyou.txt` (about 140 megabytes, 14.34 million lines, already
unique and sorted) the numbers we see are:

- in place single file: roughly 470 milliseconds
- output to a new file: roughly 460 milliseconds
- merge of two such files: roughly 850 milliseconds

These are not microbenchmark tricks. They are the real tool on real data, and the output is
byte for byte identical to `LC_ALL=C sort -u` on the same input.

The speed comes from stacking small honest wins rather than one big one:

- mmap means the file is never copied on the way in.
- the 8 byte prefix cache turns most comparisons into integer math.
- extraction and writing are parallel and touch each byte once.
- the sorted fast path skips sorting entirely when it can.
- the output is one contiguous buffer written with a single system call per worker, with one
  atomic add for progress so the display never stalls the writer.

Compared with the classic `sort | uniq` shell pipeline, `refine` avoids spawning processes,
avoids the pipe copy between them, and avoids allocating a string per line. It is the same
algorithm family, just without the ceremony.

## Correctness is not optional

Being fast is worthless if the answer is wrong, so the ordering is defined precisely. We
sort by unsigned byte value, comparing the 8 byte prefix first and falling back to a full
byte compare for ties. That is exactly the C locale byte order that `sort` uses under
`LC_ALL=C`. Because equal lines are identical, keeping one of them is stable by definition,
and the merged or sorted result matches what a careful `sort -u` would produce.

We back that claim with tests rather than vibes. There is a fuzz test,
`FuzzDeduplicate`, that throws arbitrary generated inputs at the core and asserts the output
is always sorted and always free of duplicates. Run it for a while and it hammers millions
of cases looking for a violation. On top of that the continuous integration runs `go test
-race` so concurrency bugs in the parallel stages get caught, and an end to end check
compares a large real run against `sort -u` to confirm byte identity.

## Caveats and sharp edges

A few things to know so the tool never surprises you badly:

- Line length is held in a 32 bit field, so a single line longer than about two gigabytes
  is not supported. For text files this is not a real limit.
- Ordering is C locale byte order. If you need UTF-8 collation or a specific language sort,
  this is not the tool for that job. It sorts bytes, predictably.
- Merge assumes its inputs are individually sorted, which they are after `refine` writes
  them, so merging previously refined files is safe.
- On Windows the input is read fully into memory instead of mapped. Same result, a bit more
  memory.
- Because we map the whole file, huge inputs use address space, not physical RAM. The OS
  pages the data in as we touch it, which is exactly what we want.

## Project layout

The code is split into small packages with clear boundaries so each piece can be read on its
own:

- `cmd/refine` is the thin entry point. It parses arguments, sets up styling, and hands off.
- `internal/cli` owns argument parsing and the mode decision.
- `internal/core` owns the real work: span extraction, comparison, sorting, and dedupe.
- `internal/store` owns bytes: reading (mmap or stdin), atomic writing, and the merge.
- `internal/report` owns the result shape, the JSON and human rendering, and the live
  progress display.
- `internal/engine` is the conductor. It picks a mode and wires core, store, and report
  together.
- `internal/style` and `internal/term` are tiny helpers for color and terminal detection.

## Building, CI, and releases

If you cloned the source, the `Makefile` covers the everyday tasks:

```
make build     # compile the refine binary
make test      # run the unit tests
make test-race # run the tests with the race detector
make lint      # run golangci-lint
make fuzz      # fuzz the dedupe core (override FUZZTIME)
make bench     # run the benchmarks
```

There is a `golangci-lint` config, a `goreleaser` config for cross compiled binaries, a
continuous integration workflow that lints and race tests on every push, a release workflow
that builds and publishes on version tags, and a Dependabot config that keeps the actions
and any future dependencies current. The project has zero runtime dependencies, which is why
the build is just a `go build`.

## But why use our tool

I know there are already popular tools out there, like tomnomnom's `anew`, and they are
great. `refine` is built from a different angle. Instead of layering features on top of a
string per line model, it starts from the buffer and the span, which is what makes the
sorted fast path and the parallel write possible in the first place. The wildcard mode with
exceptions, the merge of existing files, and the honest single line report are all
consequences of that same foundation.

If you process wordlists, logs, or any large pile of text lines and you care that the job
is fast, safe, and predictable, this is built for exactly that.

## Contributing 🤝

Contributions are welcome. If you have suggestions, bug reports, or feature requests, feel
free to open an issue or submit a pull request. The codebase is small on purpose, so it is
easy to follow where a change should go.
