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

	"github.com/benoit-pereira-da-silva/textual/pkg/carrier"
)

// Transcoder is a chainable processing stage that converts a stream of carriers
// from S1 to S2.
//
// In the textual pipeline model:
//
//   - Processor[S]    : S  -> S
//   - Transcoder[S1,S2]: S1 -> S2
//
// A Transcoder reads values of type S1 from an input channel and produces values
// of type S2 on its output channel.
//
// Implementations are expected to:
//
//   - Read zero or more values from the input channel.
//   - Produce zero or more values on the returned channel.
//   - Respect ctx.Done() and stop promptly when the context is canceled.
//   - Close the returned channel when processing is complete or when ctx is done.
//   - Never close the input channel; the upstream stage is responsible for closing it.
//
// The returned channel must be non-nil. Callers are expected to consume from
// the returned channel until it is closed.
type Transcoder[S1 carrier.Carrier[S1], S2 carrier.Carrier[S2]] interface {
	// Apply starts the transcoding stage.
	//
	// The call should return quickly, typically after starting any necessary
	// goroutines. Implementations should monitor ctx.Done() and abort processing
	// when the context is canceled.
	Apply(ctx context.Context, in <-chan S1) <-chan S2
}

// TranscoderFunc is a function adapter that implements Transcoder.
//
// It allows plain functions to be used as Transcoder values:
//
//	t := TranscoderFunc[carrier.String, carrier.Parcel](func(ctx context.Context, in <-chan carrier.String) <-chan carrier.Parcel {
//		proto := carrier.Parcel{}
//		return Async(ctx, in, func(s carrier.String) carrier.Parcel {
//			// Convert String -> Parcel.
//			res := proto.FromUTF8String(carrier.UTF8String("P:" + s.Value)).WithIndex(s.GetIndex())
//
//			// Preserve per-item error as data.
//			if err := s.GetError(); err != nil {
//				res = res.WithError(err)
//			}
//			return res
//		})
//	})
//
// This can make it easier to construct lightweight transcoders inline.
type TranscoderFunc[S1 carrier.Carrier[S1], S2 carrier.Carrier[S2]] func(ctx context.Context, in <-chan S1) <-chan S2

// Apply calls f(ctx, in).
func (f TranscoderFunc[S1, S2]) Apply(ctx context.Context, in <-chan S1) <-chan S2 {
	return f(ctx, in)
}
