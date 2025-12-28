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

package carrier

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

// JSON is a minimal Carrier implementation that transports a single JSON value.
//
// It is useful when your pipeline operates on raw JSON values instead of plain
// UTF‑8 text. The Value field holds the raw JSON bytes for one top-level value
// (typically an object `{...}` or array `[...]`).
//
// Aggregate concatenates multiple JSON values into a single JSON array.
type JSON struct {
	Value json.RawMessage `json:"value"`
	Index int             `json:"index,omitempty"`
	Error error           `json:"error,omitempty"`
}

func (s JSON) UTF8String() UTF8String {
	// Value is expected to contain UTF‑8 JSON bytes.
	return UTF8String(string(s.Value))
}

func (s JSON) FromUTF8String(str UTF8String) JSON {
	// Note: no JSON validation is performed here; the carrier only transports bytes.
	return JSON{
		Value: json.RawMessage([]byte(str)),
		Index: 0,
		Error: nil,
	}
}

func (s JSON) WithIndex(idx int) JSON {
	s.Index = idx
	return s
}

func (s JSON) GetIndex() int {
	return s.Index
}

// Aggregate concatenates multiple JSON values into a JSON array.
//
// The input slice is copied and stably sorted by Index, so callers can emit
// out-of-order fragments and still obtain a deterministic output.
//
// When indices are equal, the Value is used as a tie-breaker to keep the sort
// stable and deterministic.
//
// Errors from all inputs are merged (using errors.Join) and attached to the
// returned value.
func (s JSON) Aggregate(values []JSON) JSON {
	items := make([]JSON, len(values))
	copy(items, values)

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Index != items[j].Index {
			return items[i].Index < items[j].Index
		}
		return string(items[i].Value) < string(items[j].Value)
	})

	// Precompute capacity: sum of elements + brackets + commas.
	total := 2 // '[' + ']'
	if len(items) > 1 {
		total += len(items) - 1 // commas
	}
	for _, it := range items {
		total += len(it.Value)
	}

	var b strings.Builder
	b.Grow(total)

	var aggErr error

	b.WriteString("[")
	for i, it := range items {
		if i > 0 {
			b.WriteString(",")
		}
		b.Write(it.Value)
		if it.Error != nil {
			aggErr = errors.Join(aggErr, it.Error)
		}
	}
	b.WriteString("]")

	return JSON{Value: json.RawMessage(b.String()), Index: 0, Error: aggErr}
}

func (s JSON) WithError(err error) JSON {
	if err == nil {
		return s
	}
	if s.Error == nil {
		s.Error = err
	} else {
		s.Error = errors.Join(s.Error, err)
	}
	return s
}

func (s JSON) GetError() error {
	return s.Error
}
