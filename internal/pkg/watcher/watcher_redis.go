package watcher

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/redis/go-redis/v9"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xredis"
)

type RedisWatcherOptions struct {
	Channel string
	Buffer  int
}

type redisWatcher[T any] struct {
	client  *redis.Client
	channel string
	buffer  int

	mu     sync.Mutex
	nextID uint64
	subs   map[uint64]chan T

	active int
	pubsub *redis.PubSub
	cancel context.CancelFunc
}

func NewRedisWatcher[T any](client *redis.Client, opts RedisWatcherOptions) (Notifier[T], error) {
	if client == nil {
		return nil, errors.New("watcher.RedisWatcher: redis client is required")
	}

	if opts.Channel == "" {
		return nil, errors.New("watcher.RedisWatcher: channel is required")
	}

	buffer := opts.Buffer
	if buffer <= 0 {
		buffer = 1
	}

	return &redisWatcher[T]{
		client:  client,
		channel: opts.Channel,
		buffer:  buffer,
		subs:    make(map[uint64]chan T),
	}, nil
}

func NewRedisWatcherFromConfig[T any](cfg xredis.Config, opts RedisWatcherOptions) (Notifier[T], error) {
	client, err := xredis.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	return NewRedisWatcher[T](client, opts)
}

func (w *redisWatcher[T]) Watch() (<-chan T, func()) {
	w.mu.Lock()
	defer w.mu.Unlock()

	id := w.nextID
	w.nextID++

	ch := make(chan T, w.buffer)
	w.subs[id] = ch

	w.active++
	if w.active == 1 {
		w.startLocked()
	}

	return ch, func() {
		w.mu.Lock()
		defer w.mu.Unlock()

		sub, ok := w.subs[id]
		if !ok {
			return
		}

		delete(w.subs, id)
		close(sub)

		w.active--
		if w.active == 0 {
			w.stopLocked()
		}
	}
}

func (w *redisWatcher[T]) Notify(ctx context.Context, v T) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return w.client.Publish(ctx, w.channel, payload).Err()
}

func (w *redisWatcher[T]) startLocked() {
	if w.pubsub != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel

	w.pubsub = w.client.Subscribe(ctx, w.channel)
	_, _ = w.pubsub.Receive(ctx)

	ps := w.pubsub
	go func(ps *redis.PubSub) {
		for {
			msg, err := ps.ReceiveMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, redis.ErrClosed) || ctx.Err() != nil {
					return
				}

				log.Warn(context.Background(), "watcher redis watcher receive failed",
					log.String("channel", w.channel),
					log.Cause(err))

				continue
			}

			var v T
			if err := json.Unmarshal([]byte(msg.Payload), &v); err != nil {
				log.Warn(context.Background(), "watcher redis watcher decode failed",
					log.String("channel", w.channel),
					log.String("payload", msg.Payload),
					log.Cause(err))

				continue
			}

			w.mu.Lock()

			for _, sub := range w.subs {
				select {
				case sub <- v:
				default:
				}
			}

			w.mu.Unlock()
		}
	}(ps)
}

func (w *redisWatcher[T]) stopLocked() {
	if w.cancel != nil {
		w.cancel()
		w.cancel = nil
	}

	if w.pubsub != nil {
		_ = w.pubsub.Close()
		w.pubsub = nil
	}
}
