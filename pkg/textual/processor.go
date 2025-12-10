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

import "context"

// Processor is a chainable building block for a textual pipeline.
//
// Implementations are expected to:
//
//   - Read zero or more Result values from the input channel.
//   - Produce zero or more processed Result values on the returned channel.
//   - Respect ctx.Done() and stop processing promptly when the context is
//     canceled.
//   - Close the returned channel when processing is complete or when the
//     context is canceled.
//   - Never close the input channel; the upstream stage is responsible for
//     closing it.
//
// The returned channel must be non-nil. Callers are expected to consume
// from the returned channel until it is closed.
type Processor interface {
	// Apply starts the processing stage.
	//
	// The call should return quickly, typically after starting any
	// necessary goroutines. Implementations should monitor ctx.Done()
	// and abort processing when the context is canceled.
	Apply(ctx context.Context, in <-chan Result) <-chan Result
}

// ProcessorFunc is a function adapter that implements Processor.
//
// It allows plain functions to be used as Processor values:
//
//	p := ProcessorFunc(func(ctx context.Context, in <-chan Result) <-chan Result {
//	    out := make(chan Result)
//	    go func() {
//	        defer close(out)
//	        for {
//	            select {
//	            case <-ctx.Done():
//	                return
//	            case r, ok := <-in:
//	                if !ok {
//	                    return
//	                }
//	                // Process r and send to out as needed.
//	                out <- r
//	            }
//	        }
//	    }()
//	    return out
//	})
//
// This can make it easier to construct lightweight processors inline.
type ProcessorFunc func(ctx context.Context, in <-chan Result) <-chan Result

// Apply calls f(ctx, in).
func (f ProcessorFunc) Apply(ctx context.Context, in <-chan Result) <-chan Result {
	return f(ctx, in)
}
