package pelican

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestService_ClosesResultsInterruptedByARestart(t *testing.T) {
	store := NewStoreAt(t.TempDir())
	// A restart happens while an attempt is in flight: the entry was persisted as running and can
	// never finish, so it would stay a spinner in the gallery forever.
	require.NoError(t, store.Update(func(_ *Config, results *[]Result) error {
		*results = append(*results,
			Result{ID: "interrupted", Model: "m", Status: StatusRunning},
			Result{ID: "done", Model: "m", Status: StatusSucceeded},
		)
		return nil
	}))
	service := newService(store, newRunner(store, &fakeCompleter{}), time.Now)

	service.failInterruptedResults(context.Background())

	_, results, err := store.Load()
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, StatusFailed, results[0].Status)
	require.Contains(t, results[0].Error, "interrupted")
	require.Equal(t, StatusSucceeded, results[1].Status, "finished attempts are left alone")
}

func TestService_RunsScheduledRoundWithoutAProject(t *testing.T) {
	store := NewStoreAt(t.TempDir())
	completer := &fakeCompleter{}
	runner := newRunner(store, completer)
	service := newService(store, runner, func() time.Time { return time.Date(2026, 9, 23, 7, 20, 0, 0, time.UTC) })

	// Admin requests never carry a project id, so a configuration saved from the UI has none. The
	// project only scopes prompt injection, so its absence must not block the hourly round.
	require.NoError(t, store.SaveConfig(Config{
		Prompt:   DefaultPrompt,
		Schedule: Schedule{Enabled: true},
		Targets:  []Target{{Channel: 1, Model: "m"}},
	}))

	service.runScheduled(t.Context())

	calls, _ := completer.snapshot()
	require.Len(t, calls, 1, "the hourly round must run without a recorded project")
	config, results, err := store.Load()
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "2026-09-23T08:00:00Z", config.Schedule.NextRunAt, "the slot is consumed either way")
}
