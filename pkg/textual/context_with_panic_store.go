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
	"sync"
)

// PanicInfo holds details about a recovered panic.
//
// Value is the value passed to panic(...). It can be any Go value.
// Stack is a stack trace captured close to the panic site (typically via
// runtime/debug.Stack()).
//
// In the textual pipeline model, panics are treated as fatal programming faults
// (invariant violations, nil deref, out-of-bounds, etc.) and are captured
// out-of-band because pipeline stages run in goroutines and do not naturally
// return errors.
//
// The recommended contract is:
//
//   - Stage recovers and stores the panic (Async does this automatically).
//   - The pipeline supervisor checks the PanicStore at the boundary and decides
//     how to surface the fault (log, convert to error, re-panic, ...).
type PanicInfo struct {
	Value any
	Stack []byte
}

// PanicStore is a mutable holder that can be placed in a context via WithPanicStore.
//
// Concurrency contract:
//
//   - Store is write-once: the first call wins, subsequent calls are ignored.
//   - Load is safe to call concurrently with Store.
//   - Load returns a COPY of the stored stack trace so callers can safely keep
//     or modify it without affecting the store.
//
// Why a store in a context?
//
// In this package's pipeline model, processors/transcoders communicate through
// channels. There is no natural "return error" path from a goroutine.
// PanicStore provides a structured way to surface unexpected panics to the
// pipeline supervisor without crashing the entire process.
type PanicStore struct {
	once sync.Once
	mu   sync.Mutex
	info PanicInfo
	set  bool
}

// Store records the first panic information.
//
// If ps is nil, Store is a no-op.
//
// Store is write-once: only the first call wins (subsequent calls are ignored).
// The provided stack is defensively copied so callers can pass transient slices
// safely.
func (ps *PanicStore) Store(value any, stack []byte) {
	if ps == nil {
		return
	}
	ps.once.Do(func() {
		// Defensive copy so the stored stack is stable even if the caller
		// reuses/mutates the original slice.
		var stackCopy []byte
		if len(stack) > 0 {
			stackCopy = make([]byte, len(stack))
			copy(stackCopy, stack)
		}

		ps.mu.Lock()
		ps.info = PanicInfo{Value: value, Stack: stackCopy}
		ps.set = true
		ps.mu.Unlock()
	})
}

// Load retrieves the stored panic information, if present.
//
// If no panic was stored, ok is false.
// The returned PanicInfo is a snapshot; in particular, Stack is copied so that
// callers cannot mutate the store's internal data.
func (ps *PanicStore) Load() (PanicInfo, bool) {
	if ps == nil {
		return PanicInfo{}, false
	}

	ps.mu.Lock()
	info := ps.info
	ok := ps.set
	ps.mu.Unlock()

	if !ok {
		return PanicInfo{}, false
	}

	// Return a copy of Stack to prevent external mutation of internal state.
	if len(info.Stack) > 0 {
		stackCopy := make([]byte, len(info.Stack))
		copy(stackCopy, info.Stack)
		info.Stack = stackCopy
	}

	return info, true
}

type panicStoreKey struct{}

// WithPanicStore returns a derived context that carries a PanicStore, plus the store.
//
// The returned context can be passed through a pipeline; stages (like Async) can
// retrieve the store via PanicStoreFromContext(ctx) and record recovered panics.
//
// Typical usage at the pipeline boundary:
//
//	base, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	ctx, ps := WithPanicStore(base)
//	out := someStage.Apply(ctx, in)
//
//	for v := range out {
//	    _ = v
//	}
//
//	if info, ok := ps.Load(); ok {
//	    // surface the fatal fault
//	}
//
// WithPanicStore never returns a nil context. If parent is nil, it falls back to
// context.Background().
func WithPanicStore(parent context.Context) (context.Context, *PanicStore) {
	if parent == nil {
		parent = context.Background()
	}
	ps := &PanicStore{}
	return context.WithValue(parent, panicStoreKey{}, ps), ps
}

// PanicStoreFromContext retrieves the PanicStore from a context, if present.
//
// It returns nil when:
//   - ctx is nil, or
//   - no PanicStore has been attached via WithPanicStore.
//
// This function is intentionally small and non-allocating so it can be used in
// hot paths (e.g. deferred panic recovery).
func PanicStoreFromContext(ctx context.Context) *PanicStore {
	if ctx == nil {
		return nil
	}
	ps, _ := ctx.Value(panicStoreKey{}).(*PanicStore)
	return ps
}
