package distributed

import (
	"context"
	"sync"
)

type Lock interface {
	sync.Locker
	TryLock(ctx context.Context) error
}
