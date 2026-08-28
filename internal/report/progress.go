package report

import (
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// Progress is a lightweight, non-blocking live status display.
//
// Design goals (the brief asked for "live updating" without it ever becoming a
// bottleneck):
//
//   - All UI work happens on ONE goroutine driven by a 50 ms ticker. It never contends with
//     the dedup/write hot paths.
//   - The only cross-goroutine communication is a couple of atomics and a mutex-guarded
//     metadata struct. The writer loop never waits on the UI.
//   - Throughput is sampled, not counted per line: WriteSpans bumps a single atomic counter
//     and the ticker reads it, so a 14M-line file costs ~zero extra per line.
//   - Output goes to stderr with a carriage return + clear-to-end-of-line, so it overwrites
//     itself in place and never pollutes stdout (which may be carrying piped line data).
//
// If enabled is false (quiet mode, JSON mode, or non-TTY), every method is a no-op and no
// goroutine is spawned.
type Progress struct {
	// mu guards msg, total and counter — the only mutable metadata. The draw loop
	// reads them under mu; the hot paths (Set/UseCounter) write them under mu.
	mu sync.Mutex
	// w is where the live status is drawn. It is always os.Stderr so piped stdout
	// data is never touched.
	w io.Writer
	// msg is the current phase label, rendered in dim after the spinner.
	msg string
	// total is the expected maximum for the counter. A value of 0 means the phase
	// is indeterminate (e.g. scan/dedupe), which renders as a spinner with no bar.
	total int64
	// counter, when non-nil, is an external atomic counter (e.g. bytes written)
	// that the ticker samples instead of the internal one. Only one may be attached.
	counter *int64

	// current is the internal progress counter, bumped by Add and reset to zero by Set.
	current atomic.Int64
	// start is when the run began; it drives both the spinner animation frame and
	// the throughput computation.
	start time.Time
	// stop is closed by Done to ask the UI goroutine to exit.
	stop chan struct{}
	// done is closed by the UI goroutine once it has fully stopped, so Done can
	// block until the line is cleared.
	done chan struct{}
	// enabled gates everything: when false every method is a no-op and no goroutine
	// is spawned (quiet / JSON / non-TTY).
	enabled bool
}

// NewProgress starts the ticker goroutine if enabled. w is where the live status is drawn; it
// should be os.Stderr so piped stdout data is never touched.
func NewProgress(w io.Writer, enabled bool) *Progress {
	p := &Progress{
		w:       w,
		start:   time.Now(),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
		enabled: enabled,
	}
	if enabled {
		go p.run()
	}
	return p
}

// Set transitions to a new phase. total is the expected maximum for the counter (0 means
// "unknown / don't show a fraction", which renders as an indeterminate spinner). The internal
// progress counter is reset so per-phase fractions start at zero.
func (p *Progress) Set(phase string, total int64) {
	if !p.enabled {
		return
	}
	p.mu.Lock()
	p.msg = phase
	p.total = total
	p.mu.Unlock()
	p.current.Store(0)
}

// Add increments the counter, used for discrete units such as "files done" in wildcard mode.
func (p *Progress) Add(n int64) {
	if !p.enabled {
		return
	}
	p.current.Add(n)
}

// UseCounter attaches an external atomic counter (e.g. bytes written) that the ticker will
// sample instead of the internal counter. Only one counter may be attached at a time; call it
// right before the write phase begins. This is safe only with a single in-flight write (not in
// the concurrent wildcard loop), because the throughput readout assumes one active counter.
func (p *Progress) UseCounter(c *int64) {
	if !p.enabled {
		return
	}
	p.mu.Lock()
	p.counter = c
	p.mu.Unlock()
}

// Done stops the ticker and clears the current line so subsequent output is not visually smeared
// by a leftover status.
func (p *Progress) Done() {
	if !p.enabled {
		return
	}
	close(p.stop)
	<-p.done
	// Erase the status line on exit; the write target is a terminal and the error is non-actionable.
	_, _ = p.w.Write([]byte("\r\033[K"))
}

// run is the single UI goroutine. It draws on a fixed interval and exits when Done is called.
func (p *Progress) run() {
	defer close(p.done)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	// buf is reused across ticks to avoid allocating a fresh slice 20 times a second.
	buf := make([]byte, 0, 160)
	for {
		select {
		case <-p.stop:
			return
		case <-ticker.C:
			p.draw(buf)
		}
	}
}
