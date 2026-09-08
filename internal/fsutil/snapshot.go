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

// FileState captures contents, permissions, and extended attributes for migration.
// Timestamps are deliberately excluded: Git may refresh them while reading.
type FileState struct {
	Mode       os.FileMode
	Hash       [sha256.Size]byte
	Link       string
	Attributes [sha256.Size]byte
}

func Snapshot(ctx context.Context, root string, excludes []string) (map[string]FileState, error) {
	return snapshot(ctx, root, excludes, true)
}

// SnapshotMetadata avoids reading file contents, for databases whose contents
// have their own integrity checks. Permissions and xattrs are still checked.
func SnapshotMetadata(ctx context.Context, root string, excludes []string) (map[string]FileState, error) {
	return snapshot(ctx, root, excludes, false)
}

func snapshot(ctx context.Context, root string, excludes []string, contents bool) (map[string]FileState, error) {
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
		if excluded[rel] {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		state, err := snapshotFile(ctx, path, contents)
		if err == nil {
			result[rel] = state
		}
		return err
	})
	return result, err
}

// SnapshotFile inspects one path without following symlinks.
func SnapshotFile(ctx context.Context, path string) (FileState, error) {
	return snapshotFile(ctx, path, true)
}

func snapshotFile(ctx context.Context, path string, contents bool) (FileState, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return FileState{}, err
	}
	state := FileState{Mode: info.Mode()}
	state.Attributes, err = metadataHash(path)
	if err != nil {
		return state, err
	}
	switch {
	case info.Mode().IsRegular():
		if !contents {
			return state, nil
		}
		file, err := os.Open(path)
		if err != nil {
			return state, err
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, &contextReader{ctx: ctx, reader: file})
		closeErr := file.Close()
		if copyErr != nil {
			return state, copyErr
		}
		if closeErr != nil {
			return state, closeErr
		}
		copy(state.Hash[:], hash.Sum(nil))
	case info.Mode()&os.ModeSymlink != 0:
		state.Link, err = os.Readlink(path)
	case info.IsDir():
	default:
		err = fmt.Errorf("unsupported file type: %s", path)
	}
	return state, err
}

func VerifySnapshot(ctx context.Context, root string, excludes []string, expected map[string]FileState) error {
	actual, err := Snapshot(ctx, root, excludes)
	if err != nil {
		return err
	}
	if !maps.Equal(actual, expected) {
		return fmt.Errorf("filesystem verification failed for %s: file contents, paths, modes, or extended attributes changed", root)
	}
	return nil
}
