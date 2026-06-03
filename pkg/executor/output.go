package executor

// CaptureOutput processes raw stdout and stderr byte slices and applies tail
// truncation if either exceeds maxBytes.
//
// For each stream independently:
//   - If the length is ≤ maxBytes, the full content is returned and the
//     truncated flag is false.
//   - If the length is > maxBytes, only the last maxBytes bytes (tail) are
//     returned and the truncated flag is true.
func CaptureOutput(stdout, stderr []byte, maxBytes int64) (outStr, errStr string, outTruncated, errTruncated bool) {
	outStr, outTruncated = truncateTail(stdout, maxBytes)
	errStr, errTruncated = truncateTail(stderr, maxBytes)
	return
}

// truncateTail returns the tail of data if it exceeds maxBytes, otherwise the
// full content. The boolean indicates whether truncation occurred.
func truncateTail(data []byte, maxBytes int64) (string, bool) {
	if int64(len(data)) <= maxBytes {
		return string(data), false
	}
	// Keep only the last maxBytes bytes.
	offset := int64(len(data)) - maxBytes
	return string(data[offset:]), true
}
