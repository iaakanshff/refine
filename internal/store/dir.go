package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// ListFiles returns the regular files inside dir, sorted by name for
// deterministic ordering, skipping any whose base name is in exclude.
//
// Exclusions are keyed by base name (matching how users think about files in a
// directory) and are built once by the caller into a set for O(1) lookups.
// Directories and symlinks-to-directories are ignored; we only deduplicate
// regular files.
func ListFiles(dir string, exclude map[string]bool) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("list directory %q: %w", dir, err)
	}
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			// A race with another process removed it; skip rather than fail.
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		name := e.Name()
		if exclude[name] {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	return files, nil
}
