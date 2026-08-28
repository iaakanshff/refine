package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseOutputMode(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.txt")
	out := filepath.Join(dir, "out.txt") // intentionally absent
	if err := os.WriteFile(in, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// `refine in.txt out.txt` with out.txt missing means output mode: only the
	// first file is read, and the (new) second file is the destination.
	cfg, err := Parse([]string{in, out})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Inputs) != 1 || cfg.Inputs[0] != in {
		t.Fatalf("Inputs = %v, want [%q]", cfg.Inputs, in)
	}
	if cfg.Output != out {
		t.Fatalf("Output = %q, want %q", cfg.Output, out)
	}
}

func TestParseMergeMode(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.txt")
	out := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(in, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Both files exist: this is merge mode (reads both, writes into out.txt).
	cfg, err := Parse([]string{in, out})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Inputs) != 2 {
		t.Fatalf("Inputs = %v, want both files", cfg.Inputs)
	}
	if cfg.Output != out {
		t.Fatalf("Output = %q, want %q", cfg.Output, out)
	}
}

func TestParseExplicitOutputFlag(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.txt")
	if err := os.WriteFile(in, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// `-o out.txt in.txt` is output mode regardless of whether out.txt exists.
	cfg, err := Parse([]string{"-o", filepath.Join(dir, "out.txt"), in})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Inputs) != 1 || cfg.Inputs[0] != in {
		t.Fatalf("Inputs = %v, want [%q]", cfg.Inputs, in)
	}
}
