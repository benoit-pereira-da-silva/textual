package textual

// UTF8String is used for code expressivity.
// We manipulate many encodings, but our internal string representation is always in utf8
type UTF8String = string

// UTF8Stringer defines an interface for types that can
// + convert to and from UTF8String representations
// + associate with an index (order in a text)
// + and aggregate multiple instances.
// It is implemented by:
//   - textual.String a minimal example.
//   - textual.Result a more complex structure, used by processor that may partially process the string and expose variants.
//
// You can implement your own textual.UTF8Stringer type to benefit from the stack (Processor, Transformation, Router)
type UTF8Stringer[S any] interface {
	UTF8String() UTF8String
	FromUTF8String(s UTF8String) S
	WithIndex(index int) S
	GetIndex() int
	Aggregate(stringers []S) S
}
