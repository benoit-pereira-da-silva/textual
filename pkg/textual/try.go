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

import "context"

// HasError reports whether the carrier currently holds a non-nil per-item error.
//
// It is provided as a reusable predicate for Router / IF / Try wrappers.
func HasError[S Carrier[S]](ctx context.Context, item S) bool {
	_ = ctx
	return item.GetError() != nil
}

// HasNoError reports whether the carrier currently holds a nil per-item error.
func HasNoError[S Carrier[S]](ctx context.Context, item S) bool {
	_ = ctx
	return item.GetError() == nil
}

// TryCatchFinally is a Processor wrapper that turns the Carrier error field
// into a control-flow construct similar to try/catch/finally.
//
// In textual, per-item errors are normally *data* (the stream continues).
// TryCatchFinally lets you treat item.GetError() != nil as a "throw":
//
//   - Items that already carry an error bypass the Try block.
//   - While executing the Try block, as soon as an item gains an error,
//     it stops going through remaining Try processors ("throw" short-circuit).
//   - Thrown items are routed to Catch (if provided).
//   - Finally (if provided) always runs after Try/Catch.
//
// Index preservation:
//
// This wrapper never clones or rebuilds carriers. It only routes existing
// items through processors (or forwards them unchanged). Therefore the Carrier
// index is preserved by the wrapper itself.
//
// Note: as usual, any Processor that *creates* new carriers must preserve the
// index itself (typically by calling WithIndex on its outputs).
type TryCatchFinally[S Carrier[S]] struct {
	tryProcessors     []Processor[S]
	catchProcessors   []Processor[S]
	finallyProcessors []Processor[S]
}

// Try starts a Try/Catch/Finally builder for Processor pipelines.
//
// Usage:
//
//	p := Try[S](tryP1, tryP2).
//		Catch(catchP).
//		Finally(finallyP)
//
// Try returns a configured *TryCatchFinally that implements Processor[S] and
// can be inserted directly in a pipeline.
func Try[S Carrier[S]](try ...Processor[S]) *TryCatchFinally[S] {
	return &TryCatchFinally[S]{
		tryProcessors: try,
	}
}

// Catch sets (replaces) the catch processors executed for thrown items
// (items where GetError() != nil after Try, or already errored at input).
//
// If no catch processors are set, thrown items are simply forwarded to Finally.
func (t *TryCatchFinally[S]) Catch(catch ...Processor[S]) *TryCatchFinally[S] {
	if t == nil {
		t = &TryCatchFinally[S]{}
	}
	t.catchProcessors = catch
	return t
}

// Finally sets (replaces) the finally processors executed for *all* items,
// whether they were thrown or not.
//
// If no finally processors are set, the wrapper ends after Try/Catch.
func (t *TryCatchFinally[S]) Finally(finally ...Processor[S]) *TryCatchFinally[S] {
	if t == nil {
		t = &TryCatchFinally[S]{}
	}
	t.finallyProcessors = finally
	return t
}

// ProcessorFunc returns a compiled ProcessorFunc implementing the configured
// Try/Catch/Finally semantics.
//
// This is convenient when you want to further compose using ProcessorFunc.Then(...).
func (t *TryCatchFinally[S]) ProcessorFunc() ProcessorFunc[S] {
	// Freeze configuration (defensive copy). This avoids surprises if the builder
	// is mutated after being inserted in a pipeline.
	var tryProcs, catchProcs, finallyProcs []Processor[S]
	if t != nil {
		tryProcs = append([]Processor[S](nil), t.tryProcessors...)
		catchProcs = append([]Processor[S](nil), t.catchProcessors...)
		finallyProcs = append([]Processor[S](nil), t.finallyProcessors...)
	}

	// Build blocks.
	tryBlock := guardedTryChain[S](tryProcs...)
	catchBlock := blockOrNil[S](catchProcs...)
	finallyBlock := blockOrNil[S](finallyProcs...)

	// Compose:
	//
	// 1) Pre-try "throw": if item already has error, bypass Try.
	//    - THEN: pass-through (nil => pass-through inside IF)
	//    - ELSE: execute Try block
	//
	// 2) Post-try catch: if item has error, execute Catch.
	//
	// 3) Finally: always execute if provided.
	pre := If[S](HasError[S]).Then(nil).Else(tryBlock)
	post := If[S](HasError[S]).Then(catchBlock)

	p := ProcessorFuncFrom[S](pre).Chain(post)
	if finallyBlock != nil {
		p = p.Chain(finallyBlock)
	}
	return p
}

// Apply implements Processor[S] by delegating to ProcessorFunc().
func (t *TryCatchFinally[S]) Apply(ctx context.Context, in <-chan S) <-chan S {
	return t.ProcessorFunc().Apply(ctx, in)
}

// guardedTryChain composes processors left-to-right, but only applies each
// processor to items that do not carry an error at that point in the chain.
//
// This enforces the "throw short-circuit" semantics:
//
//   - once an item has GetError() != nil, remaining Try processors are skipped,
//     and the item is forwarded unchanged until it reaches Catch/Finally.
func guardedTryChain[S Carrier[S]](processors ...Processor[S]) ProcessorFunc[S] {
	// No processors: identity.
	if len(processors) == 0 {
		return ProcessorFunc[S](func(ctx context.Context, in <-chan S) <-chan S {
			_ = ctx
			return in
		})
	}
	return ProcessorFunc[S](func(ctx context.Context, in <-chan S) <-chan S {
		out := in
		for _, p := range processors {
			if p == nil {
				continue
			}
			// Only apply p when the item does not yet carry an error.
			out = If[S](HasError[S]).Then(nil).Else(p).Apply(ctx, out)
		}
		return out
	})
}

// blockOrNil returns a composed Processor for the given block processors.
//
// When the block is empty (or only contains nil processors), nil is returned.
// This allows IF(...) to fall back to pass-through for missing blocks.
func blockOrNil[S Carrier[S]](processors ...Processor[S]) Processor[S] {
	nonNil := false
	for _, p := range processors {
		if p != nil {
			nonNil = true
			break
		}
	}
	if !nonNil {
		return nil
	}
	return NewChain[S](processors...)
}
