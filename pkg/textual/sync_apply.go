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

// SyncApply applies a Processor to a single input value and returns a single
// output value.
//
// The processor may emit:
//   - 0 values: SyncApply returns the input (pass-through).
//   - 1 value : SyncApply returns it.
//   - N>1     : SyncApply aggregates them using S.Aggregate.
//
// Context cancellation is respected while reading the output channel.
func SyncApply[S AggregatableCarrier[S], P Processor[S]](ctx context.Context, p P, in S) S {
	if ctx == nil {
		ctx = context.Background()
	}
	inCh := make(chan S, 1)
	inCh <- in
	close(inCh)
	outCh := p.Apply(ctx, inCh)
	results := make([]S, 0, 1)
	for res := range outCh {
		results = append(results, res)
	}
	if len(results) == 0 {
		// Pass-through in the degenerate case.
		return in
	}
	if len(results) == 1 {
		return results[0]
	}
	// Aggregate if we received multiple results.
	proto := *new(S)
	return proto.Aggregate(results)
}
