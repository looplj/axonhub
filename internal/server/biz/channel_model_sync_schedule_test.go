package biz

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestChannelService_ShouldRunModelSync_SameIntervalSkips(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	now := time.Date(2024, 1, 1, 10, 2, 0, 0, time.UTC)
	require.True(t, svc.shouldRunModelSync(now, AutoSyncFrequencyOneHour))
	require.False(t, svc.shouldRunModelSync(now.Add(2*time.Minute), AutoSyncFrequencyOneHour))
}

func TestChannelService_ShouldRunModelSync_NextIntervalRuns(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	first := time.Date(2024, 1, 1, 10, 2, 0, 0, time.UTC)
	next := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)

	require.True(t, svc.shouldRunModelSync(first, AutoSyncFrequencyOneHour))
	require.True(t, svc.shouldRunModelSync(next, AutoSyncFrequencyOneHour))
}

func TestChannelService_ShouldRunModelSync_DefaultHourly(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	first := time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC)
	sameHour := time.Date(2024, 1, 1, 10, 59, 0, 0, time.UTC)
	nextHour := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)

	require.True(t, svc.shouldRunModelSync(first, AutoSyncFrequencyOneHour))
	require.False(t, svc.shouldRunModelSync(sameHour, AutoSyncFrequencyOneHour))
	require.True(t, svc.shouldRunModelSync(nextHour, AutoSyncFrequencyOneHour))
}

func TestChannelService_ShouldRunModelSync_SixHourInterval(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	first := time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC)
	sameWindow := time.Date(2024, 1, 1, 11, 59, 0, 0, time.UTC)
	nextWindow := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	require.True(t, svc.shouldRunModelSync(first, AutoSyncFrequencySixHours))
	require.False(t, svc.shouldRunModelSync(sameWindow, AutoSyncFrequencySixHours))
	require.True(t, svc.shouldRunModelSync(nextWindow, AutoSyncFrequencySixHours))
}

func TestChannelService_ShouldRunModelSync_DailyInterval(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	first := time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC)
	sameWindow := time.Date(2024, 1, 1, 23, 59, 0, 0, time.UTC)
	nextWindow := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

	require.True(t, svc.shouldRunModelSync(first, AutoSyncFrequencyOneDay))
	require.False(t, svc.shouldRunModelSync(sameWindow, AutoSyncFrequencyOneDay))
	require.True(t, svc.shouldRunModelSync(nextWindow, AutoSyncFrequencyOneDay))
}

