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

// StickLeft composes:
//
//	Transcoder[S1,S2] then Processor[S2]  => Transcoder[S1,S2]
//
// This is useful when you first *convert* a stream (S1 -> S2) and then apply
// additional same-type processing on S2.
//
// If the processor is nil, StickLeft returns the transcoder unchanged.
// If the transcoder is nil, StickLeft returns nil.
func StickLeft[S1 Carrier[S1], S2 Carrier[S2]](transcoder Transcoder[S1, S2], processor Processor[S2]) Transcoder[S1, S2] {
	if transcoder == nil {
		return nil
	}
	if processor == nil {
		return transcoder
	}
	return TranscoderFunc[S1, S2](func(ctx context.Context, in <-chan S1) <-chan S2 {
		if ctx == nil {
			ctx = context.Background()
		}
		mid := transcoder.Apply(ctx, in)
		return processor.Apply(ctx, mid)
	})
}

// StickRight composes:
//
//	Processor[S1] then Transcoder[S1,S2]  => Transcoder[S1,S2]
//
// This is useful when you want to do some same-type processing first (S1 -> S1)
// and only then convert to a different carrier type (S1 -> S2).
//
// If the processor is nil, StickRight returns the transcoder unchanged.
// If the transcoder is nil, StickRight returns nil.
func StickRight[S1 Carrier[S1], S2 Carrier[S2]](processor Processor[S1], transcoder Transcoder[S1, S2]) Transcoder[S1, S2] {
	if transcoder == nil {
		return nil
	}
	if processor == nil {
		return transcoder
	}
	return TranscoderFunc[S1, S2](func(ctx context.Context, in <-chan S1) <-chan S2 {
		if ctx == nil {
			ctx = context.Background()
		}
		mid := processor.Apply(ctx, in)
		return transcoder.Apply(ctx, mid)
	})
}
