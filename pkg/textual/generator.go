package textual

func Generator[P any](items ...P) <-chan P {
	out := make(chan P)
	go func() {
		defer close(out) // close the channel
		for _, item := range items {
			out <- item // Send each item
		}
	}()
	return out
}
