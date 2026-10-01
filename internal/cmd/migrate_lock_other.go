//go:build !unix

package cmd

import (
	"fmt"
	"os"
)

func lockMigrationJournal(*os.File) (func(), error) {
	return nil, fmt.Errorf("migrate is not supported on this platform")
}
