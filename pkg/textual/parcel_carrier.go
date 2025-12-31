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
	"errors"
	"sort"
	"strings"
)

// Parcel is a Carrier and AggregatableCarrier implementation designed for partial transformations.
//
// It keeps the original input (`Text`) and a set of transformed spans
// (`Fragments`). Each fragment references a rune-based range within `Text`
// using (Pos, Len). The remaining, unprocessed parts of `Text` can be derived
// via RawTexts().
//
// Processors can use Parcel when they:
//
//   - only transform some expressions inside a token,
//   - need to keep per-span metadata (confidence, variant, ...), or
//   - want to propagate errors while keeping the stream alive.
//
// Parcel implements Carrier[Parcel], which means it can flow through the
// generic stack (Processor, Chain, Router, Transformation, ...).
//
// Index is an optional ordering hint used by aggregators (e.g. when reassembling
// split outputs). A value of -1 typically means "unset".
//
// Note on variants:
//
// Multiple fragments can share the same Pos (for example different candidates
// for the same span). Parcel.UTF8String() currently renders at most one fragment
// per Pos (the first encountered for that position). If you need to pick a
// specific variant, filter / sort Fragments first.
type Parcel struct {
	Index     int        `json:"index,omitempty"` // Optional order in a stream (token index). -1 means unset.
	Text      UTF8String `json:"text"`            // Original text (UTF-8).
	Fragments []Fragment `json:"fragments"`       // Transformed spans within Text.
	Error     error      `json:"error,omitempty"` // Optional processing error.
}

// Fragment describes a transformed span inside a Parcel.
//
// Pos and Len are expressed in runes (character indices) relative to Parcel.Text.
// This makes them stable for UTF-8 text.
//
// Variant can be used to represent multiple candidates for the same span
// (e.g. alternative phonetic renderings).
// Confidence is an arbitrary score (0..1 by convention) produced by the
// processor.
type Fragment struct {
	Transformed UTF8String `json:"transformed"` // Transformed text (dialect-specific: IPA, SAMPA, pseudo phonetics, ...).
	Pos         int        `json:"pos"`         // Start position (rune index) in the original text.
	Len         int        `json:"len"`         // Length (in runes) of the original span.
	Confidence  float64    `json:"confidence"`  // Confidence score.
	Variant     int        `json:"variant"`     // Variant number when offering multiple candidates.
}

// RawTexts is a set of raw (non-transformed) segments derived from a Parcel.
//
// It is computed by subtracting fragment ranges from Parcel.Text. See RawTexts()
// for details.
type RawTexts []RawText

// RawText is a remaining (non-transformed) segment of the original text.
//
// Pos and Len are expressed in runes (character indices) relative to Parcel.Text.
type RawText struct {
	Text UTF8String `json:"text"` // Remaining raw text.
	Pos  int        `json:"pos"`  // Start position (rune index) in the original text.
	Len  int        `json:"len"`  // Length (in runes).
}

func (r Parcel) FromUTF8String(s UTF8String) Parcel {
	return Parcel{
		Index:     -1,
		Text:      s,
		Fragments: make([]Fragment, 0),
		Error:     nil,
	}
}

func (r Parcel) WithIndex(idx int) Parcel {
	r.Index = idx
	return r
}

func (r Parcel) GetIndex() int {
	return r.Index
}

// UTF8String reconstructs a plain string by interleaving transformed fragments
// and raw text segments.
//
// Reconstruction rules:
//   - Both fragments and raw texts reference absolute rune positions in the
//     original string.
//   - All segments are collected into a common list annotated with their start
//     Pos.
//   - Segments are sorted by Pos to restore the original sequence.
//   - Fragment output uses Fragment.Transformed.
//   - RawText output uses RawText.Text.
//
// No additional transformation is performed: this is only a positional merge.
func (r Parcel) UTF8String() UTF8String {
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
	// Convert fragments to reconstruction segments.
	for _, f := range r.Fragments {
		if f.Pos != lastFrag.Pos {
			segments = append(segments, segment{
				pos:  f.Pos,
				text: f.Transformed,
			})
			lastFrag.Pos = f.Pos
		}
	}

	// Convert raw texts to reconstruction segments.
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
	var out strings.Builder
	for _, seg := range segments {
		out.WriteString(string(seg.text))
	}
	return UTF8String(out.String())
}

func (r Parcel) WithError(err error) Parcel {
	if err == nil {
		return r
	}
	if r.Error == nil {
		r.Error = err
	} else {
		r.Error = errors.Join(r.Error, err)
	}
	return r
}
func (r Parcel) GetError() error {
	return r.Error
}

///////////////////////////////////////
// AggregatableCarrier implementation
///////////////////////////////////////

// Aggregate concatenates the Text fields of results and rebases all fragment
// positions into the coordinate space of the aggregated Text.
//
// Errors are merged by taking the first non-nil error.
func (r Parcel) Aggregate(parcels []Parcel) Parcel {
	var aggregated Parcel
	if len(parcels) == 0 {
		return aggregated
	}
	var builder strings.Builder

	// Precompute capacity and first error, if any.
	totalFragments := 0
	for _, res := range parcels {
		totalFragments += len(res.Fragments)
		if aggregated.Error == nil && res.Error != nil {
			aggregated.Error = res.Error
		}
	}
	aggregated.Fragments = make([]Fragment, 0, totalFragments)
	aggregated.Index = -1
	offset := 0 // rune offset in the aggregated Text

	for _, res := range parcels {
		textStr := res.Text
		builder.WriteString(textStr)

		// Compute the rune length for offset rebasing.
		runeLen := len([]rune(textStr))

		for _, f := range res.Fragments {
			adjusted := f
			adjusted.Pos += offset
			aggregated.Fragments = append(aggregated.Fragments, adjusted)
		}
		offset += runeLen
	}
	aggregated.Text = builder.String()
	return aggregated
}

/////////////////////////////////
//
//
/////////////////////////////////

// RawTexts computes the non-transformed segments of the original Text.
//
// Fragments are treated as rune ranges within the original Text identified by
// Fragment.Pos and Fragment.Len. RawTexts walks over the Text and returns every
// contiguous region not covered by any fragment.
//
// Behavior and assumptions:
//
//   - If there are no fragments, a single RawText covering the whole Text is returned.
//   - Fragments are copied, sorted by Pos, and treated as a union of ranges.
//     Overlapping fragments or multiple variants at the same Pos are merged by
//     always advancing a cursor to the furthest end seen so far.
//   - Zero-length fragments and fragments that fall completely outside the Text
//     are ignored.
//   - Out-of-range fragment bounds are clamped to [0, len(TextInRunes)] so that
//     RawTexts never panics even if the fragment coordinates are slightly off.
//
// The resulting slice is suitable for UTF8String(), which interleaves
// transformed fragments with these raw segments to reconstruct an output string.
func (r Parcel) RawTexts() RawTexts {
	raw := make(RawTexts, 0)
	// Work in rune space so that positions and lengths are expressed in
	// characters (not bytes) for UTF-8 text.
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
			// Tie-break on length to provide a stable ordering; the actual
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

		// Advance the cursor to the end of the fragment but never move it
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
