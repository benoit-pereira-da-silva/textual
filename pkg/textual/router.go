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

// RoutePredicate decides whether a given Result should be handled by a route.
//
// The predicate has access to:
//   - ctx: the processing context (for deadlines / external state).
//   - res: the current Result.
//
// Examples:
//
//	// Route Result that still has raw text.
//	pred := func(ctx context.Context, res Result) bool {
//	    return len(res.RawTexts()) > 0
//	}
//
//	// Route Result where Error is a "reachability" error.
//	pred := func(ctx context.Context, res Result) bool {
//	    var re *ReachabilityError
//	    return errors.As(res.Error, &re)
//	}
type RoutePredicate func(ctx context.Context, res Result) bool

// RoutingStrategy controls how the Router selects target routes among the ones
// whose predicate matches.
type RoutingStrategy int

const (
	// RoutingStrategyFirstMatch sends each Result to the first route whose
	// predicate returns true. If multiple routes match, only the first one
	// (in registration order) is used.
	RoutingStrategyFirstMatch RoutingStrategy = iota

	// RoutingStrategyBroadcast sends each Result to every route whose
	// predicate returns true (or to all routes when predicates are nil).
	RoutingStrategyBroadcast

	// RoutingStrategyRoundRobin distributes Results over the set of routes
	// whose predicate returns true using a round‑robin counter.
	RoutingStrategyRoundRobin

	// RoutingStrategyRandom randomly picks a route among those whose
	// predicate returns true.
	RoutingStrategyRandom
)

// route is an internal configuration element combining a Processor and its
// selection predicate.
type route struct {
	processor Processor
	predicate RoutePredicate // nil means "always eligible"
}

// Router is a Processor that routes incoming Results to one or more downstream
// processors according to configurable predicates and a routing strategy.
//
// Typical usages:
//
//   - Conditional routing:
//
//     router := NewRouter(RoutingStrategyFirstMatch)
//     router.AddRoute(
//     func(ctx context.Context, res Result) bool {
//     // If there is still raw text, send to a dictionary processor.
//     return len(res.RawTexts()) > 0
//     },
//     dictProcessor,
//     )
//
//     router.AddRoute(
//     func(ctx context.Context, res Result) bool {
//     // If the error is a reachability issue, send to a fallback processor.
//     // isReachabilityError is left to the caller to implement.
//     return isReachabilityError(res.Error)
//     },
//     fallbackProcessor,
//     )
//
//   - Load‑balancing / randomization:
//
//     router := NewRouter(RoutingStrategyRoundRobin, procA, procB, procC)
//     // No predicates means all processors are always eligible.
//     // Results will be distributed A -> B -> C -> A -> ...
//
//   - Broadcasting:
//
//     router := NewRouter(RoutingStrategyBroadcast)
//     router.AddProcessor(loggingProcessor)
//     router.AddProcessor(transformProcessor)
//
//     // Every Result goes to both processors; their outputs are merged.
type Router struct {
	routes   []route
	strategy RoutingStrategy

	mu      sync.Mutex // protects rnd and counter
	counter uint64
	rnd     *rand.Rand
}

// NewRouter constructs a new Router with the given strategy.
//
// Optionally, a list of processors can be provided. They are registered as
// routes with no predicate (always eligible). This is useful for simple
// balancing setups (round‑robin, random, broadcast) where routing does
// not depend on the Result content.
func NewRouter(strategy RoutingStrategy, processors ...Processor) *Router {
	r := &Router{
		strategy: strategy,
		rnd:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	for _, p := range processors {
		if p == nil {
			continue
		}
		r.routes = append(r.routes, route{processor: p})
	}
	return r
}

// AddRoute registers a new route with an optional predicate.
//
//   - If predicate is nil, the route is always considered eligible.
//   - If processor is nil, the route is ignored.
//
// This method is not concurrency‑safe; it is intended to be called during
// pipeline construction, before any call to Apply.
func (r *Router) AddRoute(predicate RoutePredicate, processor Processor) {
	if processor == nil {
		return
	}
	r.routes = append(r.routes, route{
		processor: processor,
		predicate: predicate,
	})
}

// AddProcessor is a convenience wrapper around AddRoute for routes that are
// always eligible (predicate == nil).
func (r *Router) AddProcessor(processor Processor) {
	r.AddRoute(nil, processor)
}

// SetStrategy changes the routing strategy.
//
// This method is not concurrency‑safe; it is intended to be called during
// pipeline construction, before any call to Apply.
func (r *Router) SetStrategy(strategy RoutingStrategy) {
	r.strategy = strategy
}

// Apply implements the Processor interface.
//
// It wires all configured routes into a fan‑out / fan‑in pattern:
//
//   - Fan‑out: Results coming from `in` are dispatched to zero, one, or many
//     route‑specific input channels, depending on predicates and strategy.
//   - Fan‑in: all downstream route outputs are merged back into a single
//     output channel, which is returned to the caller.
//
// Context handling:
//
//   - If ctx is nil, context.Background() is used.
//   - The same ctx is passed to every underlying Processor.
//   - When ctx is canceled, the router stops reading from the upstream `in`,
//     closes all route inputs, and drains all child outputs before closing
//     the returned channel.
//
// Routing semantics:
//
//   - If no route is configured, Router behaves as a pass‑through Processor.
//   - For each incoming Result:
//   - the set of eligible routes is computed (predicate true or nil).
//   - strategy decides which subset of eligible routes receives the Result.
//   - if no routes are selected, the Result is forwarded unchanged to the
//     output channel (pass‑through behavior).
func (r *Router) Apply(ctx context.Context, in <-chan Result) <-chan Result {
	if ctx == nil {
		ctx = context.Background()
	}

	// No routes: transparent pass‑through Processor.
	if len(r.routes) == 0 {
		out := make(chan Result)
		go func() {
			defer close(out)
			for {
				select {
				case <-ctx.Done():
					// Stop emitting new results on cancellation.
					return
				case res, ok := <-in:
					if !ok {
						return
					}
					select {
					case <-ctx.Done():
						// Context canceled while sending.
						return
					case out <- res:
					}
				}
			}
		}()
		return out
	}

	// Create one input channel per route and start each underlying Processor.
	childIns := make([]chan Result, len(r.routes))
	childOuts := make([]<-chan Result, len(r.routes))

	for i, rt := range r.routes {
		ch := make(chan Result)
		childIns[i] = ch
		childOuts[i] = rt.processor.Apply(ctx, ch)
	}

	out := make(chan Result)

	// Fan‑in: merge all child outputs into the single out channel.
	var wg sync.WaitGroup
	wg.Add(len(childOuts))

	for i := range childOuts {
		go func(ch <-chan Result) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					// Context canceled: drain remaining results from the child
					// channel so that downstream processors are not blocked on
					// send, but do not forward them anymore.
					for range ch {
					}
					return
				case res, ok := <-ch:
					if !ok {
						// Child processor closed its output.
						return
					}
					// Normal operation: forward to the merged output.
					select {
					case out <- res:
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

	// Fan‑out: dispatch incoming Results to the selected routes.
	go func() {
		defer func() {
			// Signal downstream processors that no more input will arrive.
			for _, ch := range childIns {
				close(ch)
			}
			// Wait until all fan‑in goroutines have completed, then close
			// the merged output channel.
			wg.Wait()
			close(out)
		}()

		for {
			select {
			case <-ctx.Done():
				// Stop reading from upstream when the context is canceled.
				return
			case res, ok := <-in:
				if !ok {
					// Upstream closed; we're done.
					return
				}

				// Resolve which routes should receive this Result.
				indices := r.selectRoutes(ctx, res)
				if len(indices) == 0 {
					// No matching route: behave as pass‑through.
					select {
					case <-ctx.Done():
						return
					case out <- res:
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
					case childIns[idx] <- res:
					}
				}
			}
		}
	}()

	return out
}

// eligibleRoutes returns the indices of routes whose predicate matches
// the given Result (or all routes with nil predicates).
func (r *Router) eligibleRoutes(ctx context.Context, res Result) []int {
	indices := make([]int, 0, len(r.routes))
	for i, rt := range r.routes {
		if rt.processor == nil {
			continue
		}
		if rt.predicate == nil || rt.predicate(ctx, res) {
			indices = append(indices, i)
		}
	}
	return indices
}

// selectRoutes picks one or more routes among the eligible ones according to
// the configured routing strategy.
func (r *Router) selectRoutes(ctx context.Context, res Result) []int {
	eligible := r.eligibleRoutes(ctx, res)
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
		// Route Randomly to one among the matching routes.
		r.mu.Lock()
		idx := r.rnd.Intn(len(eligible))
		chosen := eligible[idx]
		r.mu.Unlock()
		return []int{chosen}

	case RoutingStrategyRoundRobin:
		//Route to one among matching routes, balance the load equitably.
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
