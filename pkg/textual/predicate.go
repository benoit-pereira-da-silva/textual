package textual

import "context"

// Predicate represents a function that evaluates whether a given item satisfies certain conditions.
// It takes a context and an input of type S (a Carrier) and returns a boolean indicating acceptance.
type Predicate[S Carrier[S]] func(ctx context.Context, item S) bool
