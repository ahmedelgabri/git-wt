// Package testutil isolates Git fixtures from developer configuration.
package testutil

import "os"

func IsolateGit() {
	for _, key := range []string{"GIT_DIR", "GIT_COMMON_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_CONFIG_PARAMETERS", "GIT_TEMPLATE_DIR", "DEBUG", "GIT_WT_SELECT", "FZF_DEFAULT_OPTS", "FZF_DEFAULT_OPTS_FILE"} {
		_ = os.Unsetenv(key)
	}
	_ = os.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	_ = os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	_ = os.Setenv("GIT_CONFIG_COUNT", "0")
	_ = os.Setenv("GIT_DEFAULT_HASH", "sha1")
	_ = os.Setenv("GIT_DEFAULT_REF_FORMAT", "files")
}
