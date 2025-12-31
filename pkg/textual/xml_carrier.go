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

// XmlCarrier is a minimal Carrier and AggregatableCarrier implementation that transports an
// opaque XML fragment.
//
// Intended usage:
//
//   - Each XmlCarrier.Value contains one complete top-level XML element,
//     encoded as UTF-8.
//
// textual.ScanXML produces tokens with exactly that shape (it ignores leading
// prolog / PI / comments / doctype and returns one complete element).
//
// Aggregate concatenates multiple XmlCarrier fragments into one well-formed XML
// document by wrapping them inside a container element "<items>...</items>"
// after stably sorting by Index.
//
// Important:
//   - Aggregation assumes inputs are elements (not full documents with XML declarations).
//   - No additional whitespace is inserted between elements.
type XmlCarrier struct {
	Value UTF8String `json:"value"`
	Index int        `json:"index,omitempty"`
	Error error      `json:"error,omitempty"`
}

func (s XmlCarrier) UTF8String() UTF8String {
	return s.Value
}

func (s XmlCarrier) FromUTF8String(str UTF8String) XmlCarrier {
	// Note: no XML validation is performed here; the carrier only transports bytes.
	return XmlCarrier{
		Value: str,
		Index: 0,
		Error: nil,
	}
}

func (s XmlCarrier) WithIndex(idx int) XmlCarrier {
	s.Index = idx
	return s
}

func (s XmlCarrier) GetIndex() int {
	return s.Index
}

func (s XmlCarrier) WithError(err error) XmlCarrier {
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

func (s XmlCarrier) GetError() error {
	return s.Error
}

///////////////////////////////////////
// AggregatableCarrier implementation
///////////////////////////////////////

// Aggregate concatenates multiple XML elements into a single document wrapped
// in a container element "<items>...</items>".
//
// The input slice is copied and stably sorted by Index, so callers can emit
// out-of-order fragments and still obtain a deterministic output.
//
// When indices are equal, the Value is used as a tie-breaker to keep the sort
// stable and deterministic.
//
// Errors from all inputs are merged (using errors.Join) and attached to the
// returned value.
func (s XmlCarrier) Aggregate(values []XmlCarrier) XmlCarrier {
	items := make([]XmlCarrier, len(values))
	copy(items, values)

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Index != items[j].Index {
			return items[i].Index < items[j].Index
		}
		return items[i].Value < items[j].Value
	})

	const open = "<items>"
	const close = "</items>"

	total := len(open) + len(close)
	for _, it := range items {
		total += len(it.Value)
	}

	var b strings.Builder
	b.Grow(total)

	var aggErr error

	b.WriteString(open)
	for _, it := range items {
		b.WriteString(string(it.Value))
		if it.Error != nil {
			aggErr = errors.Join(aggErr, it.Error)
		}
	}
	b.WriteString(close)

	return XmlCarrier{Value: b.String(), Index: 0, Error: aggErr}
}
