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
)

func TestGlue_StickLeft_ComposesTranscoderThenProcessor(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// StringCarrier -> Parcel transcoder.
	toParcel := TranscoderFunc[StringCarrier, Parcel](func(ctx context.Context, in <-chan StringCarrier) <-chan Parcel {
		return Async(ctx, in, func(ctx context.Context, t StringCarrier) Parcel {
			proto := Parcel{}
			updated := proto.FromUTF8String(UTF8String("P:" + t.Value)).WithIndex(t.GetIndex())
			if err := t.GetError(); err != nil {
				updated = updated.WithError(err)
			}
			return updated
		})
	})

	// Parcel -> Parcel processor.
	appendSuffix := ProcessorFunc[Parcel](func(ctx context.Context, in <-chan Parcel) <-chan Parcel {
		return Async(ctx, in, func(ctx context.Context, t Parcel) Parcel {
			proto := Parcel{}
			updated := proto.FromUTF8String(UTF8String(string(t.Text) + "|S")).WithIndex(t.GetIndex())
			if err := t.GetError(); err != nil {
				updated = updated.WithError(err)
			}
			return updated
		})
	})

	composed := StickLeft(toParcel, appendSuffix)

	in := make(chan StringCarrier, 1)
	outCh := composed.Apply(ctx, in)

	in <- StringCarrier{Value: "x", Index: 7}
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

	// StringCarrier -> StringCarrier processor.
	appendA := ProcessorFunc[StringCarrier](func(ctx context.Context, in <-chan StringCarrier) <-chan StringCarrier {
		return Async(ctx, in, func(ctx context.Context, t StringCarrier) StringCarrier {
			t.Value = t.Value + "A"
			return t
		})
	})

	// StringCarrier -> Parcel transcoder.
	toParcel := TranscoderFunc[StringCarrier, Parcel](func(ctx context.Context, in <-chan StringCarrier) <-chan Parcel {
		return Async(ctx, in, func(ctx context.Context, t StringCarrier) Parcel {
			proto := Parcel{}
			updated := proto.FromUTF8String(UTF8String("P:" + t.Value)).WithIndex(t.GetIndex())
			if err := t.GetError(); err != nil {
				updated = updated.WithError(err)
			}
			return updated
		})
	})

	composed := StickRight(appendA, toParcel)

	in := make(chan StringCarrier, 1)
	outCh := composed.Apply(ctx, in)

	in <- StringCarrier{Value: "x", Index: 3}
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
