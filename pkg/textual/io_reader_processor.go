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
)

// IOReaderProcessor connects an io.Reader to a Processor by scanning the input
// stream into tokens.
//
// Tokenization is controlled by a bufio.SplitFunc (default: bufio.ScanLines).
// Each token is converted into the carrier type S via:
//
//	prototype.FromUTF8String(token).WithIndex(i)
//
// where prototype is the zero value of S and i is the token sequence number.
//
// Important: the scanner yields bytes as-is. IOReaderProcessor assumes those
// bytes represent UTF-8 text. If your source encoding is not UTF‑8, decode the
// reader first (for example with NewUTF8Reader) before plugging it here.
//
// Usage pattern:
//
//	p := NewIOReaderProcessor(myProcessor, reader)
//	p.SetContext(ctx)      // optional, must be called before Start / StartWithTimeout
//	p.SetSplitFunc(...)    // optional, must be called before Start / StartWithTimeout
//	out := p.Start()       // or p.StartWithTimeout(...)
//	for item := range out { /* consume results */ }
//
// Start / StartWithTimeout spawn a goroutine that scans the input and feeds the
// processor's input channel. Stop cancels the context, which should cause the
// processor and the scanner goroutine to exit promptly.
//
// The generic type parameter S is the carrier flowing through the pipeline (see
// Carrier). P is the concrete processor type.
//
// Note: methods such as FromUTF8String are typically called on the zero value
// of S, so implementations must not depend on receiver state.
type IOReaderProcessor[S Carrier[S], P Processor[S]] struct {
	reader    io.Reader
	splitFunc bufio.SplitFunc // splitFunc defines the bufio.SplitFunc used to tokenize the input from the io.Reader.
	processor P

	// ctx and cancel control the lifetime of the scanning / processing loop.
	// When ctx is nil, Start / StartWithTimeout will create a background
	// context. cancel can be nil until a cancellable context is created.
	ctx    context.Context
	cancel context.CancelFunc
}

// NewIOReaderProcessor constructs a new IOReaderProcessor using the provided
// processor and reader. By default, it uses bufio.ScanLines as a split function
// and a background context created on the first Start / StartWithTimeout.
func NewIOReaderProcessor[S Carrier[S], P Processor[S]](processor P, reader io.Reader) *IOReaderProcessor[S, P] {
	return &IOReaderProcessor[S, P]{
		splitFunc: ScanLines,
		reader:    reader,
		processor: processor,
	}
}

// SetContext sets the base context used by Start / StartWithTimeout.
//
// It must be called before Start / StartWithTimeout. The provided context is
// wrapped in a cancellable child so that Stop can terminate the processing
// loop even if the parent context is still alive.
func (p *IOReaderProcessor[S, P]) SetContext(ctx context.Context) {
	if ctx == nil {
		// Avoid keeping a nil context internally; always fall back to Background.
		ctx = context.Background()
	}
	p.ctx, p.cancel = context.WithCancel(ctx)
}

// SetSplitFunc customizes the tokenization strategy.
//
// It must be called before Start / StartWithTimeout. If left unset, bufio.ScanLines
// is used, which yields a token per line (without the trailing newline).
func (p *IOReaderProcessor[S, P]) SetSplitFunc(splitFunc bufio.SplitFunc) {
	p.splitFunc = splitFunc
}

// ensureContext initializes ctx / cancel if needed.
//
// When a context has been injected via SetContext, it is reused. If ctx is set
// but cancel is nil (for instance, after manual field initialization), a
// cancellable child context is derived so that Stop can be used safely.
func (p *IOReaderProcessor[S, P]) ensureContext() {
	switch {
	case p.ctx == nil && p.cancel == nil:
		p.ctx, p.cancel = context.WithCancel(context.Background())
	case p.ctx != nil && p.cancel == nil:
		p.ctx, p.cancel = context.WithCancel(p.ctx)
	}
}

// Start reads from p.reader using a bufio.Scanner, splits according to
// splitFunc, converts each scanned token into an S, and sends it into the
// underlying processor.
//
// Scanning stops as soon as:
//   - scanner.Scan() returns false (EOF or error), or
//   - the context is canceled or its deadline is exceeded.
//
// The underlying processor is expected to respect ctx and stop when it is done
// or when ctx is canceled.
func (p *IOReaderProcessor[S, P]) Start() <-chan S {
	p.ensureContext()

	scanner := bufio.NewScanner(p.reader)
	if p.splitFunc != nil {
		scanner.Split(p.splitFunc)
	}

	// Channel feeding the underlying processor.
	in := make(chan S)

	// Start the processor on the stream of S values.
	out := p.processor.Apply(p.ctx, in)

	// Goroutine responsible for scanning and feeding the input channel.
	go func() {
		prototype := *new(S)
		defer close(in)

		counter := 0
		for {
			// Check for cancellation before attempting to scan.
			select {
			case <-p.ctx.Done():
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

			// Send the value to the processor, remaining cancellable.
			select {
			case <-p.ctx.Done():
				// Context canceled while we were trying to send.
				return
			case in <- item:
				// Successfully sent to processor.
			}
		}
	}()
	return out
}

// StartWithTimeout is like Start but automatically cancels the context when
// the provided timeout elapses.
//
// If timeout <= 0, it simply delegates to Start without adding a timeout.
func (p *IOReaderProcessor[S, P]) StartWithTimeout(timeout time.Duration) <-chan S {
	if timeout <= 0 {
		return p.Start()
	}
	// Use the existing context as a parent when available; otherwise fall back
	// to Background. This avoids the panic that context.WithTimeout would
	// trigger on a nil parent.
	parent := p.ctx
	if parent == nil {
		parent = context.Background()
	}
	p.ctx, p.cancel = context.WithTimeout(parent, timeout)
	return p.Start()
}

// Stop cancels the current processing context, if any.
//
// It is safe to call Stop even if Start / StartWithTimeout has not been
// invoked yet; in that case it is a no-op.
func (p *IOReaderProcessor[S, P]) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}
