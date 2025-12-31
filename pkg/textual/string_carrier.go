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
)

// StringCarrier is a simple Carrier implementation.
//
// It is useful when you only need to stream UTF-8 text through processors and
// you don't need partial spans, variants, or per-token metadata beyond an
// optional ordering index and an optional per-item error.
//
// Index is an ordering hint used by Aggregate (and by IOReaderProcessor, which
// sets it to the token sequence number). Value carries the UTF-8 text.
// Error carries a non-fatal processing error attached by processors.
//
// StringCarrier implements Carrier[StringCarrier] and can be used with the generic stack
// (Processor, Chain, Router, Transformation, ...).
type StringCarrier struct {
	Value string
	Index int
	Error error
}

func (s StringCarrier) UTF8String() UTF8String {
	return s.Value
}

func (s StringCarrier) FromUTF8String(str UTF8String) StringCarrier {
	return StringCarrier{
		Value: str,
		Index: 0,
	}
}

func (s StringCarrier) WithIndex(idx int) StringCarrier {
	s.Index = idx
	return s
}

func (s StringCarrier) GetIndex() int {
	return s.Index
}

func (s StringCarrier) WithError(err error) StringCarrier {
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

func (s StringCarrier) GetError() error {
	return s.Error
}
