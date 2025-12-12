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

// String is the minimal UTF8Stringer implementation.
//
// It is useful when you only need to stream UTF-8 text through processors and
// you don't need partial spans, variants, or per-token metadata beyond an
// optional ordering index.
//
// Index is an ordering hint used by Aggregate (and by IOReaderProcessor, which
// sets it to the token sequence number). Value carries the UTF-8 text.
//
// String implements UTF8Stringer[String] and can be used with the generic stack
// (Processor, Chain, Router, Transformation, ...).
type String struct {
	Value string
	Index int
}

func (s String) UTF8String() UTF8String {
	return s.Value
}

func (s String) FromUTF8String(str UTF8String) String {
	return String{
		Value: str,
		Index: 0,
	}
}

func (s String) WithIndex(idx int) String {
	s.Index = idx
	return s
}

func (s String) GetIndex() int {
	return s.Index
}

// Aggregate concatenates multiple String values into one.
//
// The input slice is copied and stably sorted by Index, so callers can emit
// out-of-order fragments and still obtain a deterministic output.
//
// When indices are equal, the Value is used as a tie-breaker to keep the sort
// stable and deterministic.
func (s String) Aggregate(stringers []String) String {
	items := make([]String, len(stringers))
	copy(items, stringers)

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Index != items[j].Index {
			return items[i].Index < items[j].Index
		}
		return items[i].Value < items[j].Value
	})

	total := 0
	for _, it := range items {
		total += len(it.Value)
	}

	var b strings.Builder
	b.Grow(total)

	for _, it := range items {
		b.WriteString(it.Value)
	}

	return String{Value: b.String(), Index: 0}
}
