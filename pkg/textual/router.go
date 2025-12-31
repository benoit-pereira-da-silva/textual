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
	"math/rand"
	"sync"
	"time"
)

// RoutePredicate decides whether a given item should be handled by a route.
//
// The predicate has access to:
//   - ctx: the processing context (for deadlines / external state).
//   - item: the current value flowing through the pipeline.
//
// Examples:
//
//	// Route Parcel values that still have raw text.
//	pred := func(ctx context.Context, res Parcel) bool {
//		return len(res.RawTexts()) > 0
//	}
//
//	// Route StringCarrier values matching a prefix.
//	pred2 := func(ctx context.Context, s StringCarrier) bool {
//		return strings.HasPrefix(s.Value, "WARN")
//	}
type RoutePredicate[S Carrier[S]] func(ctx context.Context, item S) bool

// RoutingStrategy controls how the Router selects target routes among the ones
// whose predicate matches.
type RoutingStrategy int

const (
	// RoutingStrategyFirstMatch sends each item to the first route whose
	// predicate returns true. If multiple routes match, only the first one
	// (in registration order) is used.
	RoutingStrategyFirstMatch RoutingStrategy = iota

	// RoutingStrategyBroadcast sends each item to every route whose predicate
	// returns true (or to all routes when predicates are nil).
	RoutingStrategyBroadcast

	// RoutingStrategyRoundRobin distributes items over the set of routes whose
	// predicate returns true using a round-robin counter.
	RoutingStrategyRoundRobin

	// RoutingStrategyRandom randomly picks a route among those whose predicate
	// returns true.
	RoutingStrategyRandom
)

// route is an internal configuration element combining a Processor and its
// selection predicate.
type route[S Carrier[S]] struct {
	processor Processor[S]
	predicate RoutePredicate[S] // nil means "always eligible"
}

// Router is a Processor that routes incoming items to one or more downstream
// processors according to configurable predicates and a routing strategy.
//
// Router implements a fan-out / fan-in pattern:
//
//   - Fan-out: items coming from `in` are dispatched to zero, one, or many
//     route-specific input channels, depending on predicates and strategy.
//   - Fan-in: all downstream outputs are merged back into a single output
//     channel, which is returned to the caller.
//
// Routing semantics:
//
//   - If no route is configured, Router behaves as a pass-through Processor.
//   - For each incoming item:
//     1) the set of eligible routes is computed (predicate true or nil),
//     2) the strategy decides which subset of eligible routes receives the item,
//     3) if no route is selected, the item is forwarded unchanged.
//
// Note: AddRoute/AddProcessor/SetStrategy are not concurrency-safe; configure
// the router during pipeline construction, before calling Apply.
type Router[S Carrier[S]] struct {
	routes   []route[S]
	strategy RoutingStrategy

	mu      sync.Mutex // protects rnd and counter
	counter uint64
	rnd     *rand.Rand
}

// NewRouter constructs a new Router with the given strategy.
//
// Optionally, a list of processors can be provided. They are registered as
// routes with no predicate (always eligible). This is useful for simple
// balancing setups (round-robin, random, broadcast) where routing does not
// depend on the item content.
func NewRouter[S Carrier[S]](strategy RoutingStrategy, processors ...Processor[S]) *Router[S] {
	r := &Router[S]{
		strategy: strategy,
		rnd:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	for _, p := range processors {
		if p == nil {
			continue
		}
		r.routes = append(r.routes, route[S]{processor: p})
	}
	return r
}

// AddRoute registers a new route with an optional predicate.
//
//   - If predicate is nil, the route is always considered eligible.
//   - If processor is nil, the route is ignored.
func (r *Router[S]) AddRoute(predicate RoutePredicate[S], processor Processor[S]) {
	if processor == nil {
		return
	}
	r.routes = append(r.routes, route[S]{
		processor: processor,
		predicate: predicate,
	})
}

// AddProcessor is a convenience wrapper around AddRoute for routes that are
// always eligible (predicate == nil).
func (r *Router[S]) AddProcessor(processor Processor[S]) {
	r.AddRoute(nil, processor)
}

// SetStrategy changes the routing strategy.
func (r *Router[S]) SetStrategy(strategy RoutingStrategy) {
	r.strategy = strategy
}

// Apply implements the Processor interface.
//
// Context handling:
//
//   - If ctx is nil, context.Background() is used.
//   - The same ctx is passed to every underlying Processor.
//   - When ctx is canceled, the router stops reading from `in`, closes all
//     route inputs, drains all child outputs, then closes the returned channel.
func (r *Router[S]) Apply(ctx context.Context, in <-chan S) <-chan S {
	if ctx == nil {
		ctx = context.Background()
	}

	// No routes: transparent pass-through Processor.
	if len(r.routes) == 0 {
		out := make(chan S)
		go func() {
			defer close(out)
			for {
				select {
				case <-ctx.Done():
					// Stop emitting new values on cancellation.
					return
				case item, ok := <-in:
					if !ok {
						return
					}
					select {
					case <-ctx.Done():
						// Context canceled while sending.
						return
					case out <- item:
					}
				}
			}
		}()
		return out
	}

	// Create one input channel per route and start each underlying Processor.
	childIns := make([]chan S, len(r.routes))
	childOuts := make([]<-chan S, len(r.routes))

	for i, rt := range r.routes {
		ch := make(chan S)
		childIns[i] = ch
		childOuts[i] = rt.processor.Apply(ctx, ch)
	}

	out := make(chan S)

	// Fan-in: merge all child outputs into the single out channel.
	var wg sync.WaitGroup
	wg.Add(len(childOuts))

	for i := range childOuts {
		go func(ch <-chan S) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					// Context canceled: drain remaining values from the child
					// channel so that downstream processors are not blocked on
					// send, but do not forward them anymore.
					for range ch {
					}
					return
				case item, ok := <-ch:
					if !ok {
						// Child processor closed its output.
						return
					}
					// Normal operation: forward to the merged output.
					select {
					case out <- item:
					case <-ctx.Done():
						// Context canceled while sending: start draining.
						for range ch {
						}
						return
					}
				}
			}
		}(childOuts[i])
	}

	// Fan-out: dispatch incoming items to the selected routes.
	go func() {
		defer func() {
			// Signal downstream processors that no more input will arrive.
			for _, ch := range childIns {
				close(ch)
			}
			// Wait until all fan-in goroutines have completed, then close
			// the merged output channel.
			wg.Wait()
			close(out)
		}()

		for {
			select {
			case <-ctx.Done():
				// Stop reading from upstream when the context is canceled.
				return
			case item, ok := <-in:
				if !ok {
					// Upstream closed; we're done.
					return
				}

				// Resolve which routes should receive this item.
				indices := r.selectRoutes(ctx, item)
				if len(indices) == 0 {
					// No matching route: behave as pass-through.
					select {
					case <-ctx.Done():
						return
					case out <- item:
					}
					continue
				}

				// Dispatch to every selected route.
				for _, idx := range indices {
					if idx < 0 || idx >= len(childIns) {
						// Defensive bounds check; should never happen.
						continue
					}

					select {
					case <-ctx.Done():
						return
					case childIns[idx] <- item:
					}
				}
			}
		}
	}()

	return out
}

// eligibleRoutes returns the indices of routes whose predicate matches the
// given item (or all routes with nil predicates).
func (r *Router[S]) eligibleRoutes(ctx context.Context, item S) []int {
	indices := make([]int, 0, len(r.routes))
	for i, rt := range r.routes {
		if rt.processor == nil {
			continue
		}
		if rt.predicate == nil || rt.predicate(ctx, item) {
			indices = append(indices, i)
		}
	}
	return indices
}

// selectRoutes picks one or more routes among the eligible ones according to
// the configured routing strategy.
func (r *Router[S]) selectRoutes(ctx context.Context, item S) []int {
	eligible := r.eligibleRoutes(ctx, item)
	if len(eligible) == 0 {
		return nil
	}

	switch r.strategy {
	case RoutingStrategyBroadcast:
		// Route to every matching route.
		return eligible

	case RoutingStrategyFirstMatch:
		// Route only to the first matching route.
		return []int{eligible[0]}

	case RoutingStrategyRandom:
		// Route randomly to one among the matching routes.
		r.mu.Lock()
		idx := r.rnd.Intn(len(eligible))
		chosen := eligible[idx]
		r.mu.Unlock()
		return []int{chosen}

	case RoutingStrategyRoundRobin:
		// Route to one among matching routes, balancing load equitably.
		r.mu.Lock()
		idx := int(r.counter % uint64(len(eligible)))
		chosen := eligible[idx]
		r.counter++
		r.mu.Unlock()
		return []int{chosen}

	default:
		// Fallback: behave like broadcast.
		return eligible
	}
}
