package fsutil

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// CopyDir copies src into dst. Excludes are paths relative to src, not base
// names: excluding .git does not discard nested repositories. Existing files
// are replaced without following destination symlinks.
func CopyDir(src, dst string, excludes []string) error {
	return CopyDirContext(context.Background(), src, dst, excludes)
}

func CopyDirContext(ctx context.Context, src, dst string, excludes []string) error {
	excluded := make(map[string]bool, len(excludes))
	for _, e := range excludes {
		excluded[filepath.Clean(e)] = true
	}
	var dirs []string
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if excluded[rel] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if existing, err := os.Lstat(target); err == nil {
			if !d.IsDir() || !existing.IsDir() {
				// Refuse to recursively discard unexpected destination directories.
				if err := os.Remove(target); err != nil {
					return err
				}
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		if d.IsDir() {
			if err := os.MkdirAll(target, info.Mode().Perm()|0o700); err != nil {
				return err
			}
			dirs = append(dirs, rel)
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported file type: %s", path)
		}
		return copyFileContext(ctx, path, target)
	})
	if err != nil {
		return err
	}
	// Restore directory metadata after children have been written.
	for i := len(dirs) - 1; i >= 0; i-- {
		info, err := os.Stat(filepath.Join(src, dirs[i]))
		if err != nil {
			return err
		}
		target := filepath.Join(dst, dirs[i])
		if err := os.Chmod(target, info.Mode().Perm()); err != nil {
			return err
		}
		if err := os.Chtimes(target, info.ModTime(), info.ModTime()); err != nil {
			return err
		}
	}
	return nil
}

func copyFileContext(ctx context.Context, src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	// Exclusive creation makes a raced-in symlink an error, never a write target.
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, &contextReader{ctx: ctx, reader: in})
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Chmod(dst, info.Mode().Perm()); err != nil {
		return err
	}
	return os.Chtimes(dst, info.ModTime(), info.ModTime())
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
