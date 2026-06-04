package controllers

const (
	maxChannelLen     = 64
	maxFingerprintLen = 128
)

// isValidChannel reports whether ch is safe to use as a channel name.
//
// Channel names are used as cold-storage file names ({channel}.cold.zst) and are
// written into the binary snapshot with a uint16 length prefix, so they must be
// path-safe and bounded in length. Allowed characters are ASCII letters, digits,
// '_', '-' and '.'; the path-traversal forms "." and ".." are rejected, and the
// charset already excludes the path separator '/'.
func isValidChannel(ch string) bool {
	if len(ch) == 0 || len(ch) > maxChannelLen {
		return false
	}
	if ch == "." || ch == ".." {
		return false
	}
	for i := 0; i < len(ch); i++ {
		c := ch[i]
		switch {
		case c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9',
			c == '_', c == '-', c == '.':
		default:
			return false
		}
	}
	return true
}

// isValidFingerprint bounds the fingerprint length so it cannot overflow the
// uint16 length prefix used when fingerprints are written to the binary snapshot.
// An empty fingerprint is allowed because the field is optional.
func isValidFingerprint(fp string) bool {
	return len(fp) <= maxFingerprintLen
}
