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

	"github.com/benoit-pereira-da-silva/textual/pkg/carrier"
)

func TestIOReaderTranscoder_Start_ScanLinesAndIndexes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	input := "hello\nworld\n"
	reader := strings.NewReader(input)

	// String -> Parcel transcoder that prefixes and preserves index.
	tprefix := TranscoderFunc[carrier.String, carrier.Parcel](func(ctx context.Context, in <-chan carrier.String) <-chan carrier.Parcel {
		out := make(chan carrier.Parcel)
		go func() {
			defer close(out)
			proto := carrier.Parcel{}
			for {
				select {
				case <-ctx.Done():
					return
				case s, ok := <-in:
					if !ok {
						return
					}
					res := proto.FromUTF8String(carrier.UTF8String("P:" + s.Value)).WithIndex(s.GetIndex())
					if err := s.GetError(); err != nil {
						res = res.WithError(err)
					}
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
				}
			}
		}()
		return out
	})

	ioT := NewIOReaderTranscoder[carrier.String](tprefix, reader)
	ioT.SetContext(ctx)

	outCh := ioT.Start()
	items, err := collectWithContext(ctx, outCh)
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}
	sortByIndex(items)

	if len(items) != 2 {
		t.Fatalf("unexpected output count: got %d want %d items=%#v", len(items), 2, items)
	}

	if got, want := items[0].UTF8String(), "P:hello"; got != want {
		t.Fatalf("unexpected item[0]: got %q want %q", got, want)
	}
	if got, want := items[0].GetIndex(), 0; got != want {
		t.Fatalf("unexpected item[0] index: got %d want %d", got, want)
	}

	if got, want := items[1].UTF8String(), "P:world"; got != want {
		t.Fatalf("unexpected item[1]: got %q want %q", got, want)
	}
	if got, want := items[1].GetIndex(), 1; got != want {
		t.Fatalf("unexpected item[1] index: got %d want %d", got, want)
	}
}

func TestIOReaderTranscoder_CustomSplit_ScanJSON(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	input := " \n,\t{\"a\":1}  [1,2,{\"b\":\"x\"}]  {\"c\":\"{[\\\"}]\"}\n"
	reader := strings.NewReader(input)

	// String -> JSON transcoder that preserves index.
	toJSON := TranscoderFunc[carrier.String, carrier.JSON](func(ctx context.Context, in <-chan carrier.String) <-chan carrier.JSON {
		out := make(chan carrier.JSON)
		go func() {
			defer close(out)
			proto := carrier.JSON{}
			for {
				select {
				case <-ctx.Done():
					return
				case s, ok := <-in:
					if !ok {
						return
					}
					res := proto.FromUTF8String(s.UTF8String()).WithIndex(s.GetIndex())
					if err := s.GetError(); err != nil {
						res = res.WithError(err)
					}
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
				}
			}
		}()
		return out
	})

	ioT := NewIOReaderTranscoder[carrier.String](toJSON, reader)
	ioT.SetContext(ctx)
	ioT.SetSplitFunc(ScanJSON)

	outCh := ioT.Start()
	items, err := collectWithContext(ctx, outCh)
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}
	sortByIndex(items)

	want := []string{
		`{"a":1}`,
		`[1,2,{"b":"x"}]`,
		`{"c":"{[\"}]"}`,
	}

	if len(items) != len(want) {
		t.Fatalf("unexpected output count: got %d want %d items=%#v", len(items), len(want), items)
	}
	for i := range want {
		if got := items[i].UTF8String(); got != want[i] {
			t.Fatalf("token %d mismatch: got %q want %q", i, got, want[i])
		}
		if gotIdx, wantIdx := items[i].GetIndex(), i; gotIdx != wantIdx {
			t.Fatalf("token %d index mismatch: got %d want %d", i, gotIdx, wantIdx)
		}
	}
}
