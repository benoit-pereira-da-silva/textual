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
	"strings"
	"testing"
	"time"
)

// procSuffixParcel returns a Processor[Parcel] that appends suffix to Text.
func procSuffix(suffix string) Processor[StringCarrier] {
	return ProcessorFunc[StringCarrier](func(ctx context.Context, in <-chan StringCarrier) <-chan StringCarrier {
		return Async(ctx, in, func(ctx context.Context, t StringCarrier) StringCarrier {
			proto := *new(StringCarrier)
			updated := proto.FromUTF8String(t.UTF8String() + suffix).WithIndex(t.GetIndex())
			if err := t.GetError(); err != nil {
				updated = updated.WithError(err)
			}
			return updated
		})
	})
}

func TestChain_SequentialAndIgnoresNil(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	chain := NewChain[StringCarrier](
		procSuffix("A"),
		nil, // should be ignored
		procSuffix("B"),
	)

	in := make(chan StringCarrier, 1)
	outCh := chain.Apply(ctx, in)

	in <- StringCarrier{Value: "X", Index: 42}
	close(in)

	items, err := collectWithContext(ctx, outCh)
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected output count: got %d want %d", len(items), 1)
	}

	if got, want := items[0].Value, "XAB"; got != want {
		t.Fatalf("unexpected chain output: got %q want %q", got, want)
	}
	if got, want := items[0].Index, 42; got != want {
		t.Fatalf("unexpected index: got %d want %d", got, want)
	}
}

func TestChain_NoProcessorsReturnsInputChannel(t *testing.T) {
	chain := NewChain[StringCarrier]()

	in := make(chan StringCarrier)
	var inR <-chan StringCarrier = in

	out := chain.Apply(context.Background(), inR)
	if out != inR {
		t.Fatalf("expected Apply to return the input channel when no processors are configured")
	}
	close(in)
}

func TestRouter_PassThroughWhenNoRoutes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	router := NewRouter[StringCarrier](RoutingStrategyBroadcast /* no processors */)

	in := make(chan StringCarrier, 2)
	outCh := router.Apply(ctx, in)

	in <- StringCarrier{Value: "one", Index: 0}
	in <- StringCarrier{Value: "two", Index: 1}
	close(in)

	items, err := collectWithContext(ctx, outCh)
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}
	sortByIndex(items)

	if len(items) != 2 {
		t.Fatalf("unexpected output count: got %d want %d", len(items), 2)
	}
	if items[0].Value != "one" || items[0].Index != 0 {
		t.Fatalf("unexpected item[0]: %#v", items[0])
	}
	if items[1].Value != "two" || items[1].Index != 1 {
		t.Fatalf("unexpected item[1]: %#v", items[1])
	}
}

func TestRouter_FirstMatchAndUnmatchedPassThrough(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	router := NewRouter[StringCarrier](RoutingStrategyFirstMatch)

	predA := func(_ context.Context, s StringCarrier) bool {
		return strings.HasPrefix(s.Value, "A")
	}

	router.AddRoute(predA, procSuffix("|r1"))
	router.AddRoute(predA, procSuffix("|r2")) // should NOT receive "A..." in FirstMatch

	in := make(chan StringCarrier, 2)
	outCh := router.Apply(ctx, in)

	in <- StringCarrier{Value: "A", Index: 0}
	in <- StringCarrier{Value: "B", Index: 1} // unmatched => passthrough
	close(in)

	items, err := collectWithContext(ctx, outCh)
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}
	sortByIndex(items)

	if len(items) != 2 {
		t.Fatalf("unexpected output count: got %d want %d items=%#v", len(items), 2, items)
	}

	if got, want := items[0].Value, "A|r1"; got != want {
		t.Fatalf("unexpected first-match result for A: got %q want %q", got, want)
	}
	if got, want := items[1].Value, "B"; got != want {
		t.Fatalf("unexpected passthrough result for B: got %q want %q", got, want)
	}
}

func TestRouter_BroadcastToAllEligibleRoutes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	router := NewRouter[StringCarrier](RoutingStrategyBroadcast)

	predX := func(_ context.Context, s StringCarrier) bool {
		return strings.Contains(s.Value, "x")
	}
	router.AddRoute(predX, procSuffix("|a"))
	router.AddRoute(predX, procSuffix("|b"))

	in := make(chan StringCarrier, 1)
	outCh := router.Apply(ctx, in)

	in <- StringCarrier{Value: "x", Index: 0}
	close(in)

	items, err := collectWithContext(ctx, outCh)
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("unexpected output count: got %d want %d items=%#v", len(items), 2, items)
	}

	got := map[string]bool{
		items[0].Value: true,
		items[1].Value: true,
	}
	if !got["x|a"] || !got["x|b"] {
		t.Fatalf("unexpected broadcast outputs: got=%v", got)
	}
}

func TestRouter_RoundRobinDistributesAcrossRoutes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	p1 := procSuffix("|p1")
	p2 := procSuffix("|p2")
	router := NewRouter[StringCarrier](RoutingStrategyRoundRobin, p1, p2)

	in := make(chan StringCarrier, 4)
	outCh := router.Apply(ctx, in)

	in <- StringCarrier{Value: "i0", Index: 0}
	in <- StringCarrier{Value: "i1", Index: 1}
	in <- StringCarrier{Value: "i2", Index: 2}
	in <- StringCarrier{Value: "i3", Index: 3}
	close(in)

	items, err := collectWithContext(ctx, outCh)
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}
	sortByIndex(items)

	if len(items) != 4 {
		t.Fatalf("unexpected output count: got %d want %d items=%#v", len(items), 4, items)
	}

	// Round-robin starts at route 0.
	want := []string{"i0|p1", "i1|p2", "i2|p1", "i3|p2"}
	for i := range want {
		if got := items[i].Value; got != want[i] {
			t.Fatalf("unexpected round-robin output at index %d: got %q want %q", i, got, want[i])
		}
	}
}
