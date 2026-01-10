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
type Transcoder[S1 Carrier[S1], S2 Carrier[S2]] interface {
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
//	t := TranscoderFunc[carrier.StringCarrier, carrier.Parcel](func(ctx context.Context, in <-chan carrier.StringCarrier) <-chan carrier.Parcel {
//		proto := carrier.Parcel{}
//		return Async(ctx, in, func(ctx context.Context, s carrier.StringCarrier) carrier.Parcel {
//			// Convert StringCarrier -> Parcel.
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
type TranscoderFunc[S1 Carrier[S1], S2 Carrier[S2]] func(ctx context.Context, in <-chan S1) <-chan S2

// NewTranscoderFunc adapts an item-level function f(ctx, item) -> item into a TranscoderFunc.
//
// The returned TranscoderFunc uses Async to apply f to each item in the input stream.
//
// It applies f(ctx, item) to every input item (1:1) using Async.
func NewTranscoderFunc[S1 Carrier[S1], S2 Carrier[S2]](f func(ctx context.Context, c S1) S2) TranscoderFunc[S1, S2] {
	return TranscoderFunc[S1, S2](func(ctx context.Context, in <-chan S1) <-chan S2 {
		return Async(ctx, in, func(ctx context.Context, s S1) S2 {
			return f(ctx, s)
		})
	})
}

// Apply calls f(ctx, in).
//
// For safety, Apply enforces the Transcoder contract that the returned channel is
// never nil. If f panics (including the case where f is nil), the panic is
// recovered, recorded into the PanicStore carried by ctx (ensured via
// EnsurePanicStore), and a closed channel is returned.
func (f TranscoderFunc[S1, S2]) Apply(ctx context.Context, in <-chan S1) (out <-chan S2) {
	ctx, ps := EnsurePanicStore(ctx)

	defer func() {
		if r := recover(); r != nil {
			if ps != nil {
				ps.Store(r, debug.Stack())
			}
			out = closedChan[S2]()
		}
	}()

	out = f(ctx, in)
	if out == nil {
		if ps != nil {
			ps.Store("textual: TranscoderFunc returned a nil channel", debug.Stack())
		}
		out = closedChan[S2]()
	}
	return out
}

// Prepend composes one or more Processor[S1] stages *before* this transcoder.
//
// Given p1, p2 and transcoder f, the resulting transcoder behaves like:
//
//	out := f.Apply(ctx, p2.Apply(ctx, p1.Apply(ctx, in)))
//
// Nil processors are ignored (via NewChain).
func (f TranscoderFunc[S1, S2]) Prepend(p ...Processor[S1]) Transcoder[S1, S2] {
	if len(p) == 0 {
		return f
	}
	chain := NewChain[S1](p...)
	return TranscoderFunc[S1, S2](func(ctx context.Context, in <-chan S1) <-chan S2 {
		return f.Apply(ctx, chain.Apply(ctx, in))
	})
}

// Append composes one or more Processor[S2] stages *after* this transcoder.
//
// Given transcoder f and processors p1, p2, the resulting transcoder behaves like:
//
//	out := p2.Apply(ctx, p1.Apply(ctx, f.Apply(ctx, in)))
//
// Nil processors are ignored (via NewChain).
func (f TranscoderFunc[S1, S2]) Append(p ...Processor[S2]) Transcoder[S1, S2] {
	if len(p) == 0 {
		return f
	}
	chain := NewChain[S2](p...)
	return TranscoderFunc[S1, S2](func(ctx context.Context, in <-chan S1) <-chan S2 {
		return chain.Apply(ctx, f.Apply(ctx, in))
	})
}
