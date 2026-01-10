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
	"context"
	"runtime/debug"
)

// closedChan returns a channel that is already closed.
//
// It is used as a safe fallback when a Processor/Transcoder violates the contract
// by returning a nil channel or when a panic is recovered.
func closedChan[T any]() <-chan T {
	ch := make(chan T)
	close(ch)
	return ch
}

// safeCloseChan closes ch and captures any panic into ps.
//
// This protects pipeline infrastructure from contract violations where a downstream
// stage incorrectly closes an input channel it did not create.
func safeCloseChan[T any](ps *PanicStore, ch chan T) {
	if ch == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			if ps != nil {
				ps.Store(r, debug.Stack())
			}
		}
	}()
	close(ch)
}

// safeApplyProcessor calls p.Apply(ctx, in) defensively:
//
//   - recovers panics and stores them into ps,
//   - enforces the non-nil output channel contract,
//   - returns a closed channel on failure.
//
// ok is false when a panic was recovered or when the processor returned a nil
// output channel (contract violation).
func safeApplyProcessor[S Carrier[S]](ctx context.Context, ps *PanicStore, p Processor[S], in <-chan S) (out <-chan S, ok bool) {
	ok = true

	// Nil processor: behave as pass-through (handled elsewhere in most APIs, but
	// keep this helper robust).
	if p == nil {
		return in, true
	}

	defer func() {
		if r := recover(); r != nil {
			ok = false
			if ps != nil {
				ps.Store(r, debug.Stack())
			}
			out = closedChan[S]()
		}
	}()

	out = p.Apply(ctx, in)
	if out == nil {
		ok = false
		if ps != nil {
			ps.Store("textual: Processor.Apply returned a nil channel", debug.Stack())
		}
		out = closedChan[S]()
	}
	return out, ok
}
