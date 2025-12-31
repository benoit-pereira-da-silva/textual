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

// CsvCarrier is a minimal Carrier and AggregatableCarrier implementation that transports an
// opaque CSV record value.
//
// Semantics (mirrors JsonCarrier's role for JSON):
//
//   - CsvCarrier.Value holds the raw UTF-8 bytes for a single CSV record.
//     By convention, Value should NOT include the trailing record separator
//     (newline). textual.ScanCSV follows that convention.
//
//   - CsvCarrier does not interpret CSV dialect (delimiter, comments, ...).
//     It only transports bytes. Use CastCsvRecord (or encoding/csv directly)
//     to parse Value when needed.
//
// Aggregate concatenates multiple CsvCarrier values into a single CSV text by
// joining records with "\n" after stably sorting by Index.
//
// Note: This aggregation strategy intentionally does not add an XML-like wrapper
// or a JSON-like array: CSV’s natural “fan-in” representation is simply a
// multi-record CSV stream.
type CsvCarrier struct {
	Value UTF8String `json:"value"`
	Index int        `json:"index,omitempty"`
	Error error      `json:"error,omitempty"`
}

func (s CsvCarrier) UTF8String() UTF8String {
	return s.Value
}

func (s CsvCarrier) FromUTF8String(str UTF8String) CsvCarrier {
	// Note: no CSV validation is performed here; the carrier only transports bytes.
	return CsvCarrier{
		Value: str,
		Index: 0,
		Error: nil,
	}
}

func (s CsvCarrier) WithIndex(idx int) CsvCarrier {
	s.Index = idx
	return s
}

func (s CsvCarrier) GetIndex() int {
	return s.Index
}

func (s CsvCarrier) WithError(err error) CsvCarrier {
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

func (s CsvCarrier) GetError() error {
	return s.Error
}

///////////////////////////////////////
// AggregatableCarrier implementation
///////////////////////////////////////

// Aggregate concatenates multiple CSV records into a multi-record CSV text.
//
// The input slice is copied and stably sorted by Index, so callers can emit
// out-of-order fragments and still obtain a deterministic output.
//
// When indices are equal, the Value is used as a tie-breaker to keep the sort
// stable and deterministic.
//
// Errors from all inputs are merged (using errors.Join) and attached to the
// returned value.
func (s CsvCarrier) Aggregate(records []CsvCarrier) CsvCarrier {
	items := make([]CsvCarrier, len(records))
	copy(items, records)

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Index != items[j].Index {
			return items[i].Index < items[j].Index
		}
		return items[i].Value < items[j].Value
	})

	// Capacity: sum of record sizes + separators.
	total := 0
	if len(items) > 1 {
		total += len(items) - 1 // '\n' separators
	}
	for _, it := range items {
		total += len(it.Value)
	}

	var b strings.Builder
	b.Grow(total)

	var aggErr error
	for i, it := range items {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(string(it.Value))
		if it.Error != nil {
			aggErr = errors.Join(aggErr, it.Error)
		}
	}

	return CsvCarrier{Value: b.String(), Index: 0, Error: aggErr}
}
