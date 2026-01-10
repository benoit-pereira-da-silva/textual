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

type Processors[S Carrier[S]] []Processor[S]

// NewChain creates a single ProcessorFunc by composing processors left-to-right.
//
// Given p1, p2, p3, the resulting processor behaves like:
//
//	out := p3.Apply(ctx, p2.Apply(ctx, p1.Apply(ctx, in)))
//
// Nil processors are ignored.
func NewChain[S Carrier[S]](processors ...Processor[S]) ProcessorFunc[S] {
	ps := Processors[S](processors)
	return ps.ProcessorFunc()
}

func (p Processors[C]) ProcessorFunc() ProcessorFunc[C] {
	return func(ctx context.Context, in <-chan C) <-chan C {
		return p.Apply(ctx, in)
	}
}

func (p Processors[C]) Apply(ctx context.Context, in <-chan C) <-chan C {
	ctx, ps := EnsurePanicStore(ctx)

	out := in
	for _, proc := range p {
		if proc == nil {
			continue
		}

		var ok bool
		out, ok = safeApplyProcessor(ctx, ps, proc, out)
		if !ok {
			// A stage panicked or violated the channel contract (nil output).
			// A closed channel has been substituted; stop composing further stages.
			break
		}
	}

	// Enforce the "returned channel must be non-nil" contract even for degenerate
	// cases (e.g. no processors and nil input).
	if out == nil {
		if ps != nil {
			ps.Store("textual: Processors.Apply produced a nil channel", debug.Stack())
		}
		out = closedChan[C]()
	}

	return out
}
