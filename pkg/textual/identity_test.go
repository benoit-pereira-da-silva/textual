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

func TestIdentityProcessor_PassThrough(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	p := IdentityProcessor[carrier.String]{}

	in := make(chan carrier.String, 2)
	outCh := p.Apply(ctx, in)

	in <- carrier.String{Value: "one", Index: 1}
	in <- carrier.String{Value: "zero", Index: 0}
	close(in)

	items, err := collectWithContext(ctx, outCh)
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}
	sortByIndex(items)

	if len(items) != 2 {
		t.Fatalf("unexpected output count: got %d want %d", len(items), 2)
	}
	if got, want := items[0].Value, "zero"; got != want {
		t.Fatalf("unexpected item[0] value: got %q want %q", got, want)
	}
	if got, want := items[0].Index, 0; got != want {
		t.Fatalf("unexpected item[0] index: got %d want %d", got, want)
	}
	if got, want := items[1].Value, "one"; got != want {
		t.Fatalf("unexpected item[1] value: got %q want %q", got, want)
	}
	if got, want := items[1].Index, 1; got != want {
		t.Fatalf("unexpected item[1] index: got %d want %d", got, want)
	}
}

func TestIdentityTranscoder_PassThrough(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tid := IdentityTranscoder[carrier.String]{}

	in := make(chan carrier.String, 1)
	outCh := tid.Apply(ctx, in)

	in <- carrier.String{Value: "x", Index: 42}
	close(in)

	items, err := collectWithContext(ctx, outCh)
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected output count: got %d want %d", len(items), 1)
	}
	if got, want := items[0].Value, "x"; got != want {
		t.Fatalf("unexpected value: got %q want %q", got, want)
	}
	if got, want := items[0].Index, 42; got != want {
		t.Fatalf("unexpected index: got %d want %d", got, want)
	}
}
