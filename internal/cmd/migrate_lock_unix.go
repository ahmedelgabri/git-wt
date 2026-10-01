//go:build unix

package cmd

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// The lock lives as long as the migrating process, so a concurrent run cannot
// mistake an active migration for an interrupted one and undo it.
func lockMigrationJournal(file *os.File) (func(), error) {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return nil, fmt.Errorf("another git wt migrate is running in this repository")
	}
	if err != nil {
		return nil, err
	}
	return func() { _ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN) }, nil
}
