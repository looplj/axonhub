package xcache

import (
	"time"

	"github.com/eko/gocache/lib/v4/store"
)

type Option = store.Option

func WithExpiration(expiration time.Duration) Option {
	return store.WithExpiration(expiration)
}
