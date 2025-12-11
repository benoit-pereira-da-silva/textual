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
	"unicode"
	"unicode/utf8"
)

// ScanExpression is a bufio.SplitFunc that tokenizes an input stream into
// "expressions" centered around words.
//
// Each token has the shape:
//
//	[optional leading whitespace]
//	[a non-empty run of non-whitespace characters]
//	[optional trailing whitespace]
//
// Consequences:
//
//   - Punctuation surrounding a word (commas, quotes, etc.) stays in the same
//     token as the word.
//   - Spaces and line breaks that follow the word are attached to that token.
//   - Spaces that precede the first word in the stream (or in a line) are
//     attached to the token for that word.
//   - Concatenating all tokens in order reconstructs the original byte stream.
//
// This function is useful when streaming text "word-by-word" but you still want
// each piece to be directly printable without guessing where to insert spaces
// or newlines.
func ScanExpression(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// No data and nothing more to read.
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// Find the first non-space rune, keeping any leading whitespace so that it
	// ends up in the same token as the following word / punctuation.
	firstNonSpace := -1
	i := 0
	for i < len(data) {
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size == 1 {
			// Treat decoding errors as non-space to avoid getting stuck.
			firstNonSpace = i
			break
		}
		if !unicode.IsSpace(r) {
			firstNonSpace = i
			break
		}
		i += size
	}

	if firstNonSpace == -1 {
		// The buffer currently only contains whitespace.
		if atEOF {
			// Deliver it as a single final token.
			return len(data), data, nil
		}
		// Request more data; keep the whitespace so it can be attached to the
		// first word that follows.
		return 0, nil, nil
	}

	// Find the end of the non-space "core" (word + punctuation).
	endNon := firstNonSpace
	for endNon < len(data) {
		r, size := utf8.DecodeRune(data[endNon:])
		if r == utf8.RuneError && size == 1 {
			// Keep undecodable bytes inside the core.
			endNon += size
			continue
		}
		if unicode.IsSpace(r) {
			break
		}
		endNon += size
	}

	if endNon == len(data) {
		// We reached the end of the buffer while scanning the core.
		// If this is also EOF, we can safely return everything we have (including
		// any leading whitespace). Otherwise, ask for more data so we don't split
		// the core mid-rune.
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	}

	// Extend the token through any trailing whitespace so that printing the
	// token reproduces both the word and the layout that follows it (including
	// newlines).
	end := endNon
	for end < len(data) {
		r, size := utf8.DecodeRune(data[end:])
		if r == utf8.RuneError && size == 1 {
			break
		}
		if !unicode.IsSpace(r) {
			break
		}
		end += size
	}

	// The token always starts at the beginning of the buffer so that any
	// leading whitespace is included.
	return end, data[:end], nil
}
