package textual

import "context"

type Composer struct {
	processors []Processor
}

func NewComposer() *Composer {
	return &Composer{
		processors: []Processor{},
	}
}
func (c *Composer) Add(processor ...Processor) {
	c.processors = append(c.processors, processor...)
}

func (c *Composer) Apply(ctx context.Context, in <-chan Result) <-chan Result {
	return nil
}
