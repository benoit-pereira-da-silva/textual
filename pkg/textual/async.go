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

// Async starts a single-worker streaming "map" stage.
//
// It consumes values from `in`, applies `f` to each value, and sends the
// resulting values to the returned channel.
//
// Async is primarily intended as a low-ceremony building block to implement
// Processor and Transcoder stages in this package's channel-based pipeline model.
//
// -----------------------------------------------------------------------------
// Streaming contract
//
//   - Async NEVER closes `in`. The upstream stage owns the input channel.
//   - Async closes the returned channel exactly once, when it is done.
//   - The worker goroutine exits when:
//   - ctx is canceled (ctx.Done() is closed), OR
//   - `in` is closed by upstream, OR
//   - `f` panics (the panic is recovered; see "Panic handling").
//   - Async emits at most one output for each input (1:1 mapping).
//     If you need fan-out (1:N) or fan-in, write a custom stage or use Router.
//
// -----------------------------------------------------------------------------
// Context cancellation and interruption
//
// Async is designed for pipelines where the context is the *out-of-band*
// interruption signal.
//
// Key properties:
//
//   - Every receive and every send is performed in a select that also watches
//     ctx.Done().
//   - This prevents goroutine leaks when downstream stops consuming.
//   - If the final consumer wants to stop early, it must cancel the context.
//     Simply "breaking" from a for-range on the output channel without canceling
//     can leave upstream goroutines blocked on sends.
//
// Because `f` does not take a context parameter, cancellation is cooperative.
// If `f` calls APIs that accept a context, capture `ctx` in the closure:
//
//	out := Async(ctx, in, func(v T1) T2 {
//	    // Use ctx inside the mapping if you need cancellation-aware work.
//	    res, err := doSomething(ctx, v)
//	    _ = err
//	    return res
//	})
//
// -----------------------------------------------------------------------------
// Backpressure and buffering
//
// Async returns an unbuffered output channel. This is deliberate: it makes
// backpressure explicit and keeps memory bounded. A slow consumer will slow
// down the whole upstream pipeline.
//
// If you need buffering, insert it explicitly (e.g. a stage that forwards into
// a buffered channel) or scale out explicitly (Router + multiple workers).
//
// -----------------------------------------------------------------------------
// Panic handling (PanicStore)
//
// Async recovers any panic raised by `f` (or by code it calls).
//
// If ctx carries a *PanicStore* (see WithPanicStore), the first recovered panic
// is stored there together with a stack trace (runtime/debug.Stack).
//
// For safety, Async also ensures that a PanicStore exists: if ctx does not carry
// one, Async attaches a new PanicStore to an internal derived context so the
// recovery path is never a silent no-op.
//
// Important: if Async had to create that PanicStore itself, the store is not
// observable by the caller (Async does not return the derived context).
// For production code, attach a store at the pipeline boundary and keep the
// returned *PanicStore* so you can surface failures deterministically.
//
// The panic is NOT rethrown: the worker simply stops and closes the output
// channel.
//
// This behavior keeps streaming pipelines from crashing the whole process, but
// it also means that panics become an out-of-band signal that MUST be checked
// by the pipeline supervisor.
//
// Recommended supervision pattern:
//
//	base, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	ctx, ps := WithPanicStore(base)
//
//	out := Async(ctx, in, f)
//	for v := range out {
//	    _ = v
//	}
//
//	if info, ok := ps.Load(); ok {
//	    // Treat as fatal: log, convert to error, or re-panic with stack.
//	    // log.Printf("panic: %v\n%s", info.Value, info.Stack)
//	}
//
// -----------------------------------------------------------------------------
// Implementing Processor / Transcoder with Async
//
// Async is particularly convenient for 1:1 stages:
//
//	p := ProcessorFunc[carrier.String](func(ctx context.Context, in <-chan carrier.String) <-chan carrier.String {
//	    return Async(ctx, in, func(s carrier.String) carrier.String {
//	        s.Value = strings.ToUpper(s.Value)
//	        return s
//	    })
//	})
//
//	t := TranscoderFunc[carrier.String, carrier.Parcel](func(ctx context.Context, in <-chan carrier.String) <-chan carrier.Parcel {
//	    proto := carrier.Parcel{}
//	    return Async(ctx, in, func(s carrier.String) carrier.Parcel {
//	        return proto.FromUTF8String("P:" + s.Value).WithIndex(s.GetIndex())
//	    })
//	})
//
// -----------------------------------------------------------------------------
// Discipline required (the "rules of the road")
//
// The channel + context pipeline model is powerful but unforgiving:
//
//   - Always provide a cancellable context to the pipeline and call its cancel
//     function when you stop consuming early.
//   - Always drain the returned channel (or cancel ctx) to let goroutines exit.
//   - Never close an input channel you did not create.
//   - Make sure every stage (including custom ones) selects on ctx.Done() when
//     receiving AND when sending.
//   - Treat PanicStore as a mandatory error channel: if you ignore it, panics
//     become silent data loss.
//
// When used with those rules, Async provides predictable resource lifetime,
// bounded memory via backpressure, simple stage composition, and panic
// containment across goroutines.
func Async[T1 any, T2 any](ctx context.Context, in <-chan T1, f func(t T1) T2) <-chan T2 {
	if ctx == nil {
		ctx = context.Background()
	}

	// Ensure a PanicStore is present so that recovered panics are never silently
	// dropped. If the caller didn't provide one, we attach an internal store.
	if PanicStoreFromContext(ctx) == nil {
		ctx, _ = WithPanicStore(ctx)
	}

	// Derive a cancellable child context so ctx.Done() is always non-nil and
	// so we can always call cancel() on exit to release context resources.
	//
	// Note: canceling this child context does NOT cancel the parent context; it
	// only signals this stage (and any goroutines derived from it).
	ctx, cancel := context.WithCancel(ctx)

	out := make(chan T2)
	go func() {
		defer close(out)

		// Always cancel the child context so the context tree can be released
		// promptly (best practice with context.WithCancel / WithTimeout).
		defer cancel()

		// Recover panics in this worker and store them (PanicStore is always present).
		// The panic is swallowed: Async terminates the stream early by closing `out`.
		defer func() {
			if r := recover(); r != nil {
				if ps := PanicStoreFromContext(ctx); ps != nil {
					ps.Store(r, debug.Stack())
				}
				// No re-panic: let the pipeline supervisor decide how to surface
				// the failure (log, cancel the root context, return an error, ...).
				return
			}
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case s, ok := <-in:
				if !ok {
					return
				}

				// Any panic in f(s) is recovered by the defer above.
				res := f(s)

				select {
				case <-ctx.Done():
					return
				case out <- res:
				}
			}
		}
	}()
	return out
}
