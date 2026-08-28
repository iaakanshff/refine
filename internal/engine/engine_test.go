package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yourpwnguy/refine/internal/cli"
)

func TestRunFileInPlace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("b\na\nb\nc\na\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := cli.Config{Inputs: []string{path}, Output: path, Workers: 2, Quiet: true}
	sum, err := Run(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if sum.FilesProcessed != 1 {
		t.Fatalf("processed %d files, want 1", sum.FilesProcessed)
	}
	if sum.UniqueLines != 3 || sum.DuplicateLines != 2 {
		t.Fatalf("unique=%d dupes=%d, want 3/2", sum.UniqueLines, sum.DuplicateLines)
	}

	got, _ := os.ReadFile(path)
	if string(got) != "a\nb\nc\n" {
		t.Errorf("output = %q, want %q", got, "a\nb\nc\n")
	}
	assertNoTemp(t, dir)
}

func TestRunFileMerge(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.txt")
	out := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(in, []byte("x\ny\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, []byte("y\nz\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// `refine in.txt out.txt` merges both into out.txt.
	cfg := cli.Config{Inputs: []string{in, out}, Output: out, Workers: 2, Quiet: true}
	if _, err := Run(cfg); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(out)
	if string(got) != "x\ny\nz\n" {
		t.Errorf("merged output = %q, want %q", got, "x\ny\nz\n")
	}
	assertNoTemp(t, dir)
}

func TestRunFileOutput(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.txt")
	out := filepath.Join(dir, "out.txt") // does NOT exist yet
	if err := os.WriteFile(in, []byte("b\na\nb\nc\na\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Output mode: read in.txt, write the deduped result to out.txt, leaving
	// the source file untouched.
	cfg := cli.Config{Inputs: []string{in}, Output: out, Workers: 2, Quiet: true}
	sum, err := Run(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if sum.UniqueLines != 3 || sum.DuplicateLines != 2 {
		t.Fatalf("unique=%d dupes=%d, want 3/2", sum.UniqueLines, sum.DuplicateLines)
	}
	if sum.BytesWritten != int64(len("a\nb\nc\n")) {
		t.Fatalf("bytes written = %d, want %d", sum.BytesWritten, len("a\nb\nc\n"))
	}

	got, _ := os.ReadFile(out)
	if string(got) != "a\nb\nc\n" {
		t.Errorf("output = %q, want %q", got, "a\nb\nc\n")
	}
	// Source must be unchanged.
	src, _ := os.ReadFile(in)
	if string(src) != "b\na\nb\nc\na\n" {
		t.Errorf("source was modified: %q", src)
	}
	assertNoTemp(t, dir)
}

func TestRunWildcardExclude(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "1.txt"), []byte("x\ny\nx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2.txt"), []byte("a\na\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skip.txt"), []byte("KEEP\nKEEP\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := cli.Config{WildcardDir: dir, Exclude: []string{"skip.txt"}, Workers: 2, Quiet: true}
	sum, err := Run(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if sum.FilesProcessed != 2 {
		t.Fatalf("processed %d files, want 2", sum.FilesProcessed)
	}

	got1, _ := os.ReadFile(filepath.Join(dir, "1.txt"))
	if string(got1) != "x\ny\n" {
		t.Errorf("1.txt = %q, want %q", got1, "x\ny\n")
	}
	got2, _ := os.ReadFile(filepath.Join(dir, "2.txt"))
	if string(got2) != "a\nb\n" {
		t.Errorf("2.txt = %q, want %q", got2, "a\nb\n")
	}
	// Excluded file is left completely untouched.
	gotSkip, _ := os.ReadFile(filepath.Join(dir, "skip.txt"))
	if string(gotSkip) != "KEEP\nKEEP\n" {
		t.Errorf("excluded file changed: %q", gotSkip)
	}
	assertNoTemp(t, dir)
}

// assertNoTemp fails the test if any atomic-write temp file was left behind.
func assertNoTemp(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".refine-") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}
