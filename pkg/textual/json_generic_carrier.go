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

// JsonGenericCarrier is a generic Carrier that encodes a typed Value with "json".
// It is the typed counterpart of JsonCarrier
type JsonGenericCarrier[T any] struct {
	Value T     `json:"value"`
	Index int   `json:"index,omitempty"`
	Error error `json:"error,omitempty"`
}

func (s JsonGenericCarrier[T]) UTF8String() UTF8String {
	b, err := json.Marshal(s)
	if err != nil {
		return err.Error()
	}
	return UTF8String(b)
}

func (s JsonGenericCarrier[T]) FromUTF8String(str UTF8String) JsonGenericCarrier[T] {
	// Note: no JsonCarrier validation is performed here; the carrier only transports bytes.
	proto := *new(JsonGenericCarrier[T])
	err := json.Unmarshal([]byte(str), &proto)
	if err != nil {
		proto.Error = err
	}
	return proto
}

func (s JsonGenericCarrier[T]) WithIndex(idx int) JsonGenericCarrier[T] {
	s.Index = idx
	return s
}

func (s JsonGenericCarrier[T]) GetIndex() int {
	return s.Index
}

func (s JsonGenericCarrier[T]) WithError(err error) JsonGenericCarrier[T] {
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

func (s JsonGenericCarrier[T]) GetError() error {
	return s.Error
}
