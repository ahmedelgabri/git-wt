package fsutil

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
)

// FileState captures the content and permissions that migration must preserve.
// Timestamps are deliberately excluded: Git may refresh them while reading.
type FileState struct {
	Mode os.FileMode
	Hash [sha256.Size]byte
	Link string
}

func Snapshot(ctx context.Context, root string, excludes []string) (map[string]FileState, error) {
	excluded := make(map[string]bool)
	for _, path := range excludes {
		excluded[filepath.Clean(path)] = true
	}
	result := make(map[string]FileState)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if excluded[rel] {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		state := FileState{Mode: info.Mode()}
		switch {
		case info.Mode().IsRegular():
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			hash := sha256.New()
			_, copyErr := io.Copy(hash, &contextReader{ctx: ctx, reader: file})
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			copy(state.Hash[:], hash.Sum(nil))
		case info.Mode()&os.ModeSymlink != 0:
			state.Link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		case info.IsDir():
		default:
			return fmt.Errorf("unsupported file type: %s", path)
		}
		result[rel] = state
		return nil
	})
	return result, err
}

func VerifySnapshot(ctx context.Context, root string, excludes []string, expected map[string]FileState) error {
	actual, err := Snapshot(ctx, root, excludes)
	if err != nil {
		return err
	}
	if !maps.Equal(actual, expected) {
		return fmt.Errorf("filesystem verification failed for %s: file contents, paths, or modes changed", root)
	}
	return nil
}
