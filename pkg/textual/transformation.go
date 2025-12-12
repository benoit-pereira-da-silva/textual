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
	"io"
)

// Transformation binds a Processor with information about the source and
// destination textual "nature" (dialect + encoding).
//
// The processing stack is generic over a carrier type S (see UTF8Stringer).
// S represents what flows through the pipeline (for example String or Result).
//
// The generic type parameter P is the concrete Processor implementation that
// transforms values of type S.
type Transformation[S UTF8Stringer[S], P Processor[S]] struct {
	// Name is an arbitrary identifier for diagnostic / logging purposes.
	Name string `json:"name"`

	// From describes the expected input dialect and encoding.
	From Nature `json:"from"`

	// To describes the output dialect and encoding produced by the processor.
	To Nature `json:"to"`

	// Processor is the processing stage that turns an input value into zero or
	// more output values.
	Processor P `json:"processor"`
}

// Nature captures the dialect / encoding pair used by a Transformation.
//
// Dialect is intentionally free-form (e.g. "plain", "ipa", "pseudo-phonetic")
// while EncodingID is constrained to the set supported in encoding.go.
type Nature struct {
	Dialect    string     `json:"dialect"`    // the textual dialect.
	EncodingID EncodingID `json:"encodingID"` // the string encoding.
}

// NewTransformation constructs a new Transformation instance binding a
// processor with its input / output natures.
func NewTransformation[S UTF8Stringer[S], P Processor[S]](name string, p P, from Nature, to Nature) *Transformation[S, P] {
	return &Transformation[S, P]{
		Name:      name,
		From:      from,
		To:        to,
		Processor: p,
	}
}

// Description returns a human-readable description of the transformation,
// including both dialect and encoding, in the form:
//
//	FromDialect(FromEncoding) -> ToDialect(ToEncoding)
func (t Transformation[S, P]) Description() string {
	return fmt.Sprintf(
		"%s(%s)->%s(%s)",
		t.From.Dialect,
		t.From.EncodingID.EncodingName(),
		t.To.Dialect,
		t.To.EncodingID.EncodingName(),
	)
}

// Process runs the transformation synchronously.
//
// Behaviour:
//
//  1. The full content of r is decoded from t.From.EncodingID into a UTF-8
//     string (DecodeText).
//  2. A single S value is created from that text (FromUTF8String) and sent into
//     the underlying Processor.
//  3. The input channel is then closed to signal "no more input" to the
//     Processor.
//  4. All values produced on the processor's output channel are encoded to
//     t.To.EncodingID and written to w (EncodeResult).
//  5. When the processor closes its output channel, Process returns nil.
//
// Context & lifetime semantics:
//
//   - If ctx is nil, context.Background() is used.
//   - A derived cancellable context is created and passed to the Processor.
//   - On any encoding error, the derived context is canceled and the remaining
//     outputs are drained in a background goroutine to avoid blocking downstream
//     senders.
//   - r and w are always closed before Process returns (success or error), via
//     the ignoreErr helper.
//
// End-of-stream signalling:
//
//   - Process tells the Processor that no more input is coming by closing the
//     input channel after sending the single initial value.
//   - Process learns that no more output is coming when the Processor closes the
//     output channel it returned from Apply. This is the canonical Go pattern:
//     "closed channel == end of stream".
func (t Transformation[S, P]) Process(ctx context.Context, r io.ReadCloser, w io.WriteCloser) error {
	if ctx == nil {
		ctx = context.Background()
	}

	// Derived context used both by the processor and by this method to
	// coordinate cancellation on encoding errors.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Always close the underlying I/O objects; callers hand off ownership to
	// the Transformation for the duration of Process.
	defer ignoreErr(r.Close())
	defer ignoreErr(w.Close())

	// Step 1: decode the full input to UTF-8.
	original, err := t.DecodeText(r)
	if err != nil {
		return err
	}

	// Step 2: wire the processor.
	in := make(chan S)
	outCh := t.Processor.Apply(ctx, in)

	proto := *new(S)
	input := proto.FromUTF8String(original)

	// Step 3: send a single value and immediately close the input channel
	// to signal end-of-input to the processor.
	select {
	case <-ctx.Done():
		// The context was canceled before we could send anything.
		close(in)
		return ctx.Err()
	case in <- input:
		// Input successfully sent.
	}
	close(in)

	// Step 4: consume every output until the processor closes its output
	// channel. This is how we know "there are no more values to come".
	for {
		select {
		case <-ctx.Done():
			// Context cancellation: stop forwarding outputs but keep draining
			// the processor's output channel so that it does not block on
			// sends.
			go func(ch <-chan S) {
				for range ch {
				}
			}(outCh)
			return ctx.Err()
		case res, ok := <-outCh:
			if !ok {
				// Step 5: processor signalled completion by closing its output.
				return nil
			}
			if err := t.EncodeResult(res, w); err != nil {
				// Encoding error: cancel the processor and drain any remaining
				// output so that it can terminate without blocking.
				cancel()
				go func(ch <-chan S) {
					for range ch {
					}
				}(outCh)
				return err
			}
		}
	}
}

// DecodeText reads the entire stream from r and decodes it from the
// Transformation's source encoding into a UTF-8 string.
//
// DecodeText does NOT close r; Process is responsible for closing the
// ReadCloser that it receives.
func (t Transformation[S, P]) DecodeText(r io.ReadCloser) (UTF8String, error) {
	s, err := ReaderToUTF8(r, t.From.EncodingID)
	if err != nil {
		return "", err
	}
	return s, nil
}

// EncodeResult encodes a single output value into the Transformation's
// destination encoding and writes it to w.
//
// EncodeResult does NOT close w; Process owns the WriteCloser lifecycle.
// Multiple calls to EncodeResult append sequentially to w.
func (t Transformation[S, P]) EncodeResult(res S, w io.WriteCloser) error {
	// Render the value back into a plain UTF-8 string.
	text := res.UTF8String()
	return FromUTF8ToWriter(text, t.To.EncodingID, w)
}
