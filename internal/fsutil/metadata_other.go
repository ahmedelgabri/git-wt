//go:build !darwin && !linux

package fsutil

import "fmt"

func readAttributes(path string) ([]fileAttribute, error) {
	return nil, fmt.Errorf("filesystem metadata inspection is unsupported on this platform: %s", path)
}

func setAttribute(path string, _ fileAttribute) error {
	return fmt.Errorf("extended attribute copying is unsupported on this platform: %s", path)
}

func removeAttribute(path, _ string) error {
	return fmt.Errorf("extended attribute removal is unsupported on this platform: %s", path)
}
