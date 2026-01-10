package textual

import (
	"context"
	"log/slog"
)

// Slog is a processor that logs the content of carriers with their index and string representation
func Slog[C Carrier[C]](label string) ProcessorFunc[C] {
	return ProcessorFunc[C](func(ctx context.Context, in <-chan C) <-chan C {
		return Async(ctx, in, func(ctx context.Context, p C) C {
			s := p.UTF8String()
			err := p.GetError()
			if err != nil {
				slog.Error(label, "err", err, "index", p.GetIndex(), "string", s)
			} else {
				slog.Info(label, "index", p.GetIndex(), "string", s)
			}
			return p
		})
	})
}
