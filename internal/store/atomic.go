package store

import (
	"os"
	"path/filepath"
)

// OpenAtomic creates a temporary file in the same directory as path and returns it
// together with a commit function. Callers must write to the returned file and then call
// commit, which fsyncs and renames the temp file over path.
//
// This is the fix for the old tool's data-loss bug: previously refine truncated the
// destination in place, so a crash, SIGINT or full disk mid-write would destroy the
// original file. With atomic replace the original is untouched until the new content is
// fully written and fsynced; the rename is atomic at the filesystem level, so readers never
// see a half-written file.
//
// The temp file lives in the same directory as path so the rename is on the same
// filesystem (rename across filesystems is not atomic and can fail).
func OpenAtomic(path string) (*os.File, func() error, error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".refine-*.tmp")
	if err != nil {
		return nil, nil, err
	}
	commit := func() error {
		if err := tmp.Sync(); err != nil {
			return err
		}
		return os.Rename(tmp.Name(), path)
	}
	return tmp, commit, nil
}
