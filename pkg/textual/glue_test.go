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
	"testing"
	"time"

	"github.com/benoit-pereira-da-silva/textual/pkg/carrier"
)

func TestGlue_StickLeft_ComposesTranscoderThenProcessor(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// String -> Parcel transcoder.
	toParcel := TranscoderFunc[carrier.String, carrier.Parcel](func(ctx context.Context, in <-chan carrier.String) <-chan carrier.Parcel {
		out := make(chan carrier.Parcel)
		go func() {
			defer close(out)
			proto := carrier.Parcel{}
			for {
				select {
				case <-ctx.Done():
					return
				case s, ok := <-in:
					if !ok {
						return
					}
					p := proto.FromUTF8String(carrier.UTF8String("P:" + s.Value)).WithIndex(s.GetIndex())
					if err := s.GetError(); err != nil {
						p = p.WithError(err)
					}
					select {
					case <-ctx.Done():
						return
					case out <- p:
					}
				}
			}
		}()
		return out
	})

	// Parcel -> Parcel processor.
	appendSuffix := ProcessorFunc[carrier.Parcel](func(ctx context.Context, in <-chan carrier.Parcel) <-chan carrier.Parcel {
		out := make(chan carrier.Parcel)
		go func() {
			defer close(out)
			proto := carrier.Parcel{}
			for {
				select {
				case <-ctx.Done():
					return
				case p, ok := <-in:
					if !ok {
						return
					}
					updated := proto.FromUTF8String(carrier.UTF8String(string(p.Text) + "|S")).WithIndex(p.GetIndex())
					if err := p.GetError(); err != nil {
						updated = updated.WithError(err)
					}
					select {
					case <-ctx.Done():
						return
					case out <- updated:
					}
				}
			}
		}()
		return out
	})

	composed := StickLeft(toParcel, appendSuffix)

	in := make(chan carrier.String, 1)
	outCh := composed.Apply(ctx, in)

	in <- carrier.String{Value: "x", Index: 7}
	close(in)

	items, err := collectWithContext(ctx, outCh)
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected output count: got %d want %d", len(items), 1)
	}
	if got, want := items[0].UTF8String(), "P:x|S"; got != want {
		t.Fatalf("unexpected output: got %q want %q", got, want)
	}
	if got, want := items[0].GetIndex(), 7; got != want {
		t.Fatalf("unexpected index: got %d want %d", got, want)
	}
}

func TestGlue_StickRight_ComposesProcessorThenTranscoder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// String -> String processor.
	appendA := ProcessorFunc[carrier.String](func(ctx context.Context, in <-chan carrier.String) <-chan carrier.String {
		out := make(chan carrier.String)
		go func() {
			defer close(out)
			for {
				select {
				case <-ctx.Done():
					return
				case s, ok := <-in:
					if !ok {
						return
					}
					s.Value = s.Value + "A"
					select {
					case <-ctx.Done():
						return
					case out <- s:
					}
				}
			}
		}()
		return out
	})

	// String -> Parcel transcoder.
	toParcel := TranscoderFunc[carrier.String, carrier.Parcel](func(ctx context.Context, in <-chan carrier.String) <-chan carrier.Parcel {
		out := make(chan carrier.Parcel)
		go func() {
			defer close(out)
			proto := carrier.Parcel{}
			for {
				select {
				case <-ctx.Done():
					return
				case s, ok := <-in:
					if !ok {
						return
					}
					p := proto.FromUTF8String(carrier.UTF8String("P:" + s.Value)).WithIndex(s.GetIndex())
					if err := s.GetError(); err != nil {
						p = p.WithError(err)
					}
					select {
					case <-ctx.Done():
						return
					case out <- p:
					}
				}
			}
		}()
		return out
	})
	composed := StickRight(appendA, toParcel)

	in := make(chan carrier.String, 1)
	outCh := composed.Apply(ctx, in)

	in <- carrier.String{Value: "x", Index: 3}
	close(in)

	items, err := collectWithContext(ctx, outCh)
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected output count: got %d want %d", len(items), 1)
	}
	if got, want := items[0].UTF8String(), "P:xA"; got != want {
		t.Fatalf("unexpected output: got %q want %q", got, want)
	}
	if got, want := items[0].GetIndex(), 3; got != want {
		t.Fatalf("unexpected index: got %d want %d", got, want)
	}
}
