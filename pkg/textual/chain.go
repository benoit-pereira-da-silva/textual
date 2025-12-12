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

// Usage sample:
// chain := NewChain(procA, procB, procC)
//
// ioProc = NewIOReaderProcessor[Processor](chain, reader)
// ioProc.SetContext(ctx)
// out := ioProc.Start()
// for r := range out {
//    // consume processed results
// }

// Chain is a processor that chains multiple Processor.
type Chain[S UTF8Stringer[S]] struct {
	processors []Processor[S]
}

func NewChain[S UTF8Stringer[S]](processors ...Processor[S]) *Chain[S] {
	return &Chain[S]{
		processors: processors,
	}
}

// Apply implements the Processor interface.
//
// It wires the configured processors into a linear pipeline, feeding the
// incoming channel through each stage in sequence. The returned channel is
// the output of the last processor. If no processors have been added, Apply
// simply returns the input channel unchanged.
//
// The same context is passed to every underlying processor; they are expected
// to monitor ctx.Done() and stop when the context is canceled.
func (c *Chain[S]) Apply(ctx context.Context, in <-chan S) <-chan S {
	out := in
	for _, p := range c.processors {
		if p == nil {
			continue
		}
		out = p.Apply(ctx, out)
	}
	return out
}
