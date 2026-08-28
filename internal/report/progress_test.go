package report

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// TestProgressDraw verifies the live counter renders a spinner, an optional
// progress bar + percentage, and an adaptive throughput readout. The UI writes to
// the shared buffer from its own goroutine, so we only read the buffer after Done
// has stopped that goroutine — reading concurrently would be a data race.
func TestProgressDraw(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, true)

	// Indeterminate (no total) phase: spinner only, no bar.
	p.Set("scan file.txt", 0)
	time.Sleep(120 * time.Millisecond)

	// Determinate (write) phase: bar + percentage + throughput.
	var counter int64 = 512 << 20 // 512 MiB processed
	p.Set("write file.txt", 1024<<20)
	p.UseCounter(&counter)
	time.Sleep(120 * time.Millisecond)

	p.Done() // stops the UI goroutine; the buffer is now safe to read
	out := buf.String()

	if !strings.Contains(out, "scan file.txt") {
		t.Fatalf("expected scan phase label in output, got %q", out)
	}
	if !strings.Contains(out, "%") {
		t.Fatalf("determinate phase must show a percentage, got %q", out)
	}
	if !strings.Contains(out, "MB/s") && !strings.Contains(out, "GB/s") {
		t.Fatalf("determinate phase must show a throughput, got %q", out)
	}
	if !strings.Contains(out, "▕") {
		t.Fatalf("determinate phase must show a bar, got %q", out)
	}
}

// TestRenderHumanOneLine ensures the final human report is a single line with
// the expected fields and never prints multiple newlines.
func TestRenderHumanOneLine(t *testing.T) {
	s := &Summary{
		Results:        []Result{{Target: "t.txt", TotalLines: 14_344_187, DuplicateLines: 0, BytesWritten: 133_460_000, Duration: 370 * time.Millisecond}},
		FilesProcessed: 1,
		TotalLines:     14_344_187,
		DuplicateLines: 0,
		BytesWritten:   133_460_000,
		Duration:       370 * time.Millisecond,
	}
	var buf bytes.Buffer
	RenderHuman(s, &buf)
	got := buf.String()
	if strings.Count(got, "\n") != 2 {
		t.Fatalf("expected a leading blank line plus one content line, got %q", got)
	}
	for _, want := range []string{"t.txt", "lines", "dup", ""} {
		if want != "" && !strings.Contains(got, want) {
			t.Fatalf("one-liner missing %q: %q", want, got)
		}
	}
}
