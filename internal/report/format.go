package report

import "strconv"

// compactInt renders large counts in a short, modern form: 14.34M, 1.20K, 980. This keeps
// the one-line summary narrow and scannable; precise figures remain available via JSON.
func compactInt(n int64) string {
	switch {
	case n >= 1e9:
		return strconv.FormatFloat(float64(n)/1e9, 'f', 2, 64) + "B"
	case n >= 1e6:
		return strconv.FormatFloat(float64(n)/1e6, 'f', 2, 64) + "M"
	case n >= 1e3:
		return strconv.FormatFloat(float64(n)/1e3, 'f', 2, 64) + "K"
	default:
		return strconv.FormatInt(n, 10)
	}
}

// formatBytes renders a byte count in human units (B, KB, MB, GB).
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return strconv.FormatInt(n, 10) + " B"
	}
	div := float64(unit)
	for _, u := range []string{"KB", "MB", "GB", "TB"} {
		if n < int64(div*unit) {
			return strconv.FormatFloat(float64(n)/div, 'f', 2, 64) + " " + u
		}
		div *= unit
	}
	return strconv.FormatInt(n, 10) + " B"
}

// formatRate renders a bytes-per-second throughput in adaptive units (B/s … TB/s). It is used
// by the live progress display, which samples a single atomic byte counter and divides by
// elapsed time — so the value passed in is always bytes/s.
func formatRate(bps float64) string {
	const unit = 1024.0
	if bps < unit {
		return strconv.FormatInt(int64(bps), 10) + " B/s"
	}
	div := unit
	for _, u := range []string{"KB/s", "MB/s", "GB/s", "TB/s"} {
		if bps < div*unit {
			return strconv.FormatFloat(bps/div, 'f', 1, 64) + " " + u
		}
		div *= unit
	}
	return strconv.FormatInt(int64(bps), 10) + " B/s"
}
