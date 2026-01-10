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
)

/*
If / ELSEIf / ELSE for Processor

The If builder provides a fluent API to express per-item conditional routing
over a Processor pipeline:

	If(pred1).
		Then(p1).
		ElseIf(pred2, p2).
		Else(p3)

Semantics:

  - Branches are evaluated in order:
    If(...) is the first branch, then every ElseIf(...) branch in registration
    order, finally Else(...) as the fallback when provided.

  - The first predicate that matches selects the corresponding processor.
    A nil predicate is treated as "always matches".

  - ConditionalProc the selected processor is nil, If behaves as a pass-through for that
    branch (the item is forwarded unchanged).

  - ConditionalProc no predicate matches and Else(...) is not set (or is nil), If behaves as
    a pass-through.

Index preservation:

If never clones carriers and never rewrites indices. Items are routed to the
selected branch processor (or forwarded unchanged). Therefore, the Carrier
index is preserved by the If stage itself, regardless of which branch is taken.

Note: a downstream processor that *creates new carriers* (e.g. via FromUTF8String)
must preserve the index itself (typically by calling WithIndex on the output).
*/

// ConditionalProc is a Processor that routes each incoming item to the first matching branch
// processor (If / ELSEIf) or the fallback processor (ELSE).
//
// Configure it during pipeline construction; mutating it while Apply is running
// is not concurrency-safe (same as Router).
type ConditionalProc[S Carrier[S]] struct {
	branches      []ifBranch[S]
	elseProcessor Processor[S]
}

type ifBranch[S Carrier[S]] struct {
	predicate Predicate[S] // nil means "always matches"
	processor Processor[S] // nil means "pass-through"
}

// If starts a conditional Processor builder.
//
// The returned *ConditionalProc implements Processor[S] and can be inserted directly into
// a pipeline:
//
//	p := If(pred).Then(p1).Else(p2)
func If[S Carrier[S]](predicate Predicate[S]) *ConditionalProc[S] {
	return &ConditionalProc[S]{
		branches: []ifBranch[S]{
			{predicate: predicate},
		},
	}
}

// Then sets the Processor executed when the If predicate matches.
//
// ConditionalProc processor is nil, the branch becomes a pass-through.
func (c *ConditionalProc[S]) Then(processor Processor[S]) *ConditionalProc[S] {
	if c == nil {
		c = &ConditionalProc[S]{}
	}
	if len(c.branches) == 0 {
		c.branches = append(c.branches, ifBranch[S]{})
	}
	c.branches[0].processor = processor
	return c
}

// ElseIf appends a new conditional branch evaluated after If(...) and previous
// ElseIf(...) branches, but before Else(...).
//
// ConditionalProc processor is nil, the branch becomes a pass-through.
func (c *ConditionalProc[S]) ElseIf(predicate Predicate[S], processor Processor[S]) *ConditionalProc[S] {
	if c == nil {
		c = &ConditionalProc[S]{}
	}
	c.branches = append(c.branches, ifBranch[S]{
		predicate: predicate,
		processor: processor,
	})
	return c
}

// Else sets the fallback Processor executed when no If/ELSEIf predicate matches.
//
// ConditionalProc processor is nil, If behaves as a pass-through for non-matching items.
func (c *ConditionalProc[S]) Else(processor Processor[S]) *ConditionalProc[S] {
	if c == nil {
		c = &ConditionalProc[S]{}
	}
	c.elseProcessor = processor
	return c
}

// Apply implements Processor[S].
//
// Internally, Apply uses Router in RoutingStrategyFirstMatch mode.
// This provides robust fan-out/fan-in behavior while keeping If lightweight.
func (c *ConditionalProc[S]) Apply(ctx context.Context, in <-chan S) <-chan S {
	if ctx == nil {
		ctx = context.Background()
	}

	// Nil receiver: behave as pass-through.
	if c == nil {
		return passThroughProcessor[S]().Apply(ctx, in)
	}

	// No branches: only Else (or pass-through).
	if len(c.branches) == 0 {
		if c.elseProcessor != nil {
			return c.elseProcessor.Apply(ctx, in)
		}
		return passThroughProcessor[S]().Apply(ctx, in)
	}

	// Build a FirstMatch router from the branch list.
	r := NewRouter[S](RoutingStrategyFirstMatch)

	// Reuse a single pass-through processor instance for nil branches.
	pt := passThroughProcessor[S]()

	for _, br := range c.branches {
		proc := br.processor
		if proc == nil {
			proc = pt
		}
		r.AddRoute(br.predicate, proc)
	}

	// Else is implemented as a final always-eligible route.
	if c.elseProcessor != nil {
		r.AddProcessor(c.elseProcessor)
	}

	return r.Apply(ctx, in)
}

// passThroughProcessor returns a ProcessorFunc that forwards items unchanged.
//
// This is used internally to make nil branch processors behave as "pass-through"
// while still consuming the matching branch (i.e. it stops the ELSEIf chain).
func passThroughProcessor[S Carrier[S]]() ProcessorFunc[S] {
	return ProcessorFunc[S](func(ctx context.Context, in <-chan S) <-chan S {
		if ctx == nil {
			ctx = context.Background()
		}
		out := make(chan S)
		go func() {
			defer close(out)
			for {
				select {
				case <-ctx.Done():
					return
				case item, ok := <-in:
					if !ok {
						return
					}
					select {
					case <-ctx.Done():
						return
					case out <- item:
					}
				}
			}
		}()
		return out
	})
}
