package textual

import (
	"sort"
	"strings"
)

// String is the minimal UTF8Stringer implementation.
// It just relies on an UTF8 value.
// The index is used for ordered aggregation.
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
