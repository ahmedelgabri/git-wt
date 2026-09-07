package fsutil

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"slices"
)

type fileAttribute struct {
	name string
	data []byte
}

// CheckMetadata refuses metadata we cannot preserve or inspect reliably.
func CheckMetadata(path string) error {
	_, err := readAttributes(path)
	return err
}

func metadataHash(path string) ([sha256.Size]byte, error) {
	attrs, err := readAttributes(path)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	hash := sha256.New()
	var size [8]byte
	for _, attr := range attrs {
		for _, data := range [][]byte{[]byte(attr.name), attr.data} {
			binary.LittleEndian.PutUint64(size[:], uint64(len(data)))
			hash.Write(size[:])
			hash.Write(data)
		}
	}
	var result [sha256.Size]byte
	copy(result[:], hash.Sum(nil))
	return result, nil
}

// CopyExtendedAttributes preserves xattrs without following symlinks. Existing
// identical attributes need no write, including OS-managed provenance labels.
// Extra destination attributes are removed; an inability to do so is an error.
func CopyExtendedAttributes(src, dst string) error {
	attrs, err := readAttributes(src)
	if err != nil {
		return err
	}
	existing, err := readAttributes(dst)
	if err != nil {
		return err
	}
	for _, attr := range existing {
		if !slices.ContainsFunc(attrs, func(want fileAttribute) bool { return want.name == attr.name }) {
			if err := removeAttribute(dst, attr.name); err != nil {
				return fmt.Errorf("remove extra extended attribute %s at %s: %w", attr.name, dst, err)
			}
		}
	}
	for _, attr := range attrs {
		if slices.ContainsFunc(existing, func(have fileAttribute) bool {
			return have.name == attr.name && bytes.Equal(have.data, attr.data)
		}) {
			continue
		}
		if err := setAttribute(dst, attr); err != nil {
			return fmt.Errorf("preserve extended attribute %s at %s: %w", attr.name, dst, err)
		}
	}
	actual, err := readAttributes(dst)
	if err != nil {
		return err
	}
	if !slices.EqualFunc(attrs, actual, func(a, b fileAttribute) bool {
		return a.name == b.name && bytes.Equal(a.data, b.data)
	}) {
		return fmt.Errorf("extended attribute verification failed for %s", dst)
	}
	return nil
}
