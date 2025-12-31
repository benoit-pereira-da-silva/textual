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
)

// JsonCarrier is a minimal dynamic Carrier implementation that transports an opaque JsonCarrier value.
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
