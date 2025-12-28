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
	"bufio"
	"context"
	"io"
	"time"

	"github.com/benoit-pereira-da-silva/textual/pkg/carrier"
)

// IOReaderTranscoder connects an io.Reader to a Transcoder by scanning the input
// stream into tokens.
//
// Tokenization is controlled by a bufio.SplitFunc (default: bufio.ScanLines).
// Each token is converted into the carrier type S1 via:
//
//	prototype.FromUTF8String(token).WithIndex(i)
//
// where prototype is the zero value of S1 and i is the token sequence number.
//
// Important: the scanner yields bytes as-is. IOReaderTranscoder assumes those
// bytes represent UTF-8 text. If your source encoding is not UTF‑8, decode the
// reader first (for example with NewUTF8Reader) before plugging it here.
//
// Usage pattern:
//
//	t := NewIOReaderTranscoder(myTranscoder, reader)
//	t.SetContext(ctx)      // optional, must be called before Start / StartWithTimeout
//	t.SetSplitFunc(...)    // optional, must be called before Start / StartWithTimeout
//	out := t.Start()       // or t.StartWithTimeout(...)
//	for item := range out { /* consume results of type S2 */ }
//
// Start / StartWithTimeout spawn a goroutine that scans the input and feeds the
// transcoder's input channel. Stop cancels the context, which should cause the
// transcoder and the scanner goroutine to exit promptly.
//
// The generic type parameters S1 and S2 are the carriers flowing through the
// transcoding stage (see Carrier). T is the concrete transcoder type.
//
// Note: methods such as FromUTF8String are typically called on the zero value
// of S1, so implementations must not depend on receiver state.
type IOReaderTranscoder[S1 carrier.Carrier[S1], S2 carrier.Carrier[S2], T Transcoder[S1, S2]] struct {
	reader     io.Reader
	splitFunc  bufio.SplitFunc // splitFunc defines the bufio.SplitFunc used to tokenize the input from the io.Reader.
	transcoder T

	// ctx and cancel control the lifetime of the scanning / transcoding loop.
	// When ctx is nil, Start / StartWithTimeout will create a background
	// context. cancel can be nil until a cancellable context is created.
	ctx    context.Context
	cancel context.CancelFunc
}

// NewIOReaderTranscoder constructs a new IOReaderTranscoder using the provided
// transcoder and reader. By default, it uses bufio.ScanLines as a split function
// and a background context created on the first Start / StartWithTimeout.
func NewIOReaderTranscoder[S1 carrier.Carrier[S1], S2 carrier.Carrier[S2], T Transcoder[S1, S2]](transcoder T, reader io.Reader) *IOReaderTranscoder[S1, S2, T] {
	return &IOReaderTranscoder[S1, S2, T]{
		splitFunc:  bufio.ScanLines,
		reader:     reader,
		transcoder: transcoder,
	}
}

// SetContext sets the base context used by Start / StartWithTimeout.
//
// It must be called before Start / StartWithTimeout. The provided context is
// wrapped in a cancellable child so that Stop can terminate the transcoding
// loop even if the parent context is still alive.
func (t *IOReaderTranscoder[S1, S2, T]) SetContext(ctx context.Context) {
	if ctx == nil {
		// Avoid keeping a nil context internally; always fall back to Background.
		ctx = context.Background()
	}
	t.ctx, t.cancel = context.WithCancel(ctx)
}

// SetSplitFunc customizes the tokenization strategy.
//
// It must be called before Start / StartWithTimeout. If left unset, bufio.ScanLines
// is used, which yields a token per line (without the trailing newline).
func (t *IOReaderTranscoder[S1, S2, T]) SetSplitFunc(splitFunc bufio.SplitFunc) {
	t.splitFunc = splitFunc
}

// ensureContext initializes ctx / cancel if needed.
//
// When a context has been injected via SetContext, it is reused. If ctx is set
// but cancel is nil (for instance, after manual field initialization), a
// cancellable child context is derived so that Stop can be used safely.
func (t *IOReaderTranscoder[S1, S2, T]) ensureContext() {
	switch {
	case t.ctx == nil && t.cancel == nil:
		t.ctx, t.cancel = context.WithCancel(context.Background())
	case t.ctx != nil && t.cancel == nil:
		t.ctx, t.cancel = context.WithCancel(t.ctx)
	}
}

// Start reads from t.reader using a bufio.Scanner, splits according to
// splitFunc, converts each scanned token into an S1, and sends it into the
// underlying transcoder.
//
// Scanning stops as soon as:
//   - scanner.Scan() returns false (EOF or error), or
//   - the context is canceled or its deadline is exceeded.
//
// The underlying transcoder is expected to respect ctx and stop when it is done
// or when ctx is canceled.
func (t *IOReaderTranscoder[S1, S2, T]) Start() <-chan S2 {
	t.ensureContext()

	scanner := bufio.NewScanner(t.reader)
	if t.splitFunc != nil {
		scanner.Split(t.splitFunc)
	}

	// Channel feeding the underlying transcoder.
	in := make(chan S1)

	// Start the transcoder on the stream of S1 values.
	out := t.transcoder.Apply(t.ctx, in)

	// Goroutine responsible for scanning and feeding the input channel.
	go func() {
		prototype := *new(S1)
		defer close(in)

		counter := 0
		for {
			// Check for cancellation before attempting to scan.
			select {
			case <-t.ctx.Done():
				return
			default:
				// Continue to scanning.
			}

			// Perform one scan step.
			if !scanner.Scan() {
				// scanner.Scan() returned false: EOF or error.
				// scanner.Err() can be inspected here if a dedicated
				// error-reporting mechanism is added in the future.
				return
			}

			text := scanner.Text()
			item := prototype.FromUTF8String(text).WithIndex(counter)
			counter++

			// Send the value to the transcoder, remaining cancellable.
			select {
			case <-t.ctx.Done():
				// Context canceled while we were trying to send.
				return
			case in <- item:
				// Successfully sent to transcoder.
			}
		}
	}()

	return out
}

// StartWithTimeout is like Start but automatically cancels the context when
// the provided timeout elapses.
//
// If timeout <= 0, it simply delegates to Start without adding a timeout.
func (t *IOReaderTranscoder[S1, S2, T]) StartWithTimeout(timeout time.Duration) <-chan S2 {
	if timeout <= 0 {
		return t.Start()
	}
	// Use the existing context as a parent when available; otherwise fall back
	// to Background. This avoids the panic that context.WithTimeout would
	// trigger on a nil parent.
	parent := t.ctx
	if parent == nil {
		parent = context.Background()
	}
	t.ctx, t.cancel = context.WithTimeout(parent, timeout)
	return t.Start()
}

// Stop cancels the current transcoding context, if any.
//
// It is safe to call Stop even if Start / StartWithTimeout has not been
// invoked yet; in that case it is a no-op.
func (t *IOReaderTranscoder[S1, S2, T]) Stop() {
	if t.cancel != nil {
		t.cancel()
	}
}
