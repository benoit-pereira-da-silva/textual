package textual

import "context"

// SyncApply applies a Processor to a single input Result and returns the resulting Result.
// If no results are produced by the processor, it returns the input Result.
// If multiple results are produced, they are aggregated.
// Context cancellation is respected during processing.
func SyncApply[P Processor](ctx context.Context, p P, in Result) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	inCh := make(chan Result, 1)
	inCh <- in
	close(inCh)
	outCh := p.Apply(ctx, inCh)
	results := make(Results, 0, 1)
	for res := range outCh {
		results = append(results, res)
	}
	if len(results) == 0 {
		// Pass‑through in the degenerate case.
		return in
	}
	if len(results) == 1 {
		return results[0]
	}
	// Aggregate if we received multiple results.
	return results.Aggregated()
}
