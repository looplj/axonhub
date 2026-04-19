package orchestrator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsRequestWindowDbQueried_NotQueried(t *testing.T) {
	tracker := NewChannelRequestTracker()

	assert.False(t, tracker.IsRequestWindowDbQueried(1, time.Minute, nil))
}

func TestIsRequestWindowDbQueried_AfterMark(t *testing.T) {
	tracker := NewChannelRequestTracker()

	tracker.MarkRequestWindowDbQueried(1, time.Minute, nil)

	assert.True(t, tracker.IsRequestWindowDbQueried(1, time.Minute, nil))
}

func TestIsRequestWindowDbQueried_DifferentChannel(t *testing.T) {
	tracker := NewChannelRequestTracker()

	tracker.MarkRequestWindowDbQueried(1, time.Minute, nil)

	assert.True(t, tracker.IsRequestWindowDbQueried(1, time.Minute, nil))
	assert.False(t, tracker.IsRequestWindowDbQueried(2, time.Minute, nil))
}

func TestIsRequestWindowDbQueried_DifferentDuration(t *testing.T) {
	tracker := NewChannelRequestTracker()

	tracker.MarkRequestWindowDbQueried(1, time.Minute, nil)

	assert.True(t, tracker.IsRequestWindowDbQueried(1, time.Minute, nil))
	assert.False(t, tracker.IsRequestWindowDbQueried(1, time.Hour, nil))
}

func TestIsRequestWindowDbQueried_ResetsOnWindowRotation(t *testing.T) {
	now := time.Now()
	tracker := NewChannelRequestTracker(WithClock(func() time.Time { return now }))

	tracker.MarkRequestWindowDbQueried(1, time.Minute, nil)
	assert.True(t, tracker.IsRequestWindowDbQueried(1, time.Minute, nil))

	// Advance time past the window duration
	tracker.clock = func() time.Time { return now.Add(2 * time.Minute) }

	// Window rotated — dbQueried flag should be gone since the window was reset
	assert.False(t, tracker.IsRequestWindowDbQueried(1, time.Minute, nil))
}

func TestIsTokenWindowDbQueried_NotQueried(t *testing.T) {
	tracker := NewChannelRequestTracker()

	assert.False(t, tracker.IsTokenWindowDbQueried(1, time.Minute, nil))
}

func TestIsTokenWindowDbQueried_AfterMark(t *testing.T) {
	tracker := NewChannelRequestTracker()

	tracker.MarkTokenWindowDbQueried(1, time.Minute, nil)

	assert.True(t, tracker.IsTokenWindowDbQueried(1, time.Minute, nil))
}

func TestIsTokenWindowDbQueried_DifferentChannel(t *testing.T) {
	tracker := NewChannelRequestTracker()

	tracker.MarkTokenWindowDbQueried(1, time.Minute, nil)

	assert.True(t, tracker.IsTokenWindowDbQueried(1, time.Minute, nil))
	assert.False(t, tracker.IsTokenWindowDbQueried(2, time.Minute, nil))
}

func TestIsTokenWindowDbQueried_ResetsOnWindowRotation(t *testing.T) {
	now := time.Now()
	tracker := NewChannelRequestTracker(WithClock(func() time.Time { return now }))

	tracker.MarkTokenWindowDbQueried(1, time.Minute, nil)
	assert.True(t, tracker.IsTokenWindowDbQueried(1, time.Minute, nil))

	// Advance time past the window duration
	tracker.clock = func() time.Time { return now.Add(2 * time.Minute) }

	// Window rotated — dbQueried flag should be gone
	assert.False(t, tracker.IsTokenWindowDbQueried(1, time.Minute, nil))
}
