package observability

import "context"

type Collector struct {
	ch chan RunEvent
}

func NewCollector(buffer int) *Collector {
	return &Collector{
		ch: make(chan RunEvent, buffer),
	}
}

func (c *Collector) Emit(event RunEvent) {
	select {
	case c.ch <- event:
	default:
	}
}

func (c *Collector) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-c.ch:
			if !ok {
				return
			}
			_ = event
		}
	}
}
