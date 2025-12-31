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
	"bytes"
	"fmt"
	"io"
)

// ScanXML is a bufio.SplitFunc that tokenizes an input stream into top-level XML
// elements (one complete element per token).
//
// Framing behaviour:
//
//   - Any leading bytes before the first *start element* tag (`<name ...>`) are ignored.
//     This includes whitespace, XML declarations (`<?xml ...?>`), processing instructions,
//     comments, or doctype declarations.
//   - Nesting is tracked by pushing start element names and popping on matching end tags.
//   - The split func understands and skips:
//   - comments:        <!-- ... -->
//   - CDATA sections:  <![CDATA[ ... ]]>
//   - processing instr:<? ... ?>
//   - directives:      <! ... >   (doctype / declarations), with basic bracket/quote handling
//   - The returned token begins at the '<' of the start element and ends right after the
//     matching end tag (or the '/>' of a self-closing root element).
//   - If atEOF is true and an element is still open, ScanXML returns io.ErrUnexpectedEOF.
//
// This split func is a robust framing helper for streaming pipelines.
// It does NOT aim to be a fully validating XML parser.
//
// Example:
//
//	scanner := bufio.NewScanner(r)
//	scanner.Split(textual.ScanXML)
//	for scanner.Scan() {
//	    token := scanner.Bytes() // one complete XML element, UTF-8
//	    // ...
//	}
func ScanXML(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// No data and nothing more to read.
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	start := findFirstStartElement(data)
	if start == -1 {
		// No element start found in the current buffer. Since we explicitly
		// ignore leading noise, we can safely consume the whole buffer to
		// avoid unbounded growth.
		return len(data), nil, nil
	}

	// Parse from the first start element to find its matching end.
	i := start
	stack := make([]string, 0, 8)

	for i < len(data) {
		// Fast-forward until the next markup start.
		if data[i] != '<' {
			i++
			continue
		}

		// We need at least one byte after '<'.
		if i+1 >= len(data) {
			if atEOF {
				return 0, nil, io.ErrUnexpectedEOF
			}
			if start > 0 {
				return start, nil, nil
			}
			return 0, nil, nil
		}

		// 1) Comments: <!-- ... -->
		if data[i+1] == '!' && hasPrefixBytes(data[i:], xmlCommentOpen) {
			end, ok := indexAfter(data, i+len(xmlCommentOpen), xmlCommentClose)
			if !ok {
				if atEOF {
					return 0, nil, io.ErrUnexpectedEOF
				}
				if start > 0 {
					return start, nil, nil
				}
				return 0, nil, nil
			}
			i = end
			continue
		}

		// 2) CDATA: <![CDATA[ ... ]]>
		if data[i+1] == '!' && hasPrefixBytes(data[i:], xmlCDATAOpen) {
			end, ok := indexAfter(data, i+len(xmlCDATAOpen), xmlCDATAClose)
			if !ok {
				if atEOF {
					return 0, nil, io.ErrUnexpectedEOF
				}
				if start > 0 {
					return start, nil, nil
				}
				return 0, nil, nil
			}
			i = end
			continue
		}

		// 3) Processing instruction: <? ... ?>
		if data[i+1] == '?' {
			end, ok := indexAfter(data, i+2, xmlPIClose) // search after "<?"
			if !ok {
				if atEOF {
					return 0, nil, io.ErrUnexpectedEOF
				}
				if start > 0 {
					return start, nil, nil
				}
				return 0, nil, nil
			}
			i = end
			continue
		}

		// 4) Directives / doctype / declarations: <! ... >
		if data[i+1] == '!' {
			end, ok := scanDirectiveEnd(data, i+2) // after "<!"
			if !ok {
				if atEOF {
					return 0, nil, io.ErrUnexpectedEOF
				}
				if start > 0 {
					return start, nil, nil
				}
				return 0, nil, nil
			}
			i = end
			continue
		}

		// 5) End tag: </name>
		if data[i+1] == '/' {
			name, nameEnd, ok := scanName(data, i+2)
			if !ok {
				if atEOF {
					return 0, nil, io.ErrUnexpectedEOF
				}
				if start > 0 {
					return start, nil, nil
				}
				return 0, nil, nil
			}

			closeIdx, ok := scanTagClose(data, nameEnd)
			if !ok {
				if atEOF {
					return 0, nil, io.ErrUnexpectedEOF
				}
				if start > 0 {
					return start, nil, nil
				}
				return 0, nil, nil
			}

			if len(stack) == 0 {
				return 0, nil, fmt.Errorf("scanXML: unexpected closing tag </%s> at byte %d", name, i)
			}
			top := stack[len(stack)-1]
			if top != name {
				return 0, nil, fmt.Errorf("scanXML: mismatched closing tag </%s> for <%s> at byte %d", name, top, i)
			}
			stack = stack[:len(stack)-1]

			i = closeIdx + 1

			// If we just closed the root element, return it as a token.
			if len(stack) == 0 {
				return i, data[start:i], nil
			}
			continue
		}

		// 6) Start tag: <name ...> or <name .../>
		if isXMLNameStart(data[i+1]) {
			name, nameEnd, ok := scanName(data, i+1)
			if !ok {
				if atEOF {
					return 0, nil, io.ErrUnexpectedEOF
				}
				if start > 0 {
					return start, nil, nil
				}
				return 0, nil, nil
			}

			closeIdx, selfClosing, ok := scanStartTagClose(data, nameEnd)
			if !ok {
				if atEOF {
					return 0, nil, io.ErrUnexpectedEOF
				}
				if start > 0 {
					return start, nil, nil
				}
				return 0, nil, nil
			}

			if selfClosing {
				// Root self-closing element: complete token immediately.
				if len(stack) == 0 {
					i = closeIdx + 1
					return i, data[start:i], nil
				}
				// Nested self-closing element: no stack change.
				i = closeIdx + 1
				continue
			}

			// Regular start element: push to stack.
			stack = append(stack, name)
			i = closeIdx + 1
			continue
		}

		// Otherwise, this '<' isn't something we recognize as markup we want to
		// track (e.g. malformed input). Advance one byte to avoid infinite loops.
		i++
	}

	// Buffer ended before we closed the root element.
	if atEOF {
		return 0, nil, io.ErrUnexpectedEOF
	}
	if start > 0 {
		return start, nil, nil
	}
	return 0, nil, nil
}

var (
	xmlCommentOpen  = []byte("<!--")
	xmlCommentClose = []byte("-->")

	xmlCDATAOpen  = []byte("<![CDATA[")
	xmlCDATAClose = []byte("]]>")

	xmlPIClose = []byte("?>")
)

func hasPrefixBytes(b []byte, prefix []byte) bool {
	return len(b) >= len(prefix) && bytes.HasPrefix(b, prefix)
}

// indexAfter searches for `needle` in data starting at offset `from`.
// It returns the index immediately AFTER the needle when found.
func indexAfter(data []byte, from int, needle []byte) (after int, ok bool) {
	if from < 0 {
		from = 0
	}
	if from > len(data) {
		return 0, false
	}
	idx := bytes.Index(data[from:], needle)
	if idx == -1 {
		return 0, false
	}
	end := from + idx + len(needle)
	return end, true
}

// findFirstStartElement locates the first "<name" (start element) in the buffer.
// It intentionally ignores declarations (`<?...?>`), comments/CDATA/directives (`<!...>`),
// and closing tags (`</...>`).
func findFirstStartElement(data []byte) int {
	for i := 0; i < len(data); i++ {
		if data[i] != '<' {
			continue
		}
		if i+1 >= len(data) {
			return -1
		}
		n := data[i+1]
		if n == '/' || n == '!' || n == '?' {
			continue
		}
		if isXMLNameStart(n) {
			return i
		}
	}
	return -1
}

func isXMLNameStart(b byte) bool {
	// XML NameStartChar is much broader (unicode), but for framing purposes
	// we accept the common ASCII subset.
	return (b >= 'A' && b <= 'Z') ||
		(b >= 'a' && b <= 'z') ||
		b == '_' || b == ':'
}

func isXMLNameChar(b byte) bool {
	return isXMLNameStart(b) ||
		(b >= '0' && b <= '9') ||
		b == '-' || b == '.'
}

// scanName parses an XML name starting at offset `from`.
// It returns the parsed name and the index of the first byte after the name.
func scanName(data []byte, from int) (name string, next int, ok bool) {
	if from >= len(data) {
		return "", 0, false
	}
	i := from
	for i < len(data) && isXMLNameChar(data[i]) {
		i++
	}
	if i == from {
		return "", 0, false
	}
	return string(data[from:i]), i, true
}

// scanTagClose scans forward until it finds '>' and returns its index.
// Used for end tags where attributes are not expected; whitespace is tolerated.
func scanTagClose(data []byte, from int) (idx int, ok bool) {
	for i := from; i < len(data); i++ {
		if data[i] == '>' {
			return i, true
		}
	}
	return 0, false
}

// scanStartTagClose scans a start tag until the closing '>' (outside quotes).
// It returns the index of '>', whether the tag is self-closing, and ok=false if more data is needed.
func scanStartTagClose(data []byte, from int) (closeIdx int, selfClosing bool, ok bool) {
	var quote byte // 0, '\'' or '"'
	for i := from; i < len(data); i++ {
		b := data[i]

		if quote != 0 {
			if b == quote {
				quote = 0
			}
			continue
		}

		if b == '"' || b == '\'' {
			quote = b
			continue
		}

		if b == '>' {
			// Determine whether this is "/>" (ignoring trailing whitespace).
			j := i - 1
			for j >= from && (data[j] == ' ' || data[j] == '\t' || data[j] == '\n' || data[j] == '\r') {
				j--
			}
			if j >= from && data[j] == '/' {
				return i, true, true
			}
			return i, false, true
		}
	}
	return 0, false, false
}

// scanDirectiveEnd scans an XML directive ("<! ... >") and returns the index immediately after its closing '>'.
// It implements basic quoting and doctype internal subset bracket handling so that '>' inside
// quotes or inside "[ ... ]" does not terminate the directive prematurely.
func scanDirectiveEnd(data []byte, from int) (after int, ok bool) {
	var quote byte // 0, '\'' or '"'
	bracketDepth := 0

	for i := from; i < len(data); i++ {
		b := data[i]

		if quote != 0 {
			if b == quote {
				quote = 0
			}
			continue
		}

		if b == '"' || b == '\'' {
			quote = b
			continue
		}

		switch b {
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case '>':
			if bracketDepth == 0 {
				return i + 1, true
			}
		}
	}
	return 0, false
}
