package go_lock

import (
	"context"
)

type Lock interface {
	Lock(ctx context.Context) error
	Unlock(ctx context.Context) error
}
