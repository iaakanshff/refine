package report

import (
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/yourpwnguy/refine/internal/style"
)

// RenderHuman writes a single, compact, colorized status line to w. Keeping the final
// report to one line (instead of a multi-line block) makes it read cleanly in a terminal
// and stay unobtrusive in captured logs. stdout is never touched, so piped line data is
// unaffected.
//
// A blank line is emitted before the report so the summary is visually separated from any
// preceding progress output or piped data.
func RenderHuman(s *Summary, w io.Writer) {
	var b strings.Builder
	b.WriteString(style.Bold(style.Cyan("✦")))
	b.WriteString(" ")
	if len(s.Results) == 1 {
		b.WriteString(style.Bold(style.Yellow(s.Results[0].Target)))
	} else {
		b.WriteString(style.Bold(style.Yellow(strconv.Itoa(len(s.Results)) + " files")))
	}

	b.WriteString(style.Dim(" · "))
	b.WriteString(style.Bold(compactInt(s.TotalLines)))
	b.WriteString(style.Dim(" lines"))

	b.WriteString(style.Dim(" · "))
	b.WriteString(style.Bold(formatBytes(s.BytesWritten)))

	b.WriteString(style.Dim(" · "))
	b.WriteString(style.Bold(style.Red(compactInt(s.DuplicateLines))))
	b.WriteString(style.Dim(" dup"))

	b.WriteString(style.Dim(" · "))
	b.WriteString(style.Bold(style.Green(s.Duration.Round(time.Millisecond).String())))

	b.WriteString("\n")
	_, _ = w.Write([]byte("\n" + b.String()))
}
