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

// IOReaderProcessor calls a processor progressively on slices coming from an io.Reader.
// The slices are split by a bufio.SplitFunc (by default a bufio.ScanLines)
// Must be started by Start or StartWithTimeout
// Can be stopped by Stop or by a context cancellation.
// SetContext enables to define a context (must be called before Start)
// SetSplitFunc enables to provide a specific split function (must be called before Start)
type IOReaderProcessor[P Processor] struct {
	reader    io.Reader
	splitFunc bufio.SplitFunc // splitFunc defines the bufio.SplitFunc used to tokenize the input from the io.Reader.
	processor P
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewIOReaderProcessor uses an IOReaderProcessor.

func NewIOReaderProcessor[P Processor](processor P, reader io.Reader) *IOReaderProcessor[P] {
	return &IOReaderProcessor[P]{
		splitFunc: bufio.ScanLines,
		reader:    reader,
		processor: processor,
	}
}

func (p *IOReaderProcessor[P]) SetContext(ctx context.Context) {
	p.ctx, p.cancel = context.WithCancel(ctx)
}

func (p *IOReaderProcessor[P]) SetSplitFunc(splitFunc bufio.SplitFunc) {
	p.splitFunc = splitFunc
}

// Start reads from s.reader using a bufio.Scanner, splits, according to splitFunc,
// turns each token into a Result, and sends it into the processor's input
// channel. It returns the output channel produced by the underlying processor.
//
// Scanning stops as soon as:
//   - scanner.Scan() returns false (EOF or error), or
//   - ctx is canceled or its deadline is exceeded.
//
// The underlying processor is expected to respect ctx and stop when it is done
// or when ctx is canceled.
func (p *IOReaderProcessor[P]) Start() <-chan Result {
	if p.ctx == nil {
		p.ctx, p.cancel = context.WithCancel(context.Background())
	}
	scanner := bufio.NewScanner(p.reader)
	if p.splitFunc != nil {
		scanner.Split(p.splitFunc)
	}
	// Channel feeding the underlying processor.
	in := make(chan Result)
	// Start the processor on the stream of Results.
	out := p.processor.Apply(p.ctx, in)
	// Goroutine responsible for scanning and feeding the input channel.
	go func() {
		defer close(in)

		counter := 0
		for {
			// Check for cancellation before attempting to scan.
			select {
			case <-p.ctx.Done():
				// Context canceled or deadline exceeded: stop scanning.
				return
			default:
				// Fall through and do the actual scan.
			}

			// Perform one scan step.
			if !scanner.Scan() {
				// scanner.Scan() returned false: EOF or error.
				// scanner.Err() can be inspected by the caller via the reader
				// or by wrapping IOReaderProcessor if needed.
				return
			}

			line := scanner.Text()
			res := Input(UTF8String(line))
			res.Index = counter
			counter++

			// Send the Result to the processor, remaining cancellable.
			select {
			case <-p.ctx.Done():
				// Context canceled while we were trying to send.
				return
			case in <- res:
				// Successfully sent to processor.
			}
		}
	}()

	return out
}

// StartWithTimeout is like Run but automatically cancels the context when
// the provided timeout elapses.
//
// If timeout <= 0, it simply delegates to Run without adding a timeout.
func (p *IOReaderProcessor[P]) StartWithTimeout(timeout time.Duration) <-chan Result {
	if timeout <= 0 {
		return p.Start()
	}
	p.ctx, p.cancel = context.WithTimeout(p.ctx, timeout)
	return p.Start()
}

func (p *IOReaderProcessor[P]) Stop() {
	p.cancel()
}
