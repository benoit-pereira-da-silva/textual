// Copyright 2026 Benoit Pereira da Silva
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
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
)

func TestIOReaderProcessor_Start_ScanLinesAndIndexes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	input := "a\nb\nc\n"
	reader := strings.NewReader(input)

	upper := ProcessorFunc[StringCarrier](func(ctx context.Context, in <-chan StringCarrier) <-chan StringCarrier {
		return Async(ctx, in, func(_ context.Context, s StringCarrier) StringCarrier {
			s.Value = strings.ToUpper(s.Value)
			return s
		})
	})

	p := NewIOReaderProcessor[StringCarrier](upper, reader)
	p.SetContext(ctx)

	outCh := p.Start()
	items, err := collectWithContext(ctx, outCh)
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}

	sortByIndex(items)

	if len(items) != 3 {
		t.Fatalf("unexpected output count: got %d want %d items=%#v", len(items), 3, items)
	}

	if items[0].Value != "A" || items[0].Index != 0 {
		t.Fatalf("unexpected item[0]: %#v", items[0])
	}
	if items[1].Value != "B" || items[1].Index != 1 {
		t.Fatalf("unexpected item[1]: %#v", items[1])
	}
	if items[2].Value != "C" || items[2].Index != 2 {
		t.Fatalf("unexpected item[2]: %#v", items[2])
	}
}

func TestIOReaderProcessor_CustomSplit_ReconstructsInput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	const input = "Hello, world!\nThis  is\ttextual.\n"
	reader := strings.NewReader(input)

	identity := ProcessorFunc[StringCarrier](func(ctx context.Context, in <-chan StringCarrier) <-chan StringCarrier {
		return Async(ctx, in, func(_ context.Context, s StringCarrier) StringCarrier {
			return s
		})
	})

	p := NewIOReaderProcessor[StringCarrier](identity, reader)
	p.SetContext(ctx)
	p.SetSplitFunc(ScanExpression)

	outCh := p.Start()
	items, err := collectWithContext(ctx, outCh)
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}

	var b strings.Builder
	for _, it := range items {
		b.WriteString(it.UTF8String())
	}

	if got := b.String(); got != input {
		t.Fatalf("reconstructed text mismatch:\n got: %q\nwant: %q", got, input)
	}
}
