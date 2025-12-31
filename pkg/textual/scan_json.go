// Copyright 2026 Benoit Pereira da Silva
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package textual

import (
	"fmt"
	"io"
)

// ScanJSON is a bufio.SplitFunc that tokenizes an input stream into top-level
// JsonCarrier values (objects `{...}` or arrays `[...]`).
//
// Framing behaviour:
//
//   - Any leading bytes before the first `{` or `[` are ignored (consumed).
//     This includes spaces, newlines, commas, or any other delimiter your
//     transport might insert.
//   - Once an opening `{` or `[` is found, nesting is tracked until the
//     matching closing delimiter is found.
//   - JsonCarrier strings are recognized; braces/brackets inside strings do not affect
//     nesting. Basic escape handling is implemented so `\"` does not end a string.
//   - If atEOF is true and a JsonCarrier value is still open, ScanJSON returns
//     io.ErrUnexpectedEOF.
//
// This split func does NOT fully validate JsonCarrier; it only provides robust framing
// suitable for streaming.
//
// Example:
//
//	scanner := bufio.NewScanner(r)
//	scanner.Split(textual.ScanJSON)
//	for scanner.Scan() {
//	    token := scanner.Bytes() // one complete JsonCarrier value
//	    // ...
//	}
func ScanJSON(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// No data and nothing more to read.
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// Find the first '{' or '['. Everything before it is ignored.
	start := -1
	for i, b := range data {
		if b == '{' || b == '[' {
			start = i
			break
		}
	}

	if start == -1 {
		// No opening delimiter in the current buffer.
		// Since we explicitly ignore leading noise, we can safely consume the
		// whole buffer (even when !atEOF) to avoid unbounded growth.
		return len(data), nil, nil
	}

	// Important: bufio.Scanner will call the split function with atEOF=true once
	// the underlying reader is exhausted, even if there is still unprocessed data
	// in its internal buffer. In that situation, returning token == nil can make
	// the scanner stop prematurely.
	//
	// To support multiple JsonCarrier values in a single buffer (e.g. "  {...}  [...]"),
	// we must be able to return a complete token even when it is preceded by
	// ignored bytes.

	// data[start] is '{' or '['.
	stack := make([]byte, 0, 8)
	stack = append(stack, data[start])

	inString := false
	escaped := false

	// Start scanning right after the opening delimiter.
	for i := start + 1; i < len(data); i++ {
		b := data[i]

		if inString {
			if escaped {
				escaped = false
				continue
			}
			if b == '\\' {
				escaped = true
				continue
			}
			if b == '"' {
				inString = false
			}
			continue
		}

		// Outside of strings.
		switch b {
		case '"':
			inString = true

		case '{', '[':
			stack = append(stack, b)

		case '}', ']':
			if len(stack) == 0 {
				return 0, nil, fmt.Errorf("scanJSON: unexpected closing %q at byte %d", b, i)
			}
			top := stack[len(stack)-1]
			matches := (b == '}' && top == '{') || (b == ']' && top == '[')
			if !matches {
				return 0, nil, fmt.Errorf("scanJSON: mismatched closing %q for %q at byte %d", b, top, i)
			}
			// Pop.
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				end := i + 1
				return end, data[start:end], nil
			}
		}
	}

	// Buffer ended before we found the matching closing delimiter.
	if atEOF {
		return 0, nil, io.ErrUnexpectedEOF
	}
	// If we had to skip leading noise, consume it now so the scanner doesn't
	// keep growing its buffer indefinitely while waiting for more bytes.
	if start > 0 {
		return start, nil, nil
	}
	return 0, nil, nil
}
