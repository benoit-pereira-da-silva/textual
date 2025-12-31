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

import "io"

// ScanCSV is a bufio.SplitFunc that tokenizes an input stream into CSV records.
//
// Framing behaviour:
//
//   - Records are delimited by '\n', '\r\n', or '\r' when the delimiter occurs
//     OUTSIDE of quoted fields.
//   - Quotes are recognized using the standard CSV escaping rules:
//   - a quoted field starts with '"'
//   - inside a quoted field, '""' represents an escaped quote
//   - The returned token does NOT include the trailing record separator.
//   - If atEOF is true and a quoted field is still open, ScanCSV returns
//     io.ErrUnexpectedEOF.
//
// This split func does not validate the full CSV dialect (delimiter choice,
// comments, etc). It only provides robust record framing suitable for streaming.
//
// Example:
//
//	scanner := bufio.NewScanner(r)
//	scanner.Split(textual.ScanCSV)
//	for scanner.Scan() {
//	    token := scanner.Bytes() // one complete CSV record (UTF-8), no trailing newline
//	    // ...
//	}
func ScanCSV(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// No data and nothing more to read.
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	inQuotes := false
	i := 0

	for i < len(data) {
		b := data[i]

		if b == '"' {
			if inQuotes {
				// Inside quotes, a doubled quote ("") is an escaped quote.
				if i+1 < len(data) && data[i+1] == '"' {
					i += 2
					continue
				}
				// Otherwise this closes the quoted field.
				inQuotes = false
				i++
				continue
			}
			// Opening quote.
			inQuotes = true
			i++
			continue
		}

		if !inQuotes {
			switch b {
			case '\n':
				// Record ends before '\n'. Trim a preceding '\r' if present.
				end := i
				if end > 0 && data[end-1] == '\r' {
					end--
				}
				return i + 1, data[:end], nil

			case '\r':
				// Record ends before '\r'. Support both '\r\n' and '\r'.
				end := i
				adv := i + 1
				if i+1 < len(data) && data[i+1] == '\n' {
					adv = i + 2
				}
				return adv, data[:end], nil
			}
		}

		i++
	}

	// No record delimiter found in current buffer.
	if atEOF {
		if inQuotes {
			return 0, nil, io.ErrUnexpectedEOF
		}
		// Last record at EOF: return the remainder (no delimiter).
		return len(data), data, nil
	}

	// Request more data.
	return 0, nil, nil
}
