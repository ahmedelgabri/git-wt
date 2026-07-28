package hook

import (
	"slices"
	"testing"
)

func TestLoadConfigPreservesMultilineValues(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "2")
	t.Setenv("GIT_CONFIG_KEY_0", "wt.testhook")
	t.Setenv("GIT_CONFIG_VALUE_0", "printf 'first\\nsecond\\n'")
	t.Setenv("GIT_CONFIG_KEY_1", "wt.testhook")
	t.Setenv("GIT_CONFIG_VALUE_1", "touch done")

	hooks, err := LoadConfig("wt.testhook")
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	want := []string{"printf 'first\\nsecond\\n'", "touch done"}
	if !slices.Equal(hooks, want) {
		t.Fatalf("LoadConfig() = %#v, want %#v", hooks, want)
	}
}

func TestLoadConfigReturnsNilForMissingKey(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "0")

	hooks, err := LoadConfig("wt.missing")
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if hooks != nil {
		t.Fatalf("LoadConfig() = %#v, want nil", hooks)
	}
}
