package tea

import (
	"context"
	"io"
)

func ReadInputs(ctx context.Context, msgs chan<- Msg, input io.Reader) {
	readInputs(ctx, msgs, input)
}
