package kubectl

import (
	"context"
)

type Executor interface {
	Exec(ctx context.Context, args ...string) ([]byte, error)
}
