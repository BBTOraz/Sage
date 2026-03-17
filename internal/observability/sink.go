package observability

import "context"

type Sink interface {
	Write(ctx context.Context, data any) error
}
