package textual

import (
	"context"
	"runtime/debug"
)

// ProcessorFunc is a function adapter that implements Processor.
//
// It allows plain functions to be used as Processor values:
//
//	p := ProcessorFunc[carrier.StringCarrier](func(ctx context.Context, in <-chan carrier.StringCarrier) <-chan carrier.StringCarrier {
//		return Async(ctx, in, func(ctx context.Context, s carrier.StringCarrier) carrier.StringCarrier {
//			s.Value = strings.ToUpper(s.Value)
//			return s
//		})
//	})
//
// This can make it easier to construct lightweight processors inline.
type ProcessorFunc[S Carrier[S]] func(ctx context.Context, in <-chan S) <-chan S

// Apply calls f(ctx, in).
//
// For safety, Apply enforces the Processor contract that the returned channel is
// never nil. If f panics (including the case where f is nil), the panic is
// recovered, recorded into the PanicStore carried by ctx (ensured via
// EnsurePanicStore), and a closed channel is returned.
func (f ProcessorFunc[S]) Apply(ctx context.Context, in <-chan S) (out <-chan S) {
	ctx, ps := EnsurePanicStore(ctx)

	defer func() {
		if r := recover(); r != nil {
			if ps != nil {
				ps.Store(r, debug.Stack())
			}
			out = closedChan[S]()
		}
	}()

	out = f(ctx, in)
	if out == nil {
		if ps != nil {
			ps.Store("textual: ProcessorFunc returned a nil channel", debug.Stack())
		}
		out = closedChan[S]()
	}
	return out
}

// Chain composes one or more processors after this processor.
//
// Given receiver f and processors p1, p2, the resulting processor behaves like:
//
//	out := p2.Apply(ctx, p1.Apply(ctx, f.Apply(ctx, in)))
//
// Nil processors are ignored (via NewChain for n>1, and explicit checks for n==1).
func (f ProcessorFunc[S]) Chain(p ...Processor[S]) ProcessorFunc[S] {
	switch len(p) {
	case 0:
		return f
	case 1:
		if p[0] == nil {
			return f
		}
		next := p[0]
		return ProcessorFunc[S](func(ctx context.Context, in <-chan S) <-chan S {
			return next.Apply(ctx, f.Apply(ctx, in))
		})
	default:
		return NewChain[S](append([]Processor[S]{f}, p...)...)
	}
}

// ProcessorFuncFrom adapts a Processor into a ProcessorFunc.
//
// This is useful when working with APIs or helpers that expect a
// ProcessorFunc, but you already have a concrete Processor implementation.
//
// The returned ProcessorFunc simply delegates Apply calls to the
// underlying Processor.
func ProcessorFuncFrom[S Carrier[S]](p Processor[S]) ProcessorFunc[S] {
	if p == nil {
		return nil
	}
	return func(ctx context.Context, in <-chan S) <-chan S {
		return p.Apply(ctx, in)
	}
}
