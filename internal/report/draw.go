package report

import (
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/yourpwnguy/refine/internal/style"
)

// spinnerFrames is a smooth braille spinner cycled by elapsed time. It is purely cosmetic: it
// tells the user the process is alive during the brief phases that have no meaningful progress
// fraction (scan, dedupe, merge).
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// draw renders one frame of the live status into buf and writes it. It is called by the single
// UI goroutine; it reads shared state under a short-lived lock and never blocks on the pipeline.
func (p *Progress) draw(buf []byte) {
	p.mu.Lock()
	msg := p.msg
	total := p.total
	c := p.counter
	p.mu.Unlock()

	var cur int64
	if c != nil {
		cur = atomic.LoadInt64(c)
	} else {
		cur = p.current.Load()
	}

	elapsed := time.Since(p.start)
	frame := spinnerFrames[int(elapsed/(70*time.Millisecond))%len(spinnerFrames)]

	buf = buf[:0]
	buf = append(buf, '\r')

	// Spinner in an accent color, then the phase label in dim.
	buf = append(buf, style.Bold(style.Cyan(frame))...)
	buf = append(buf, ' ')
	buf = append(buf, style.Dim(msg)...)

	if total > 0 {
		pct := float64(cur) / float64(total)
		if pct < 0 {
			pct = 0
		} else if pct > 1 {
			pct = 1
		}
		buf = append(buf, ' ')
		buf = append(buf, renderBar(pct, 14)...)
		buf = append(buf, ' ')
		buf = strconv.AppendInt(buf, int64(pct*100), 10)
		buf = append(buf, '%')
	}

	if c != nil && cur > 0 && elapsed > 0 {
		rate := float64(cur) / elapsed.Seconds()
		buf = append(buf, style.Dim("   ·   ")...)
		buf = append(buf, style.Green(formatRate(rate))...)
	}

	// Clear to end of line so stale wider messages don't linger.
	buf = append(buf, "\033[K"...)
	// The write is to a terminal; a broken pipe is non-actionable here.
	_, _ = p.w.Write(buf)
}

// renderBar draws a fixed-width block bar (▕█████░░▏) for the given fraction.
func renderBar(pct float64, width int) string {
	filled := int(pct * float64(width))
	if filled < 0 {
		filled = 0
	} else if filled > width {
		filled = width
	}
	var b strings.Builder
	b.WriteString("▕")
	for i := 0; i < width; i++ {
		if i < filled {
			b.WriteString("█")
		} else {
			b.WriteString("░")
		}
	}
	b.WriteString("▏")
	return b.String()
}
