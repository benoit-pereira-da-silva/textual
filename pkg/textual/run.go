package textual

import "context"

// Run runs aProcessor on a given text and returns a Result channel
//
// The returned stream is cancellable via ctx. If ctx is canceled before
// the initial Result is sent to the processor, no input is provided and
// the input channel is simply closed.
func Run[P Processor](ctx context.Context, text UTF8String, p P) <-chan Result {
	// Channel carrying the initial Result toward the processor. A small
	// buffer of 1 avoids a potential deadlock if the processor expects
	// the channel to be ready to receive before returning from
	// StreamApply.
	inChan := make(chan Result, 1)
	// Feed the initial Result in a dedicated goroutine so that Start does
	// not block and ctx cancellation is honored while sending.
	go func() {
		defer close(inChan)
		select {
		case <-ctx.Done():
			// Context canceled before we could send the initial value.
			return
		case inChan <- Input(text):
		}
	}()
	return p.Apply(ctx, inChan)
}
