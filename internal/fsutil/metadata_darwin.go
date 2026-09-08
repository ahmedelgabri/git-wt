package fsutil

import (
	"encoding/binary"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

func checkACL(path string) error {
	name, err := syscall.BytePtrFromString(path)
	if err != nil {
		return err
	}
	attrs := unix.Attrlist{Bitmapcount: unix.ATTR_BIT_MAP_COUNT, Commonattr: unix.ATTR_CMN_EXTENDED_SECURITY}
	// getattrlist returns a uint32 length followed by an attrreference_t:
	// int32 offset and uint32 data length. We only need the reference, not
	// the ACL payload. REPORT_FULLSIZE permits a header-sized buffer.
	var buffer [12]byte
	_, _, errno := syscall.Syscall6(syscall.SYS_GETATTRLIST,
		uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&attrs)),
		uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)),
		unix.FSOPT_NOFOLLOW|unix.FSOPT_REPORT_FULLSIZE, 0)
	if errno != 0 {
		return fmt.Errorf("inspect ACL at %s: %w", path, errno)
	}
	if binary.LittleEndian.Uint32(buffer[:4]) < uint32(len(buffer)) {
		return fmt.Errorf("invalid ACL metadata response at %s", path)
	}
	if binary.LittleEndian.Uint32(buffer[8:12]) != 0 {
		return fmt.Errorf("unsupported ACL metadata at %s; migration will not discard it", path)
	}
	return nil
}
