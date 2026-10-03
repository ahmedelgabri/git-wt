package cmd

import (
	"crypto/rand"
	"encoding/hex"
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

// migrationIDName is written inside the Git directory with the journal's ID.
// A clone or checkout can put a journal in the working tree, but never this
// file, so recovery only replays journals whose ID it matches.
const migrationIDName = "git-wt-migration"

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
	// Git reports resolved paths; recorded paths must be relative to the
	// same root to stay inside the repository.
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
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
	j := &migrationJournal{root: root, file: file, unlock: unlock}
	if err := j.writeID(); err != nil {
		// Nothing has moved yet; leave no journal that recovery would refuse.
		unlock()
		_ = file.Close()
		_ = os.Remove(filepath.Join(root, ".git", migrationIDName))
		return nil, errors.Join(err, removeMigrationState(root))
	}
	return j, nil
}

func (j *migrationJournal) writeID() error {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return err
	}
	if _, err := j.file.WriteString("id " + hex.EncodeToString(id) + "\n"); err != nil {
		return err
	}
	idFile := filepath.Join(".git", migrationIDName)
	if err := j.create(idFile); err != nil {
		return err
	}
	return writeSyncedFile(j.path(idFile), []byte(hex.EncodeToString(id)), 0o600)
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

// commit marks the migration complete. From then on, recovery finishes the
// cleanup instead of undoing the migration, whose saved files cleanup deletes.
func (j *migrationJournal) commit() error {
	if _, err := j.file.WriteString("commit\n"); err != nil {
		return err
	}
	return j.file.Sync()
}

// finish commits and removes the journal after a successful migration,
// holding the lock until the journal is gone. Only migration's own files are
// removed; a non-empty staging directory is an error.
func (j *migrationJournal) finish() error {
	defer j.file.Close()
	defer j.unlock()
	if err := j.commit(); err != nil {
		return err
	}
	if err := removeMigrationState(j.root); err != nil {
		return err
	}
	// Removed last: until the journal is gone, recovery needs it.
	if err := os.Remove(j.path(filepath.Join(".bare", migrationIDName))); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (j *migrationJournal) rollback() error {
	err := undoMigration(j.root, j.file)
	j.unlock()
	return errors.Join(err, j.file.Close())
}

// recoverMigration handles a journal left by an earlier process. It undoes an
// interrupted migration, or finishes cleaning up a completed one, which it
// reports with completed.
func recoverMigration(root string) (completed bool, err error) {
	file, err := os.OpenFile(migrationJournalPath(root), os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return false, err
	}
	defer file.Close()
	unlock, err := lockMigrationJournal(file)
	if err != nil {
		return false, err
	}
	defer unlock()
	state, err := readMigrationJournal(root)
	if err != nil {
		return false, err
	}
	if err := checkMigrationJournalID(root, state); err != nil {
		return false, err
	}
	for _, record := range state.records {
		if !validMigrationRecord(record) {
			return false, fmt.Errorf("corrupt migration journal entry %s %q; nothing was changed", record.op, record.paths)
		}
	}
	if state.committed {
		if err := removeMigrationState(root); err != nil {
			return true, err
		}
		if err := os.Remove(filepath.Join(root, ".bare", migrationIDName)); err != nil && !os.IsNotExist(err) {
			return true, err
		}
		return true, nil
	}
	return false, undoMigration(root, file)
}

// checkMigrationJournalID refuses journals this repository's migration did
// not write, such as one tracked in a cloned repository.
func checkMigrationJournalID(root string, state migrationJournalState) error {
	if state.id != "" {
		// .git before the Git directory moves, .bare after.
		for _, dir := range []string{".git", ".bare"} {
			if id, err := os.ReadFile(filepath.Join(root, dir, migrationIDName)); err == nil && string(id) == state.id {
				return nil
			}
		}
		// Undo removes the ID file last, so a fully undone journal has none,
		// and nothing left to replay.
		if !state.committed && state.undone == len(state.records) {
			return nil
		}
	}
	return fmt.Errorf("%s was not written by git wt migrate for this repository; nothing was changed. Remove it if you did not start a migration here", migrationJournalPath(root))
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

type migrationJournalState struct {
	id      string
	records []migrationRecord
	// undone counts records already reversed, newest first.
	undone int
	// complete is the length of the journal up to its last complete entry.
	complete int64
	// committed is set once the migration succeeded; it must not be undone.
	committed bool
}

func readMigrationJournal(root string) (migrationJournalState, error) {
	var state migrationJournalState
	data, err := os.ReadFile(migrationJournalPath(root))
	if err != nil {
		return state, err
	}
	state.complete = int64(strings.LastIndexByte(string(data), '\n') + 1)
	lines := strings.Split(string(data), "\n")
	// The last element is empty, or an entry cut off before its newline.
	for _, line := range lines[:len(lines)-1] {
		if line == "undo" {
			state.undone++
			continue
		}
		if line == "commit" {
			state.committed = true
			continue
		}
		if id, ok := strings.CutPrefix(line, "id "); ok && len(state.records) == 0 && state.id == "" {
			state.id = id
			continue
		}
		op, rest, _ := strings.Cut(line, " ")
		record := migrationRecord{op: op}
		for rest != "" {
			quoted, err := strconv.QuotedPrefix(rest)
			if err != nil {
				return state, fmt.Errorf("corrupt migration journal entry %q", line)
			}
			path, _ := strconv.Unquote(quoted)
			record.paths = append(record.paths, path)
			rest = strings.TrimPrefix(rest[len(quoted):], " ")
		}
		state.records = append(state.records, record)
	}
	if state.undone > len(state.records) {
		return state, fmt.Errorf("corrupt migration journal: more undo entries than steps")
	}
	return state, nil
}

// validMigrationRecord accepts only the entries migration writes, with paths
// inside the repository, so replay cannot touch anything outside it.
func validMigrationRecord(record migrationRecord) bool {
	for _, path := range record.paths {
		if !filepath.IsLocal(path) {
			return false
		}
	}
	switch record.op {
	case "move":
		return len(record.paths) == 2
	case "create":
		return len(record.paths) == 1
	case "create-tree":
		return len(record.paths) == 1 && record.paths[0] == filepath.Join(".bare", "worktrees")
	case "restore":
		return len(record.paths) == 2 && strings.HasPrefix(record.paths[1], filepath.Join(migrationStateDir, "backup")+string(filepath.Separator))
	}
	return false
}

// migrationUndoHook lets tests interrupt rollback at every step.
var migrationUndoHook func() error

// undoMigration reverses journal entries newest first and appends an undo
// entry after each one, so a stopped rollback resumes where it left off.
// Replaying from the end would fail: undoing a later move, such as .bare back
// to .git, changes the paths that earlier, already undone entries name. A
// crash before an undo entry is written repeats only that step, which checks
// the current state and is safe to repeat.
func undoMigration(root string, journal *os.File) error {
	state, err := readMigrationJournal(root)
	if err != nil {
		return err
	}
	if state.committed {
		return fmt.Errorf("migration already completed; refusing to undo it")
	}
	// Drop an entry cut off mid-write, which was never acted on, so undo
	// entries start on a line of their own.
	if err := journal.Truncate(state.complete); err != nil {
		return err
	}
	for i := len(state.records) - 1 - state.undone; i >= 0; i-- {
		if migrationUndoHook != nil {
			if err := migrationUndoHook(); err != nil {
				return err
			}
		}
		if err := undoMigrationRecord(root, state.records[i]); err != nil {
			return err
		}
		if _, err := journal.WriteString("undo\n"); err != nil {
			return err
		}
		if err := journal.Sync(); err != nil {
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
