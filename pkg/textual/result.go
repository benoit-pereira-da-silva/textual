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
	"sort"
	"strings"
)

type Fragment struct {
	Transformed UTF8String `json:"transformed"` // The transformed text may be in multiple dialect IPA, SAMPA, pseudo phonetics, ...
	Pos         int        `json:"pos"`         // The first char position in the original text.
	Len         int        `json:"len"`         // The len of the expression in the original text
	Confidence  float64    `json:"confidence"`  // The confidence of the result.
	Variant     int        `json:"variant"`     // A variant number, when the dictionary offers multiple candidates.
}

// RawTexts is a slice of RawTexts
// It is computed on a Result.
// It represents the parts that have not been processed.
type RawTexts []RawText

type RawText struct {
	Text UTF8String `json:"text"` // The remaining raw text.
	Pos  int        `json:"pos"`  // Its position in the original text
	Len  int        `json:"len"`  // Its length
}

type Result struct {
	Index     int        `json:"index,omitempty"` // the optional index in Results slice
	Text      UTF8String `json:"text"`            // The original text in UTF8
	Fragments []Fragment `json:"fragments"`       // The processed Fragment
	Error     error      `json:"error,omitempty"` // an optional error
}

// Input a text to create a base Result to be used as a starting point by a processor.
func Input(text UTF8String) Result {
	input := Result{
		Index:     -1,
		Text:      text,
		Fragments: make([]Fragment, 0),
		Error:     nil,
	}
	return input
}

// RawTexts computes the non‑transformed segments of the original Text.
//
// Fragments are considered as ranges within the original Text identified by
// Fragment.Pos and Fragment.Len (character coordinates, in the same space as
// RawText.Pos and RawText.Len). RawTexts walks over the Text and returns every
// contiguous region not covered by any fragment.
//
// Behavior and assumptions:
//
//   - If there are no fragments, a single RawText covering the whole Text is returned.
//   - Fragments are copied, sorted by their Pos, and treated as a union of ranges.
//     Overlapping fragments or multiple variants at the same Pos are merged by
//     always advancing a cursor to the furthest end seen so far.
//   - Zero‑length fragments and fragments that fall completely outside the Text
//     are ignored.
//   - Out‑of‑range fragment bounds are clamped to [0, len(TextInRunes)] so that
//     RawTexts never panics even if the fragment coordinates are slightly off.
//
// The resulting slice is suitable for Render(), which interleaves transformed
// fragments with these RawText segments to reconstruct an output string.
func (r Result) RawTexts() RawTexts {
	raw := make(RawTexts, 0)
	// Work in rune space so that positions and lengths are expressed in
	// characters (not bytes) for UTF‑8 text.
	runes := []rune(string(r.Text))
	textLen := len(runes)

	// Empty text: nothing to return.
	if textLen == 0 {
		return raw
	}

	// No fragments: the whole text is raw.
	if len(r.Fragments) == 0 {
		raw = append(raw, RawText{
			Text: r.Text,
			Pos:  0,
			Len:  textLen,
		})
		return raw
	}

	// Copy and sort fragments by start position to compute the union of their
	// covered ranges in a single pass.
	fragments := make([]Fragment, len(r.Fragments))
	copy(fragments, r.Fragments)

	sort.Slice(fragments, func(i, j int) bool {
		if fragments[i].Pos == fragments[j].Pos {
			// Tie‑break on length to provide a stable ordering; the actual
			// value has no semantic impact because we merge ranges via the
			// cursor logic below.
			return fragments[i].Len < fragments[j].Len
		}
		return fragments[i].Pos < fragments[j].Pos
	})

	// Cursor points to the first rune index that has not yet been classified
	// as belonging to a fragment.
	cursor := 0

	for _, f := range fragments {
		if f.Len <= 0 {
			// Ignore zero or negative length fragments.
			continue
		}

		start := f.Pos
		end := f.Pos + f.Len

		// Clamp the fragment to the valid [0, textLen] interval.
		if start < 0 {
			start = 0
		}
		if start >= textLen {
			// Starts beyond the end of the text: nothing to do.
			continue
		}
		if end > textLen {
			end = textLen
		}

		// Any gap between the cursor and the start of the fragment is raw text.
		if cursor < start {
			raw = append(raw, RawText{
				Text: UTF8String(string(runes[cursor:start])),
				Pos:  cursor,
				Len:  start - cursor,
			})
		}

		// Advance the cursor to the end of the fragment, but never move it
		// backwards. This naturally merges overlapping fragments or multiple
		// variants starting at the same position.
		if cursor < end {
			cursor = end
		}
	}

	// Trailing text after the last fragment is also raw.
	if cursor < textLen {
		raw = append(raw, RawText{
			Text: UTF8String(string(runes[cursor:textLen])),
			Pos:  cursor,
			Len:  textLen - cursor,
		})
	}
	return raw
}

// Render merges the phonetized fragments and the raw text segments back into
// a single output string. The reconstruction follows the original positional
// indices (Pos) to ensure the correct ordering.
//
// Rules for reconstruction:
//   - Both fragments and raw texts reference absolute positions in the original string.
//   - We collect all segments into a common list annotated with their start Pos.
//   - Segments are sorted by Pos to restore the original sequence.
//   - Fragment output uses Fragment.Phonetized.String().
//   - RawText output uses RawText.Text.
//   - No modification or transformation is done on the text content itself.
func (r Result) Render() UTF8String {
	// A small struct to unify fragments and raw texts during reconstruction.
	type segment struct {
		pos  int
		text UTF8String
	}
	rawTexts := r.RawTexts()
	segments := make([]segment, 0, len(r.Fragments)+len(rawTexts))

	lastFrag := Fragment{
		Pos: -1,
	}
	// Convert all fragments to reconstruction segments.
	for _, f := range r.Fragments {
		if f.Pos != lastFrag.Pos {
			// Phonetized.String() returns the human-readable representation of the phonetic form.
			segments = append(segments, segment{
				pos:  f.Pos,
				text: f.Transformed,
			})
			lastFrag.Pos = f.Pos
		}
	}

	// Convert all raw texts to reconstruction segments.
	for _, raw := range rawTexts {
		segments = append(segments, segment{
			pos:  raw.Pos,
			text: raw.Text,
		})
	}

	// Sort by position to ensure correct ordering.
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].pos < segments[j].pos
	})

	// Merge the ordered segments into the final output string.
	// The segments are assumed to cover the whole relevant reconstructed output.
	var out strings.Builder
	for _, seg := range segments {
		out.WriteString(string(seg.text))
	}
	return UTF8String(out.String())
}
