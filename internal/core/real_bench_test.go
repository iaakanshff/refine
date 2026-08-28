package core

import (
	"os"
	"testing"
)

func benchExtract(b *testing.B, trim bool) {
	path := os.Getenv("ROCKYOU")
	if path == "" {
		b.Skip("set ROCKYOU to a path to benchmark on real data")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(raw)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExtractSpans(raw, trim)
	}
}

func benchDedup(b *testing.B, trim bool) {
	path := os.Getenv("ROCKYOU")
	if path == "" {
		b.Skip("set ROCKYOU to a path to benchmark on real data")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(raw)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		spans := ExtractSpans(raw, trim)
		_ = Deduplicate(raw, spans, 1)
	}
}

func BenchmarkRealExtract(b *testing.B) { benchExtract(b, false) }
func BenchmarkRealDedup(b *testing.B)   { benchDedup(b, false) }
