package fsutil

// Linux exposes supported ACL formats through the xattr list checked by
// readAttributes, including directory default ACLs.
func checkACL(string) error { return nil }
