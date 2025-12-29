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
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestAsync_MapsValuesAndClosesOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	in := make(chan int, 3)
	in <- 1
	in <- 2
	in <- 3
	close(in)

	out := Async(ctx, in, func(v int) int { return v * 2 })

	items, err := collectWithContext(ctx, out)
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}

	want := []int{2, 4, 6}
	if !reflect.DeepEqual(items, want) {
		t.Fatalf("unexpected output: got %#v want %#v", items, want)
	}
}

func TestAsync_StopsOnContextCancellation(t *testing.T) {
	// Use a cancellable context. Cancellation is the only way for Async to stop
	// here because we never close and never send on `in`.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	in := make(chan int)
	out := Async(ctx, in, func(v int) int { return v })

	// Interrupt the stage.
	cancel()

	// Use a separate wait context so the collection doesn't abort just because
	// ctx has been canceled. We want to assert that Async closes its output.
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()

	items, err := collectWithContext(waitCtx, out)
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no output values, got %#v", items)
	}
}

func TestAsync_RecoversPanicAndStoresInContextPanicStore(t *testing.T) {
	baseCtx, baseCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer baseCancel()

	ctx, ps := WithPanicStore(baseCtx)

	in := make(chan int, 1)
	in <- 1
	close(in)

	out := Async(ctx, in, func(v int) int {
		panic("boom")
	})

	items, err := collectWithContext(baseCtx, out)
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no values after panic, got %#v", items)
	}

	info, ok := ps.Load()
	if !ok {
		t.Fatalf("expected PanicStore to contain panic info, got ok=false")
	}
	if got, want := info.Value, "boom"; got != want {
		t.Fatalf("unexpected panic value: got %#v want %#v", got, want)
	}
	if len(info.Stack) == 0 {
		t.Fatalf("expected non-empty stack trace")
	}
}

func ExampleAsync_withPanicStore() {
	ctx, ps := WithPanicStore(context.Background())

	in := make(chan int, 1)
	in <- 1
	close(in)

	out := Async(ctx, in, func(v int) int {
		panic("boom")
	})

	// Drain the output channel (it closes immediately because the worker panics).
	for range out {
	}

	info, ok := ps.Load()
	fmt.Println(ok, info.Value)
	// Output: true boom
}
