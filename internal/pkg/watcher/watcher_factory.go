package watcher

import "errors"

type WatcherFromConfigOptions struct {
	RedisChannel string
	Buffer       int
}

func NewWatcherFromConfig[T any](cfg Config, opts WatcherFromConfigOptions) (Notifier[T], error) {
	switch cfg.Mode {
	case ModeRedis:
		if opts.RedisChannel == "" {
			return nil, errors.New("watcher: redis channel is required for redis/two-level mode")
		}

		return NewRedisWatcherFromConfig[T](cfg.Redis, RedisWatcherOptions{
			Channel: opts.RedisChannel,
			Buffer:  opts.Buffer,
		})
	default:
		return NewMemoryWatcher[T](MemoryWatcherOptions{Buffer: opts.Buffer}), nil
	}
}
