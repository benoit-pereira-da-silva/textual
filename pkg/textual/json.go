package textual

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

type JSON struct {
	Value json.RawMessage
	Index int
	Error error
}

func (s JSON) UTF8String() UTF8String {
	return UTF8String(s.Value)
}

func (s JSON) FromUTF8String(str UTF8String) String {
	return String{
		Value: str,
		Index: 0,
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
func (s JSON) Aggregate(lines []JSON) JSON {
	items := make([]JSON, len(lines))
	copy(items, lines)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Index != items[j].Index {
			return items[i].Index < items[j].Index
		}
		return string(items[i].Value) < string(items[j].Value)
	})
	total := 0
	for _, it := range items {
		total += len(it.Value)
	}
	var b strings.Builder
	b.Grow(total)
	var aggErr error
	b.WriteString("[")
	for _, it := range items {
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
