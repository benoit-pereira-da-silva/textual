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
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

// JsonCarrier is a minimal dynamic Carrier and AggregatableCarrier implementation that transports an opaque JsonCarrier value.
// When the type is stable, use a generic JsonGenericCarrier
// Else you can use JsonCarrier and cast according to context or in cascade using the facility CastJson.
//
// It is useful when your pipeline operates on raw JsonCarrier values instead of plain
// UTF‑8 text. The Value field holds the raw JsonCarrier bytes for one top-level value
// (typically an object `{...}` or array `[...]`).
//
// Aggregate concatenates multiple JsonCarrier values into a single JsonCarrier array.
type JsonCarrier struct {
	Value json.RawMessage `json:"value"`
	Index int             `json:"index,omitempty"`
	Error error           `json:"error,omitempty"`
}

func (s JsonCarrier) UTF8String() UTF8String {
	// Value is expected to contain UTF‑8 JsonCarrier bytes.
	return UTF8String(s.Value)
}

func (s JsonCarrier) FromUTF8String(str UTF8String) JsonCarrier {
	// Note: no JsonCarrier validation is performed here; the carrier only transports bytes.
	return JsonCarrier{
		Value: json.RawMessage([]byte(str)),
		Index: 0,
		Error: nil,
	}
}

func (s JsonCarrier) WithIndex(idx int) JsonCarrier {
	s.Index = idx
	return s
}

func (s JsonCarrier) GetIndex() int {
	return s.Index
}

func (s JsonCarrier) WithError(err error) JsonCarrier {
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

func (s JsonCarrier) GetError() error {
	return s.Error
}

///////////////////////////////////////
// AggregatableCarrier implementation
///////////////////////////////////////

// Aggregate concatenates multiple JsonCarrier values into a JsonCarrier array.
//
// The input slice is copied and stably sorted by Index, so callers can emit
// out-of-order fragments and still obtain a deterministic output.
//
// When indices are equal, the Value is used as a tie-breaker to keep the sort
// stable and deterministic.
//
// Errors from all inputs are merged (using errors.Join) and attached to the
// returned value.
func (s JsonCarrier) Aggregate(values []JsonCarrier) JsonCarrier {
	items := make([]JsonCarrier, len(values))
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

	return JsonCarrier{Value: json.RawMessage(b.String()), Index: 0, Error: aggErr}
}
