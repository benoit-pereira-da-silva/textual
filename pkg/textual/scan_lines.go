package textual

import "bytes"

// ScanLines is a split function for a [Scanner] that returns each line of
// text, keeping any trailing end-of-line marker. The returned line may
// be empty. It is different from the bufio.ScanLines that drops the Carriage return.
func ScanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// No data and nothing more to read.
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// Look for '\n'. If found, include it in the token.
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		return i + 1, data[:i+1], nil
	}

	// If we're at EOF, return the final (non-newline-terminated) line.
	if atEOF {
		return len(data), data, nil
	}

	// Request more data.
	return 0, nil, nil
}
