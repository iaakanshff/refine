package report

import (
	"encoding/json"
	"io"
)

// RenderJSON writes the summary as indented JSON to w. Stats go to stderr in normal usage
// so stdout stays clean for piped line data.
//
// Durations are emitted as integer milliseconds rather than a Go Duration string because a
// stable, obvious unit is easier for scripts to parse and compare across runs.
func RenderJSON(s *Summary, w io.Writer) error {
	type jsonResult struct {
		Target         string `json:"target"`
		TotalLines     int64  `json:"total_lines"`
		UniqueLines    int64  `json:"unique_lines"`
		DuplicateLines int64  `json:"duplicate_lines"`
		BytesWritten   int64  `json:"bytes_written"`
		DurationMs     int64  `json:"duration_ms"`
	}
	type jsonSummary struct {
		Results        []jsonResult `json:"results"`
		DurationMs     int64        `json:"duration_ms"`
		FilesProcessed int          `json:"files_processed"`
		TotalLines     int64        `json:"total_lines"`
		UniqueLines    int64        `json:"unique_lines"`
		DuplicateLines int64        `json:"duplicate_lines"`
		BytesWritten   int64        `json:"bytes_written"`
	}
	out := jsonSummary{
		Results:        make([]jsonResult, 0, len(s.Results)),
		DurationMs:     s.Duration.Milliseconds(),
		FilesProcessed: s.FilesProcessed,
		TotalLines:     s.TotalLines,
		UniqueLines:    s.UniqueLines,
		DuplicateLines: s.DuplicateLines,
		BytesWritten:   s.BytesWritten,
	}
	for _, r := range s.Results {
		out.Results = append(out.Results, jsonResult{
			Target:         r.Target,
			TotalLines:     r.TotalLines,
			UniqueLines:    r.UniqueLines,
			DuplicateLines: r.DuplicateLines,
			BytesWritten:   r.BytesWritten,
			DurationMs:     r.Duration.Milliseconds(),
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
