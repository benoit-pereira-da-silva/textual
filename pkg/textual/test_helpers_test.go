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
	"sort"
)

// collectWithContext drains a channel until it is closed or ctx is done.
// It is used by tests to avoid hanging indefinitely if a stage forgets to close
// its output channel.
func collectWithContext[T any](ctx context.Context, ch <-chan T) ([]T, error) {
	items := make([]T, 0, 8)
	for {
		select {
		case <-ctx.Done():
			return items, ctx.Err()
		case v, ok := <-ch:
			if !ok {
				return items, nil
			}
			items = append(items, v)
		}
	}
}

// sortByIndex sorts carriers by their GetIndex() value.
// This is useful because router fan-in merges outputs concurrently, so order is
// not deterministic.
func sortByIndex[S Carrier[S]](items []S) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].GetIndex() < items[j].GetIndex()
	})
}
