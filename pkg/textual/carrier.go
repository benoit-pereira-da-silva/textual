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

// UTF8String is a symbolic alias used throughout the package.
//
// textual can decode / encode many legacy encodings at the boundaries, but once
// inside the pipeline every piece of text is represented as UTF‑8.
//
// Note: this is an alias (not a distinct type) and exists mostly for code
// readability.
type UTF8String = string

// Carrier is the “carrier” contract used by the generic pipeline.
//
// The stack (Processor, Chain, Router, IOReaderProcessor, Transformation, …)
// is parameterized by a type S that implements Carrier[S].
//
// A carrier can be as small as a string wrapper (see textual.String) or a rich
// structure that keeps track of partial transformations and alternatives (see
// textual.Parcel).
//
// Method expectations:
//
//   - UTF8String returns the current UTF‑8 representation of the carrier.
//     For rich carriers this usually means “rendering” the current state into
//     a flat string.
//
//   - FromUTF8String creates a new carrier from a UTF‑8 token. The receiver is
//     treated as a prototype: most code calls it on the zero value of S, so it
//     must not rely on receiver state.
//
//   - WithIndex / GetIndex attach and retrieve an ordering hint.
//     IOReaderProcessor uses it to record the token sequence index. Aggregate
//     may use it to reassemble outputs in a stable order.
//
//   - Aggregate combines multiple carrier values into a single value.
//     This is used when a processor emits several outputs for one logical input
//     (split, fan‑out/fan‑in, etc.).
//
//   - WithError / GetError attach and retrieve a non-fatal, per-item error.
//     This enables processors to report recoverable issues (warnings, partial
//     failures, fallbacks…) without breaking the stream.
//
//     Important: errors carried by S are *data*, not control-flow. Most of the
//     textual stack does not stop when GetError() != nil. It is up to your
//     processors and/or the final consumer to decide how to handle error-carrying
//     items (route them, log them, drop them, etc.).
//
// Implementations should be cheap to copy (typically small structs). Pointer
// receivers/types are supported, but methods must be safe to call on the zero
// value (including nil pointers).
type Carrier[S any] interface {
	UTF8String() UTF8String
	FromUTF8String(s UTF8String) S
	WithIndex(index int) S
	GetIndex() int
	Aggregate(items []S) S
	WithError(err error) S
	GetError() error
}
