//go:build darwin || linux

package fsutil

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"golang.org/x/sys/unix"
)

func readAttributes(path string) ([]fileAttribute, error) {
	if err := checkACL(path); err != nil {
		return nil, err
	}
	size, err := unix.Llistxattr(path, nil)
	if errors.Is(err, unix.ENOTSUP) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect extended attributes at %s: %w", path, err)
	}
	// Resource forks can be large. Refuse rather than risk an unbounded
	// allocation; ordinary file contents are copied and hashed as streams.
	const limit = 64 << 20
	if size > limit {
		return nil, fmt.Errorf("extended attributes exceed 64 MiB at %s", path)
	}
	if size == 0 {
		return nil, nil
	}
	buffer := make([]byte, size)
	n, err := unix.Llistxattr(path, buffer)
	if err != nil {
		return nil, fmt.Errorf("read extended attribute names at %s: %w", path, err)
	}
	names := strings.Split(strings.TrimSuffix(string(buffer[:n]), "\x00"), "\x00")
	slices.Sort(names)
	attrs := make([]fileAttribute, 0, len(names))
	for _, name := range names {
		if name == "system.posix_acl_access" || name == "system.posix_acl_default" || name == "system.nfs4_acl" || name == "system.richacl" {
			return nil, fmt.Errorf("unsupported ACL metadata at %s (%s); migration will not discard it", path, name)
		}
		n, err := unix.Lgetxattr(path, name, nil)
		if err != nil {
			return nil, fmt.Errorf("inspect extended attribute %s at %s: %w", name, path, err)
		}
		if n > limit-size {
			return nil, fmt.Errorf("extended attributes exceed 64 MiB at %s", path)
		}
		size += n
		data := make([]byte, n)
		n, err = unix.Lgetxattr(path, name, data)
		if err != nil {
			return nil, fmt.Errorf("read extended attribute %s at %s: %w", name, path, err)
		}
		if n > len(data) {
			return nil, fmt.Errorf("extended attribute %s changed while reading %s", name, path)
		}
		attrs = append(attrs, fileAttribute{name: name, data: data[:n]})
	}
	return attrs, nil
}

func setAttribute(path string, attr fileAttribute) error {
	return unix.Lsetxattr(path, attr.name, attr.data, 0)
}

func removeAttribute(path, name string) error {
	return unix.Lremovexattr(path, name)
}
