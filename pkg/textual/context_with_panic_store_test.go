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
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"
)

func TestPanicStore_NilReceiverIsNoOp(t *testing.T) {
	var ps *PanicStore
	ps.Store("boom", []byte("stack"))
	if _, ok := ps.Load(); ok {
		t.Fatalf("expected ok=false for nil PanicStore")
	}
}

func TestPanicStore_StoresOnlyFirst(t *testing.T) {
	ps := &PanicStore{}

	stack := []byte("stack1")
	ps.Store("first", stack)

	// Mutate the original slice to ensure Store performed a defensive copy.
	stack[0] = 'X'

	ps.Store("second", []byte("stack2"))

	info, ok := ps.Load()
	if !ok {
		t.Fatalf("expected ok=true, got ok=false")
	}
	if got, want := info.Value, "first"; got != want {
		t.Fatalf("unexpected stored value: got %#v want %#v", got, want)
	}
	if got, want := string(info.Stack), "stack1"; got != want {
		t.Fatalf("unexpected stored stack: got %q want %q", got, want)
	}
}

func TestWithPanicStore_AttachesStoreToContext(t *testing.T) {
	ctx, ps := WithPanicStore(context.Background())
	if ctx == nil {
		t.Fatalf("expected non-nil context")
	}
	if ps == nil {
		t.Fatalf("expected non-nil PanicStore")
	}
	if got := PanicStoreFromContext(ctx); got != ps {
		t.Fatalf("context did not return the attached PanicStore")
	}
}

func TestWithPanicStore_NilParentUsesBackground(t *testing.T) {
	ctx, ps := WithPanicStore(nil)
	if ctx == nil {
		t.Fatalf("expected non-nil context")
	}
	if ps == nil {
		t.Fatalf("expected non-nil PanicStore")
	}
	if got := PanicStoreFromContext(ctx); got != ps {
		t.Fatalf("context did not return the attached PanicStore")
	}
}

func TestPanicStore_ConcurrentStore_StoresExactlyOne(t *testing.T) {
	ps := &PanicStore{}
	var wg sync.WaitGroup

	const n = 64
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			ps.Store(i, []byte("stack"))
		}()
	}
	wg.Wait()

	info, ok := ps.Load()
	if !ok {
		t.Fatalf("expected ok=true, got ok=false")
	}

	v, ok := info.Value.(int)
	if !ok {
		t.Fatalf("expected stored Value to be an int, got %T", info.Value)
	}
	if v < 0 || v >= n {
		t.Fatalf("stored Value out of range: got %d, want 0..%d", v, n-1)
	}
	if !bytes.Equal(info.Stack, []byte("stack")) {
		t.Fatalf("unexpected stack: got %q want %q", string(info.Stack), "stack")
	}
}

func ExampleWithPanicStore() {
	ctx, ps := WithPanicStore(context.Background())
	_ = ctx // ctx is meant to be passed to pipeline stages.

	ps.Store("boom", []byte("stack"))
	info, ok := ps.Load()
	fmt.Println(ok, info.Value)
	// Output: true boom
}
