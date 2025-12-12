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
	"testing"
)

// trackingReadCloser wraps an io.Reader and records whether Close was called.
type trackingReadCloser struct {
	*bytes.Reader
	closed bool
}

func (r *trackingReadCloser) Close() error {
	r.closed = true
	return nil
}

// trackingWriteCloser wraps a bytes.Buffer and records whether Close was called.
type trackingWriteCloser struct {
	bytes.Buffer
	closed bool
}

func (w *trackingWriteCloser) Close() error {
	w.closed = true
	return nil
}

// TestTransformationProcess_PassThrough verifies that a simple echo Processor
// receives a single input Parcel, produces a single output Parcel, and that
// Process encodes / writes that result and closes both reader and writer.
func TestTransformationProcess_PassThrough(t *testing.T) {
	// Original string, encoded as UTF‑8 for simplicity.
	const original = "Hello, café!\n"

	rc := &trackingReadCloser{Reader: bytes.NewReader([]byte(original))}
	wc := &trackingWriteCloser{}

	// Echo processor: forwards every incoming Parcel as‑is.
	echo := ProcessorFunc[Parcel](func(ctx context.Context, in <-chan Parcel) <-chan Parcel {
		out := make(chan Parcel)
		go func() {
			defer close(out)
			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						return
					}
					out <- res
				}
			}
		}()
		return out
	})

	tr := NewTransformation[Parcel, Processor[Parcel]](
		"echo",
		echo,
		Nature{Dialect: "plain", EncodingID: UTF8},
		Nature{Dialect: "plain", EncodingID: UTF8},
	)

	if err := tr.Process(context.Background(), rc, wc); err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	if !rc.closed {
		t.Errorf("expected reader to be closed by Process")
	}
	if !wc.closed {
		t.Errorf("expected writer to be closed by Process")
	}

	got := wc.String()
	if got != original {
		t.Fatalf("unexpected output:\n  got: %q\n want: %q", got, original)
	}
}

// TestTransformationProcess_MultipleResults ensures that Process drains the
// processor's output channel until it is closed, not just a single Parcel.
func TestTransformationProcess_MultipleResults(t *testing.T) {
	const original = "ABCDEFGHIJ"

	rc := &trackingReadCloser{Reader: bytes.NewReader([]byte(original))}
	wc := &trackingWriteCloser{}

	// This processor splits the input Text into two Results and emits both.
	splitting := ProcessorFunc[Parcel](func(ctx context.Context, in <-chan Parcel) <-chan Parcel {
		out := make(chan Parcel)
		go func() {
			instance := Parcel{}
			defer close(out)
			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						return
					}
					text := string(res.Text)
					mid := len(text) / 2
					left := instance.FromUTF8String(text[:mid])
					right := instance.FromUTF8String(text[mid:])
					// Emit both Results.
					select {
					case <-ctx.Done():
						return
					case out <- left:
					}
					select {
					case <-ctx.Done():
						return
					case out <- right:
					}
				}
			}
		}()
		return out
	})

	tr := NewTransformation[Parcel, Processor[Parcel]](
		"split",
		splitting,
		Nature{Dialect: "plain", EncodingID: UTF8},
		Nature{Dialect: "plain", EncodingID: UTF8},
	)

	if err := tr.Process(context.Background(), rc, wc); err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	got := wc.String()
	if got != original {
		t.Fatalf("unexpected output:\n  got: %q\n want: %q", got, original)
	}
}
