package pelican

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeCompleter stands in for the gateway: it records calls and tracks how many run at once.
type fakeCompleter struct {
	mu        sync.Mutex
	calls     []Target
	prompts   []string
	active    int
	maxActive int
	handle    func(ctx context.Context, target Target) (CompletionResult, error)
}

func (f *fakeCompleter) Complete(ctx context.Context, target Target, prompt string) (CompletionResult, error) {
	f.mu.Lock()
	f.calls = append(f.calls, target)
	f.prompts = append(f.prompts, prompt)
	f.active++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	handler := f.handle
	f.mu.Unlock()

	defer func() {
		f.mu.Lock()
		f.active--
		f.mu.Unlock()
	}()

	if handler == nil {
		return CompletionResult{Reply: "<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>", FinishReason: "stop"}, nil
	}
	return handler(ctx, target)
}

func (f *fakeCompleter) snapshot() ([]Target, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Target(nil), f.calls...), f.maxActive
}

func newTestRunner(t *testing.T, targets []Target, completer ChatCompleter, options ...RunnerOption) (*Store, *Runner) {
	t.Helper()
	store := NewStoreAt(t.TempDir())
	require.NoError(t, store.SaveConfig(Config{Prompt: DefaultPrompt, Targets: targets}))
	runner := newRunner(store, completer, append([]RunnerOption{
		WithClock(func() time.Time { return time.Date(2026, 9, 23, 7, 0, 0, 0, time.UTC) }),
	}, options...)...)
	return store, runner
}

func TestRunner_RecordsTheChannelEachTargetWasPinnedTo(t *testing.T) {
	completer := &fakeCompleter{handle: func(_ context.Context, target Target) (CompletionResult, error) {
		return CompletionResult{
			Reply:       "<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>",
			ChannelName: "hifox",
		}, nil
	}}
	store, runner := newTestRunner(t, []Target{{Channel: 1, Model: "gpt-6-astra"}, {Model: "unpinned"}}, completer)

	results, err := runner.Run(t.Context())
	require.NoError(t, err)

	byModel := map[string]Result{}
	for _, result := range results {
		byModel[result.Model] = result
	}
	// The gallery has to say which channel was compared, otherwise two rows of the same model
	// are indistinguishable.
	require.Equal(t, 1, byModel["gpt-6-astra"].Channel)
	require.Equal(t, "hifox", byModel["gpt-6-astra"].ChannelName)
	require.Equal(t, 0, byModel["unpinned"].Channel, "a target without a channel stays unpinned")

	// The pinned channel reaches the gateway, which is what makes the round reproducible.
	calls, _ := completer.snapshot()
	require.Contains(t, calls, Target{Channel: 1, Model: "gpt-6-astra", Effort: EffortAuto})

	// It is persisted with the result, so the UI still shows it after a restart.
	_, persisted, err := NewStoreAt(store.Dir()).Load()
	require.NoError(t, err)
	for _, result := range persisted {
		if result.Model == "gpt-6-astra" {
			require.Equal(t, "hifox", result.ChannelName)
		}
	}
}

func TestRunner_RunsEveryTargetAtItsOwnEffort(t *testing.T) {
	completer := &fakeCompleter{}
	store, runner := newTestRunner(t, []Target{{Model: "plain"}, {Model: "solver", Effort: EffortXHigh}}, completer)

	results, err := runner.Run(t.Context())
	require.NoError(t, err)
	require.Len(t, results, 2)
	for _, result := range results {
		require.Equal(t, StatusSucceeded, result.Status, result.Error)
		require.Equal(t, "svg", result.Format)
	}

	calls, _ := completer.snapshot()
	require.ElementsMatch(t, []Target{{Model: "plain", Effort: EffortAuto}, {Model: "solver", Effort: EffortXHigh}}, calls)

	// The artifact holds only the document; the conversation keeps the whole exchange.
	content, err := store.ReadArtifact(results[0].ID, "svg")
	require.NoError(t, err)
	require.Equal(t, "<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>", content)
	conversation, err := store.ReadConversation(results[0].ID)
	require.NoError(t, err)
	require.Equal(t, DefaultPrompt, conversation.Prompt, "the prompt used must be recorded")
	require.Equal(t, "stop", conversation.FinishReason)

	// Results are persisted, so the UI can read them after a restart.
	_, persisted, err := NewStoreAt(store.Dir()).Load()
	require.NoError(t, err)
	require.Len(t, persisted, 2)
}

func TestRunner_DispatchesEveryTargetAtOnce(t *testing.T) {
	completer := &fakeCompleter{handle: func(_ context.Context, _ Target) (CompletionResult, error) {
		time.Sleep(30 * time.Millisecond)
		return CompletionResult{Reply: "<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>"}, nil
	}}
	targets := []Target{{Model: "a"}, {Model: "b"}, {Model: "c"}, {Model: "d"}, {Model: "e"}}
	_, runner := newTestRunner(t, targets, completer)

	results, err := runner.Run(t.Context())
	require.NoError(t, err)
	require.Len(t, results, 5)

	_, maxActive := completer.snapshot()
	require.Equal(t, 5, maxActive, "a round dispatches every model at once")
}

func TestRunner_HonoursAnExplicitConcurrencyLimit(t *testing.T) {
	completer := &fakeCompleter{handle: func(_ context.Context, _ Target) (CompletionResult, error) {
		time.Sleep(20 * time.Millisecond)
		return CompletionResult{Reply: "<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>"}, nil
	}}
	targets := []Target{{Model: "a"}, {Model: "b"}, {Model: "c"}, {Model: "d"}, {Model: "e"}}
	_, runner := newTestRunner(t, targets, completer, WithConcurrency(2))

	_, err := runner.Run(t.Context())
	require.NoError(t, err)

	_, maxActive := completer.snapshot()
	require.LessOrEqual(t, maxActive, 2, "an explicit limit is still respected")
}

func TestRunner_KeepsGoingWhenOneModelFails(t *testing.T) {
	completer := &fakeCompleter{handle: func(_ context.Context, target Target) (CompletionResult, error) {
		if target.Model == "broken" {
			return CompletionResult{}, errors.New("upstream returned HTTP 422: Invalid value for reasoning_effort: ultra")
		}
		return CompletionResult{Reply: "<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>"}, nil
	}}
	store, runner := newTestRunner(t, []Target{{Model: "broken"}, {Model: "fine"}}, completer)

	results, err := runner.Run(t.Context())
	require.NoError(t, err)

	byModel := map[string]Result{}
	for _, result := range results {
		byModel[result.Model] = result
	}
	require.Equal(t, StatusFailed, byModel["broken"].Status)
	require.Contains(t, byModel["broken"].Error, "HTTP 422")
	require.Equal(t, StatusSucceeded, byModel["fine"].Status)

	// A failed attempt is inspectable too: the prompt is recorded together with the reason.
	conversation, err := store.ReadConversation(byModel["broken"].ID)
	require.NoError(t, err)
	require.Equal(t, DefaultPrompt, conversation.Prompt)
	require.Empty(t, conversation.Reply)
	require.Contains(t, conversation.Error, "HTTP 422")
}

func TestRunner_ReportsTimeoutsWithTheConfiguredBudget(t *testing.T) {
	completer := &fakeCompleter{handle: func(ctx context.Context, _ Target) (CompletionResult, error) {
		<-ctx.Done()
		return CompletionResult{}, ctx.Err()
	}}
	_, runner := newTestRunner(t, []Target{{Model: "slow"}}, completer, WithTimeout(30*time.Millisecond))

	results, err := runner.Run(t.Context())
	require.NoError(t, err)

	require.Equal(t, StatusFailed, results[0].Status)
	require.Contains(t, results[0].Error, "timed out after 30ms")
}

func TestRunner_RunsRoundsInParallel(t *testing.T) {
	release := make(chan struct{})
	completer := &fakeCompleter{handle: func(ctx context.Context, _ Target) (CompletionResult, error) {
		select {
		case <-release:
		case <-ctx.Done():
		}
		return CompletionResult{Reply: "<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>"}, nil
	}}
	_, runner := newTestRunner(t, []Target{{Model: "only"}}, completer)

	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		_, err := runner.Run(t.Context())
		require.NoError(t, err)
	}()

	require.Eventually(t, runner.IsRunning, time.Second, 5*time.Millisecond, "the round must report itself as running")

	// A second round starts immediately instead of being refused.
	secondDone := make(chan struct{})
	go func() {
		defer close(secondDone)
		_, err := runner.Run(t.Context())
		require.NoError(t, err)
	}()
	require.Eventually(t, func() bool {
		calls, _ := completer.snapshot()
		return len(calls) >= 2
	}, time.Second, 5*time.Millisecond, "both rounds must call the model")

	close(release)
	<-firstDone
	<-secondDone
	require.Eventually(t, func() bool { return !runner.IsRunning() }, time.Second, 5*time.Millisecond)
}

func TestRunner_RequiresAtLeastOneTarget(t *testing.T) {
	store := NewStoreAt(t.TempDir())
	runner := newRunner(store, &fakeCompleter{})

	_, err := runner.Run(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "no model is selected")
}

func TestExtractDocument(t *testing.T) {
	const html = "<!DOCTYPE html>\n<html><body><svg><circle r=\"1\" /></svg></body></html>"
	const svg = "<svg xmlns=\"http://www.w3.org/2000/svg\"><circle r=\"2\" /></svg>"
	const canvasHTML = "<!DOCTYPE html>\n<html><body><canvas id=\"c\"></canvas><script>draw()</script></body></html>"

	cases := []struct {
		name    string
		reply   string
		content string
		format  string
		ok      bool
	}{
		{name: "html in a fence", reply: "```html\n" + html + "\n```", content: html, format: "html", ok: true},
		{name: "html with surrounding prose", reply: "好的，这是代码：\n" + html + "\n希望有帮助。", content: html, format: "html", ok: true},
		{name: "prose before a fence", reply: "这是可直接运行的文件。\n\n```html\n" + html + "\n```\n\n**说明**：可调整速度。", content: html, format: "html", ok: true},
		{name: "standalone svg", reply: "这是图形：\n" + svg + "\n（可保存为 .svg）", content: svg, format: "svg", ok: true},
		// A drawing can be a canvas, WebGL or CSS scene; requiring svg rejected valid answers.
		{
			name:    "canvas instead of svg",
			reply:   "这是动画版：\n```html\n" + canvasHTML + "\n```\n祝好。",
			content: canvasHTML,
			format:  "html",
			ok:      true,
		},
		{name: "html fragment is not a document", reply: "<div>no document</div>", ok: false},
		{name: "prose only", reply: "抱歉，我无法生成这样的图像。", ok: false},
		{name: "empty reply", reply: "   ", ok: false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			content, format, ok := ExtractDocument(testCase.reply)
			require.Equal(t, testCase.ok, ok)
			if testCase.ok {
				require.Equal(t, testCase.content, content)
				require.Equal(t, testCase.format, format)
			}
		})
	}
}

func TestRedactCredentials(t *testing.T) {
	redacted := RedactCredentials("Invalid API key provided: sk-test-placeholder-not-a-real-key and Bearer abcdefghijklmnop")
	require.NotContains(t, redacted, "sk-test-placeholder-not-a-real-key")
	require.NotContains(t, redacted, "abcdefghijklmnop")
	require.Contains(t, redacted, "Invalid API key provided")
}

func TestRunner_KeepsTheChannelOnAFailedAttempt(t *testing.T) {
	completer := &fakeCompleter{handle: func(_ context.Context, _ Target) (CompletionResult, error) {
		return CompletionResult{ChannelName: "openrouter"}, errTestFailure
	}}
	_, runner := newTestRunner(t, []Target{{Channel: 6, Model: "deepseek-flash"}}, completer)

	results, err := runner.Run(t.Context())
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, StatusFailed, results[0].Status)
	require.Equal(t, 6, results[0].Channel)
	// A rejected attempt still says which channel it was pinned to.
	require.Equal(t, "openrouter", results[0].ChannelName)
}

func TestRunner_KeepsResultsFromEarlierRounds(t *testing.T) {
	completer := &fakeCompleter{}
	store, runner := newTestRunner(t, []Target{{Channel: 1, Model: "first"}}, completer)
	require.NoError(t, store.SaveConfig(Config{Prompt: DefaultPrompt, Targets: []Target{{Channel: 1, Model: "first"}}}))

	_, err := runner.Run(t.Context())
	require.NoError(t, err)

	// A later round compares another channel; the gallery must keep accumulating.
	require.NoError(t, store.SaveConfig(Config{Prompt: DefaultPrompt, Targets: []Target{{Channel: 4, Model: "second"}}}))
	_, err = runner.Run(t.Context())
	require.NoError(t, err)

	_, history, err := store.Load()
	require.NoError(t, err)
	require.Len(t, history, 2, "starting a round must not wipe earlier results")
	require.ElementsMatch(t, []string{"first", "second"}, []string{history[0].Model, history[1].Model})
}
