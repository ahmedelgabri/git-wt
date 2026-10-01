package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Migration rewrites the repository in place. Every mutation is recorded here
// before it happens, so an interrupted run can be undone by the same process or
// by the next `git wt migrate`, even after a crash or power loss.
const migrationStateDir = ".git-wt-migrate"

type migrationJournal struct {
	root    string
	file    *os.File
	unlock  func()
	backups int
	// beforeRecord lets tests interrupt migration at every step.
	beforeRecord func() error
}

func migrationJournalPath(root string) string {
	return filepath.Join(root, migrationStateDir, "journal")
}

func startMigrationJournal(root string) (*migrationJournal, error) {
	// Mkdir is atomic, so two migrations of one repository cannot both start.
	if err := os.Mkdir(filepath.Join(root, migrationStateDir), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(migrationJournalPath(root), os.O_CREATE|os.O_EXCL|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	unlock, err := lockMigrationJournal(file)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return &migrationJournal{root: root, file: file, unlock: unlock}, nil
}

func (j *migrationJournal) path(rel string) string {
	return filepath.Join(j.root, rel)
}

// record syncs each entry before the caller acts on it. An entry without its
// trailing newline was never acted on and is ignored during undo.
func (j *migrationJournal) record(op string, paths ...string) error {
	if j.beforeRecord != nil {
		if err := j.beforeRecord(); err != nil {
			return err
		}
	}
	var line strings.Builder
	line.WriteString(op)
	for _, path := range paths {
		line.WriteByte(' ')
		line.WriteString(strconv.Quote(path))
	}
	line.WriteByte('\n')
	if _, err := j.file.WriteString(line.String()); err != nil {
		return err
	}
	return j.file.Sync()
}

func (j *migrationJournal) move(src, dst string) error {
	if err := j.record("move", src, dst); err != nil {
		return err
	}
	return renameEntry(j.path(src), j.path(dst))
}

// create records a file or directory that migration is about to create. Undo
// removes it without recursion, so user data moved into a directory is never
// deleted with it.
func (j *migrationJournal) create(rel string) error {
	return j.record("create", rel)
}

// createTree records a Git-generated directory whose contents migration does
// not control. Undo removes it recursively after moving migrated entries out.
func (j *migrationJournal) createTree(rel string) error {
	return j.record("create-tree", rel)
}

// backup saves a small file that Git is about to rewrite in place.
func (j *migrationJournal) backup(rel string) error {
	info, err := os.Lstat(j.path(rel))
	if os.IsNotExist(err) {
		return j.create(rel)
	}
	if err != nil {
		return err
	}
	data, err := os.ReadFile(j.path(rel))
	if err != nil {
		return err
	}
	j.backups++
	saved := filepath.Join(migrationStateDir, "backup", strconv.Itoa(j.backups))
	if err := os.MkdirAll(filepath.Dir(j.path(saved)), 0o700); err != nil {
		return err
	}
	if err := writeSyncedFile(j.path(saved), data, info.Mode().Perm()); err != nil {
		return err
	}
	return j.record("restore", rel, saved)
}

func writeSyncedFile(path string, data []byte, perm os.FileMode) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	return errors.Join(err, file.Close())
}

// finish removes the journal after a successful migration or undo. Only
// migration's own files are removed; a non-empty staging directory is an error.
func (j *migrationJournal) finish() error {
	j.unlock()
	if err := j.file.Close(); err != nil {
		return err
	}
	return removeMigrationState(j.root)
}

func (j *migrationJournal) rollback() error {
	err := undoMigration(j.root)
	j.unlock()
	return errors.Join(err, j.file.Close())
}

// recoverMigration undoes a migration interrupted in an earlier process.
func recoverMigration(root string) error {
	file, err := os.OpenFile(migrationJournalPath(root), os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	unlock, err := lockMigrationJournal(file)
	if err != nil {
		return err
	}
	defer unlock()
	return undoMigration(root)
}

func removeMigrationState(root string) error {
	dir := filepath.Join(root, migrationStateDir)
	for _, path := range []string{filepath.Join(dir, "work"), filepath.Join(dir, "backup"), migrationJournalPath(root), dir} {
		var err error
		if filepath.Base(path) == "backup" {
			err = os.RemoveAll(path)
		} else {
			err = os.Remove(path)
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

type migrationRecord struct {
	op    string
	paths []string
}

func readMigrationJournal(root string) ([]migrationRecord, error) {
	data, err := os.ReadFile(migrationJournalPath(root))
	if err != nil {
		return nil, err
	}
	var records []migrationRecord
	lines := strings.Split(string(data), "\n")
	// The last element is empty, or an entry cut off before its newline.
	for _, line := range lines[:len(lines)-1] {
		op, rest, _ := strings.Cut(line, " ")
		record := migrationRecord{op: op}
		for rest != "" {
			quoted, err := strconv.QuotedPrefix(rest)
			if err != nil {
				return nil, fmt.Errorf("corrupt migration journal entry %q", line)
			}
			path, _ := strconv.Unquote(quoted)
			record.paths = append(record.paths, path)
			rest = strings.TrimPrefix(rest[len(quoted):], " ")
		}
		records = append(records, record)
	}
	return records, nil
}

// undoMigration reverses journal entries newest first. Each step checks the
// current state, so undo can be repeated after it is itself interrupted.
func undoMigration(root string) error {
	records, err := readMigrationJournal(root)
	if err != nil {
		return err
	}
	for i := len(records) - 1; i >= 0; i-- {
		if err := undoMigrationRecord(root, records[i]); err != nil {
			return err
		}
	}
	return removeMigrationState(root)
}

func undoMigrationRecord(root string, record migrationRecord) error {
	paths := make([]string, len(record.paths))
	for i, path := range record.paths {
		paths[i] = filepath.Join(root, path)
	}
	switch {
	case record.op == "move" && len(paths) == 2:
		src, dst := paths[0], paths[1]
		srcExists, err := pathExists(src)
		if err != nil {
			return err
		}
		dstExists, err := pathExists(dst)
		if err != nil {
			return err
		}
		switch {
		case dstExists && !srcExists:
			return os.Rename(dst, src)
		case srcExists && !dstExists:
			return nil
		case srcExists:
			return fmt.Errorf("cannot restore %s: both it and %s exist", src, dst)
		default:
			return fmt.Errorf("cannot restore %s: it is missing from %s", src, dst)
		}
	case record.op == "create" && len(paths) == 1:
		if err := os.Remove(paths[0]); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	case record.op == "create-tree" && len(paths) == 1:
		return os.RemoveAll(paths[0])
	case record.op == "restore" && len(paths) == 2:
		// A missing saved copy was already restored by an earlier undo attempt.
		if err := os.Rename(paths[1], paths[0]); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return fmt.Errorf("unknown migration journal entry %q", record.op)
}

// renameEntry never overwrites: os.Rename would silently replace a file.
func renameEntry(src, dst string) error {
	if exists, err := pathExists(dst); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("refusing to overwrite %s", dst)
	}
	return os.Rename(src, dst)
}

func pathExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// findInterruptedMigration searches upward from the working directory, because
// an interrupted migration may have moved it into the staging directory.
func findInterruptedMigration() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		if exists, _ := pathExists(migrationJournalPath(dir)); exists {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
