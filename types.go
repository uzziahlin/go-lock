package lock

import (
	"context"
)

// Lock 核心接口，对Lock的抽象，可以提供各种各样的实现
type Lock interface {
	Lock(ctx context.Context) error
	Unlock(ctx context.Context) error
}
