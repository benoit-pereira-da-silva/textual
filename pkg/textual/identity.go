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

// IdentityProcessor is a minimal Processor that forwards values unchanged.
//
// It is useful as:
//
//   - a default stage in configurable pipelines,
//   - a placeholder while assembling / testing pipelines,
//   - a no-op stage for conditional wiring (instead of branching on nil).
//
// IdentityProcessor respects ctx.Done() and stops promptly on cancellation.
type IdentityProcessor[S carrier.Carrier[S]] struct{}

// Apply implements Processor[S] by forwarding every input value unchanged.
func (p IdentityProcessor[S]) Apply(ctx context.Context, in <-chan S) <-chan S {
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
}

// IdentityTranscoder is a minimal Transcoder that forwards values unchanged.
//
// It implements Transcoder[S,S]. It is useful when a call site expects a
// Transcoder but you want a no-op (for example in tests or as a default).
//
// IdentityTranscoder respects ctx.Done() and stops promptly on cancellation.
type IdentityTranscoder[S carrier.Carrier[S]] struct{}

// Apply implements Transcoder[S,S] by forwarding every input value unchanged.
func (t IdentityTranscoder[S]) Apply(ctx context.Context, in <-chan S) <-chan S {
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
}
