package events

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
)

func TestPublishDeliversOnlyToMatchingSubscribers(t *testing.T) {
	bus := NewBus()
	t.Cleanup(func() {
		closeBus(t, bus)
	})

	var matched sync.Map
	other := make(chan Event, 1)
	bus.Subscribe("matched-a", []string{TypeRequestCompleted}, func(_ context.Context, event Event) {
		event.Data["touched"] = true
		matched.Store("a", event)
	})
	bus.Subscribe("matched-b", []string{TypeRequestCompleted}, func(_ context.Context, event Event) {
		matched.Store("b", event)
	})
	bus.Subscribe("other", []string{TypeRequestFailed}, func(_ context.Context, event Event) {
		other <- event
	})

	data := map[string]any{"latencyMs": 5}
	bus.Publish(context.Background(), Event{
		ID:   "evt_done",
		Type: TypeRequestCompleted,
		Data: data,
		Subject: Subject{
			Channel: &ChannelRef{ID: 4, Name: "primary"},
		},
	})
	data["latencyMs"] = 99

	require.Eventually(t, func() bool {
		_, okA := matched.Load("a")
		_, okB := matched.Load("b")
		return okA && okB && bus.Stats().Delivered == 2
	}, time.Second, 5*time.Millisecond)

	first, _ := matched.Load("a")
	second, _ := matched.Load("b")
	require.Equal(t, "evt_done", first.(Event).ID)
	require.Equal(t, 5, first.(Event).Data["latencyMs"])
	require.Equal(t, 5, second.(Event).Data["latencyMs"])
	require.Equal(t, 4, second.(Event).Subject.Channel.ID)
	require.Equal(t, 99, data["latencyMs"])
	require.NotContains(t, second.(Event).Data, "touched")

	require.Never(t, func() bool {
		select {
		case <-other:
			return true
		default:
			return false
		}
	}, 50*time.Millisecond, 5*time.Millisecond)

	stats := bus.Stats()
	require.Equal(t, int64(1), stats.Published)
	require.Equal(t, int64(2), stats.Delivered)
	require.Equal(t, int64(0), stats.DroppedDepth)
	require.Equal(t, int64(0), stats.DroppedQueue)
}

func TestHasSubscribersTracksLiveSubscriptions(t *testing.T) {
	bus := NewBus()
	t.Cleanup(func() {
		closeBus(t, bus)
	})

	require.False(t, bus.HasSubscribers(TypeRequestCompleted))
	require.False(t, bus.HasSubscribers(TypeRequestFailed))
	bus.Publish(context.Background(), Event{Type: TypeRequestCompleted})
	require.Equal(t, int64(1), bus.Stats().Published)
	require.Equal(t, int64(0), bus.Stats().Delivered)

	received := make(chan Event, 1)
	unsubscribe := bus.Subscribe("live", []string{TypeRequestCompleted, TypeRequestFailed}, func(_ context.Context, event Event) {
		received <- event
	})
	require.True(t, bus.HasSubscribers(TypeRequestCompleted))
	require.True(t, bus.HasSubscribers(TypeRequestFailed))
	require.False(t, bus.HasSubscribers(TypeRequestFailover))
	require.False(t, bus.HasSubscribers("request.failed.extra"))

	empty := bus.Subscribe("empty", nil, func(context.Context, Event) {
		t.Errorf("empty subscription received an event")
	})
	require.True(t, bus.HasSubscribers(TypeRequestCompleted))
	bus.Publish(context.Background(), Event{ID: "kept", Type: TypeRequestCompleted})
	require.Eventually(t, func() bool {
		return len(received) == 1
	}, time.Second, 5*time.Millisecond)
	require.Equal(t, "kept", (<-received).ID)

	unsubscribe()
	unsubscribe()
	empty()
	require.False(t, bus.HasSubscribers(TypeRequestCompleted))
	require.False(t, bus.HasSubscribers(TypeRequestFailed))

	bus.Publish(context.Background(), Event{ID: "after", Type: TypeRequestCompleted})
	require.Never(t, func() bool {
		return len(received) > 0
	}, 50*time.Millisecond, 5*time.Millisecond)
}

func TestSubscribeReplacesDuplicateName(t *testing.T) {
	bus := NewBus()
	t.Cleanup(func() {
		closeBus(t, bus)
	})

	first := make(chan struct{}, 1)
	second := make(chan struct{}, 2)
	previousUnsubscribe := bus.Subscribe("plugin", []string{TypeRequestFailover}, func(context.Context, Event) {
		first <- struct{}{}
	})
	previous := bus.subs["plugin"]

	unsubscribe := bus.Subscribe("plugin", []string{TypeRequestFailover}, func(context.Context, Event) {
		second <- struct{}{}
	})
	waitDone(t, previous)
	require.True(t, bus.HasSubscribers(TypeRequestFailover))

	bus.Publish(context.Background(), Event{Type: TypeRequestFailover, ID: "one"})
	select {
	case <-second:
	case <-time.After(time.Second):
		t.Fatal("replacement subscription did not receive the event")
	}
	require.Never(t, func() bool {
		select {
		case <-first:
			return true
		default:
			return false
		}
	}, 50*time.Millisecond, 5*time.Millisecond)

	previousUnsubscribe()
	require.True(t, bus.HasSubscribers(TypeRequestFailover))
	bus.Publish(context.Background(), Event{Type: TypeRequestFailover, ID: "two"})
	select {
	case <-second:
	case <-time.After(time.Second):
		t.Fatal("old unsubscribe stopped the replacement")
	}

	unsubscribe()
	unsubscribe()
	require.False(t, bus.HasSubscribers(TypeRequestFailover))
}

func TestDepthAboveTwoIsNotDelivered(t *testing.T) {
	bus := NewBus()
	t.Cleanup(func() {
		closeBus(t, bus)
	})

	received := make(chan Event, 2)
	bus.Subscribe("depth", []string{TypeRequestFailover}, func(_ context.Context, event Event) {
		received <- event
	})

	bus.Publish(context.Background(), Event{Type: TypeRequestFailover, ID: "deep", Depth: 3})
	require.Equal(t, int64(1), bus.Stats().DroppedDepth)
	require.Equal(t, int64(0), bus.Stats().Published)
	require.Never(t, func() bool {
		return len(received) > 0
	}, 50*time.Millisecond, 5*time.Millisecond)

	bus.Publish(context.Background(), Event{Type: TypeRequestFailover, ID: "edge", Depth: 2})
	require.Eventually(t, func() bool {
		return len(received) == 1 && bus.Stats().Delivered == 1
	}, time.Second, 5*time.Millisecond)
	require.Equal(t, "edge", (<-received).ID)
	require.Equal(t, int64(1), bus.Stats().DroppedDepth)
	require.Equal(t, int64(1), bus.Stats().Published)
	require.Equal(t, int64(1), bus.Stats().Delivered)
}

func TestFullQueueDropsWithoutBlocking(t *testing.T) {
	bus := NewBus()
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseHandler := func() {
		releaseOnce.Do(func() {
			close(release)
		})
	}
	t.Cleanup(func() {
		releaseHandler()
		closeBus(t, bus)
	})

	entered := make(chan struct{})
	var seen atomic.Int64
	var sawOverflow atomic.Bool
	bus.Subscribe("slow", []string{TypeRequestAttemptFailed}, func(_ context.Context, event Event) {
		if event.ID == "overflow" {
			sawOverflow.Store(true)
		}
		if event.ID == "block" {
			close(entered)
			<-release
		}
		seen.Add(1)
	})

	bus.Publish(context.Background(), Event{Type: TypeRequestAttemptFailed, ID: "block"})
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}

	publishBefore(t, bus, func() {
		for i := range subscriberQueueCapacity {
			bus.Publish(context.Background(), Event{
				Type: TypeRequestAttemptFailed,
				ID:   fmt.Sprintf("queued-%d", i),
			})
		}
	})
	require.Equal(t, int64(0), bus.Stats().DroppedQueue)

	publishBefore(t, bus, func() {
		bus.Publish(context.Background(), Event{Type: TypeRequestAttemptFailed, ID: "overflow"})
	})
	require.Equal(t, int64(1), bus.Stats().DroppedQueue)
	require.Equal(t, int64(subscriberQueueCapacity+2), bus.Stats().Published)

	releaseHandler()
	sub := bus.subs["slow"]
	require.Eventually(t, func() bool {
		return seen.Load() == int64(subscriberQueueCapacity+1) &&
			len(sub.queue) == 0 &&
			bus.Stats().Delivered == int64(subscriberQueueCapacity+1)
	}, 2*time.Second, 5*time.Millisecond)
	require.False(t, sawOverflow.Load())
	require.Equal(t, int64(subscriberQueueCapacity+1), bus.Stats().Delivered)
	require.Equal(t, int64(1), bus.Stats().DroppedQueue)
}

func TestUnsubscribeAndCloseReleaseWorker(t *testing.T) {
	bus := NewBus()
	unsubscribe := bus.Subscribe("once", []string{TypeRequestCompleted}, func(context.Context, Event) {})
	sub := bus.subs["once"]
	unsubscribe()
	unsubscribe()
	waitDone(t, sub)
	require.False(t, bus.HasSubscribers(TypeRequestCompleted))

	bus.Subscribe("again", []string{TypeRequestFailed}, func(context.Context, Event) {})
	sub = bus.subs["again"]
	closeBus(t, bus)
	waitDone(t, sub)
	require.False(t, bus.HasSubscribers(TypeRequestFailed))
	closeBus(t, bus)

	require.Eventually(t, func() bool {
		return !strings.Contains(goroutineDump(), "(*subscription).run")
	}, time.Second, 10*time.Millisecond)
}

func TestWorkerPanicStaysSubscribed(t *testing.T) {
	bus := NewBus()
	t.Cleanup(func() {
		closeBus(t, bus)
	})

	var calls atomic.Int32
	bus.Subscribe("boom", []string{TypeRequestFailed}, func(context.Context, Event) {
		if calls.Add(1) == 1 {
			panic("handler failed")
		}
	})
	bus.Publish(context.Background(), Event{Type: TypeRequestFailed, ID: "first"})
	bus.Publish(context.Background(), Event{Type: TypeRequestFailed, ID: "second"})

	require.Eventually(t, func() bool {
		return calls.Load() == 2 && bus.Stats().Delivered == 1
	}, time.Second, 5*time.Millisecond)
	require.True(t, bus.HasSubscribers(TypeRequestFailed))
}

func TestModuleOnStopClosesBus(t *testing.T) {
	var bus *Bus
	app := fx.New(
		fx.NopLogger,
		Module,
		fx.Populate(&bus),
	)
	require.NoError(t, app.Start(context.Background()))
	require.NotNil(t, bus)

	bus.Subscribe("fx", []string{TypeAPIKeyQuotaExceeded}, func(context.Context, Event) {})
	require.True(t, bus.HasSubscribers(TypeAPIKeyQuotaExceeded))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, app.Stop(ctx))
	require.False(t, bus.HasSubscribers(TypeAPIKeyQuotaExceeded))
}

func TestConcurrentPublishAndUnsubscribe(t *testing.T) {
	bus := NewBus()
	t.Cleanup(func() {
		closeBus(t, bus)
	})

	var publishing sync.WaitGroup
	stop := make(chan struct{})
	publishing.Go(func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Errorf("publisher panicked: %v", recovered)
			}
		}()
		for {
			select {
			case <-stop:
				return
			default:
				bus.Publish(context.Background(), Event{
					Type:  TypeRequestCompleted,
					Depth: 1,
					Data:  map[string]any{"latencyMs": 1},
					Subject: Subject{
						Channel: &ChannelRef{ID: 1, Name: "primary"},
					},
				})
			}
		}
	})

	var subscribers sync.WaitGroup
	for i := range 8 {
		subscribers.Add(1)
		go func(i int) {
			defer subscribers.Done()
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Errorf("subscriber panicked: %v", recovered)
				}
			}()
			unsubscribe := bus.Subscribe(fmt.Sprintf("sub-%d", i), []string{TypeRequestCompleted}, func(context.Context, Event) {})
			for n := range 20 {
				next := bus.Subscribe(fmt.Sprintf("sub-%d", i), []string{TypeRequestCompleted, TypeRequestFailed}, func(context.Context, Event) {})
				if n%2 == 0 {
					unsubscribe()
				}
				unsubscribe = next
			}
			unsubscribe()
			unsubscribe()
		}(i)
	}

	subscribers.Wait()
	close(stop)
	publishing.Wait()
}

func publishBefore(t *testing.T, bus *Bus, publish func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Errorf("publish panicked: %v", recovered)
			}
		}()
		publish()
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked")
	}
}

func closeBus(t *testing.T, bus *Bus) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, bus.Close(ctx))
}

func waitDone(t *testing.T, sub *subscription) {
	t.Helper()
	select {
	case <-sub.done:
	case <-time.After(time.Second):
		t.Fatal("worker did not exit")
	}
}

func goroutineDump() string {
	buf := make([]byte, 1<<16)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return string(buf[:n])
		}
		buf = make([]byte, len(buf)*2)
	}
}
