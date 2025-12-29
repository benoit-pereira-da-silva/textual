// Copyright 2026 Benoit Pereira da Silva
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package textual

import (
	"context"

	"github.com/benoit-pereira-da-silva/textual/pkg/carrier"
)

// Processor is a chainable processing stage for a textual pipeline.
//
// The pipeline is generic over a carrier type S (see Carrier). A Processor
// reads a stream of S values from an input channel and produces a stream of S
// values on its output channel.
//
// Implementations are expected to:
//
//   - Read zero or more values from the input channel.
//   - Produce zero or more processed values on the returned channel.
//   - Respect ctx.Done() and stop processing promptly when the context is
//     canceled.
//   - Close the returned channel when processing is complete or when the
//     context is canceled.
//   - Never close the input channel; the upstream stage is responsible for
//     closing it.
//
// The returned channel must be non-nil. Callers are expected to consume from
// the returned channel until it is closed.
type Processor[S carrier.Carrier[S]] interface {
	// Apply starts the processing stage.
	//
	// The call should return quickly, typically after starting any necessary
	// goroutines. Implementations should monitor ctx.Done() and abort processing
	// when the context is canceled.
	Apply(ctx context.Context, in <-chan S) <-chan S
}

// ProcessorFunc is a function adapter that implements Processor.
//
// It allows plain functions to be used as Processor values:
//
//	p := ProcessorFunc[carrier.String](func(ctx context.Context, in <-chan carrier.String) <-chan carrier.String {
//		return Async(ctx, in, func(s carrier.String) carrier.String {
//			s.Value = strings.ToUpper(s.Value)
//			return s
//		})
//	})
//
// This can make it easier to construct lightweight processors inline.
type ProcessorFunc[S carrier.Carrier[S]] func(ctx context.Context, in <-chan S) <-chan S

// Apply calls f(ctx, in).
func (f ProcessorFunc[S]) Apply(ctx context.Context, in <-chan S) <-chan S {
	return f(ctx, in)
}
