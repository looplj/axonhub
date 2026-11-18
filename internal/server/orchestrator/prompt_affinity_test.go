// 职责: 覆盖提示词前缀亲和的 key 派生、进程级缓存语义与选择器/成功学习路径。
// 关联文件:
//
//	internal/server/orchestrator/prompt_affinity.go
//	internal/server/orchestrator/candidates.go
//	internal/server/orchestrator/select_candidates.go
//	internal/server/orchestrator/events.go
//	internal/server/orchestrator/state.go
//
// 遵循的 Rules:
//
//	.agents/rules/go-general.md
//	.agents/rules/cache-compat.md
//
// 注意事项:
//
//	缓存只保留摘要派生 key；用例不得断言或记录原始提示词，学习只覆盖成功终态。
package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

func affinityChatRequest(model, system, user string) *llm.Request {
	messages := make([]llm.Message, 0, 2)
	if system != "" {
		messages = append(messages, llm.Message{Role: "system", Content: llm.MessageContent{Content: lo.ToPtr(system)}})
	}
	if user != "" {
		messages = append(messages, llm.Message{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr(user)}})
	}

	return &llm.Request{
		Model:       model,
		Messages:    messages,
		APIFormat:   llm.APIFormatOpenAIChatCompletion,
		RequestType: llm.RequestTypeChat,
	}
}

func mustAffinityKey(t *testing.T, apiKeyID int, req *llm.Request) string {
	t.Helper()

	key, _, ok := computePromptAffinityKey(apiKeyID, req)
	require.True(t, ok)

	return key
}

func TestComputePromptAffinityKey(t *testing.T) {
	t.Run("same prefix derives the same key", func(t *testing.T) {
		first := mustAffinityKey(t, 1, affinityChatRequest("model-a", "sys", "user-1"))
		second := mustAffinityKey(t, 1, affinityChatRequest("model-a", "sys", "user-1"))
		require.Equal(t, first, second)

		_, source, _ := computePromptAffinityKey(1, affinityChatRequest("model-a", "sys", "user-1"))
		require.Equal(t, promptAffinitySourcePrefix, source)
	})

	t.Run("different first user message derives a different key", func(t *testing.T) {
		first := mustAffinityKey(t, 1, affinityChatRequest("model-a", "sys", "user-1"))
		second := mustAffinityKey(t, 1, affinityChatRequest("model-a", "sys", "user-2"))
		require.NotEqual(t, first, second)
	})

	t.Run("later turns do not change the prefix identity", func(t *testing.T) {
		base := affinityChatRequest("model-a", "sys", "user-1")
		followUp := affinityChatRequest("model-a", "sys", "user-1")
		followUp.Messages = append(followUp.Messages,
			llm.Message{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("answer")}},
			llm.Message{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("user-2")}},
		)
		require.Equal(t, mustAffinityKey(t, 1, base), mustAffinityKey(t, 1, followUp))
	})

	t.Run("api key and model isolate the key space", func(t *testing.T) {
		req := affinityChatRequest("model-a", "sys", "user-1")
		base := mustAffinityKey(t, 1, req)
		require.NotEqual(t, base, mustAffinityKey(t, 2, req))
		require.NotEqual(t, base, mustAffinityKey(t, 1, affinityChatRequest("model-b", "sys", "user-1")))
	})

	t.Run("explicit prompt cache key uses its own namespace and wins over prefix", func(t *testing.T) {
		withKey := affinityChatRequest("model-a", "sys", "user-1")
		withKey.PromptCacheKey = lo.ToPtr("client-cache-key")

		key, source, ok := computePromptAffinityKey(1, withKey)
		require.True(t, ok)
		require.Equal(t, promptAffinitySourceCacheKey, source)
		require.Contains(t, key, ":pck:")
		require.NotContains(t, key, "client-cache-key")

		// Same cache key with different messages still maps to one identity.
		other := affinityChatRequest("model-a", "other-sys", "user-9")
		other.PromptCacheKey = lo.ToPtr("client-cache-key")
		require.Equal(t, key, mustAffinityKey(t, 1, other))

		// The derived prefix identity lives in a different namespace.
		derived := mustAffinityKey(t, 1, affinityChatRequest("model-a", "sys", "user-1"))
		require.Contains(t, derived, ":pfx:")
		require.NotEqual(t, key, derived)
	})

	t.Run("developer role contributes and text parts flatten", func(t *testing.T) {
		developer := affinityChatRequest("model-a", "", "user-1")
		developer.Messages = append([]llm.Message{
			{Role: "developer", Content: llm.MessageContent{Content: lo.ToPtr("instructions")}},
		}, developer.Messages...)

		flattened := affinityChatRequest("model-a", "", "user-1")
		flattened.Messages = append([]llm.Message{
			{Role: "developer", Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{
				{Type: "text", Text: lo.ToPtr("instr")},
				{Type: "text", Text: lo.ToPtr("uctions")},
			}}},
		}, flattened.Messages...)

		require.Equal(t, mustAffinityKey(t, 1, developer), mustAffinityKey(t, 1, flattened))
	})

	t.Run("segments are truncated at the cap", func(t *testing.T) {
		prefix := strings.Repeat("a", promptAffinitySegmentMaxBytes)
		first := mustAffinityKey(t, 1, affinityChatRequest("model-a", prefix+"-tail-1", "user-1"))
		second := mustAffinityKey(t, 1, affinityChatRequest("model-a", prefix+"-tail-2", "user-1"))
		require.Equal(t, first, second)
	})

	t.Run("requests without a stable prefix have no affinity", func(t *testing.T) {
		_, _, ok := computePromptAffinityKey(1, &llm.Request{Model: "model-a"})
		require.False(t, ok)

		_, _, ok = computePromptAffinityKey(1, &llm.Request{
			Model:    "model-a",
			Messages: []llm.Message{{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("hi")}}},
		})
		require.False(t, ok)

		_, _, ok = computePromptAffinityKey(1, nil)
		require.False(t, ok)
	})
}

func TestPromptAffinityCache(t *testing.T) {
	t.Run("set and get", func(t *testing.T) {
		cache := newPromptAffinityCache(0, 0)
		cache.set("key", 42)

		channelID, ok := cache.get("key")
		require.True(t, ok)
		require.Equal(t, 42, channelID)

		_, ok = cache.get("missing")
		require.False(t, ok)
	})

	t.Run("entries expire after the TTL", func(t *testing.T) {
		cache := newPromptAffinityCache(0, time.Minute)
		now := time.Now()
		cache.now = func() time.Time { return now }

		cache.set("key", 7)
		_, ok := cache.get("key")
		require.True(t, ok)

		now = now.Add(2 * time.Minute)
		_, ok = cache.get("key")
		require.False(t, ok)
	})

	t.Run("an expired entry is revived by a refresh", func(t *testing.T) {
		cache := newPromptAffinityCache(0, time.Minute)
		now := time.Now()
		cache.now = func() time.Time { return now }

		cache.set("key", 7)

		// The entry is stale here. A refresh must win over the lazy expiry
		// delete, otherwise the mapping just learned is thrown away.
		now = now.Add(2 * time.Minute)
		cache.set("key", 9)

		channelID, ok := cache.get("key")
		require.True(t, ok)
		require.Equal(t, 9, channelID)
	})

	t.Run("least recently used entries are evicted beyond capacity", func(t *testing.T) {
		cache := newPromptAffinityCache(2, time.Hour)
		cache.set("a", 1)
		cache.set("b", 2)
		cache.set("c", 3)

		_, ok := cache.get("a")
		require.False(t, ok)

		for _, key := range []string{"b", "c"} {
			_, ok := cache.get(key)
			require.True(t, ok, key)
		}
	})

	t.Run("zero channel and empty key are not learned", func(t *testing.T) {
		cache := newPromptAffinityCache(0, 0)
		cache.set("key", 0)
		cache.set("", 1)
		require.Zero(t, cache.lru.Len())
	})

	t.Run("concurrent reads and writes are safe", func(t *testing.T) {
		cache := newPromptAffinityCache(128, time.Minute)

		var wg sync.WaitGroup
		for worker := range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range 500 {
					key := fmt.Sprintf("key-%d", (worker*500+i)%128)
					cache.set(key, worker+1)
					_, _ = cache.get(key)
				}
			}()
		}
		wg.Wait()
	})
}

func newAffinityTestSelector(
	candidates []*ChannelModelsCandidate,
	stickyMode biz.TraceStickyMode,
	provider PreviousChannelProvider,
	apiKey *ent.APIKey,
	cache *promptAffinityCache,
) *LoadBalancedSelector {
	return &LoadBalancedSelector{
		wrapped:                 &staticChannelSelector{candidates: candidates},
		loadBalancer:            NewLoadBalancer(nil, nil),
		retryPolicy:             &biz.RetryPolicy{Enabled: true, MaxChannelRetries: 2, TraceStickyMode: stickyMode},
		previousChannelProvider: provider,
		apiKey:                  apiKey,
		promptAffinity:          cache,
	}
}

func TestLoadBalancedSelector_PromptAffinity(t *testing.T) {
	req := affinityChatRequest("model-a", "sys", "user-1")

	t.Run("second request with the same prefix pins the learned channel", func(t *testing.T) {
		cache := newPromptAffinityCache(0, 0)
		cache.set(mustAffinityKey(t, 0, req), 2)

		candidates := []*ChannelModelsCandidate{stickyTestCandidate(1, 0), stickyTestCandidate(2, 0)}
		selector := newAffinityTestSelector(candidates, biz.TraceStickyPreferPreviousChannel, nil, nil, cache)

		result, err := selector.Select(context.Background(), req)
		require.NoError(t, err)
		require.Equal(t, 2, result[0].Channel.ID)
		require.True(t, result[0].TraceSticky)
		require.Equal(t, 1, result[1].Channel.ID)
	})

	t.Run("a rate limited affinity channel defers to load balancing", func(t *testing.T) {
		rpm := int64(1)
		tracker := NewChannelRequestTracker()
		strategy := NewRateLimitAwareStrategy(tracker, nil)
		require.True(t, tracker.TryAcquireRequest(2, rpm))

		cache := newPromptAffinityCache(0, 0)
		cache.set(mustAffinityKey(t, 0, req), 2)

		exhausted := stickyTestCandidate(2, 0)
		exhausted.Channel.Settings = &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{RPM: &rpm},
		}

		candidates := []*ChannelModelsCandidate{stickyTestCandidate(1, 0), exhausted}
		selector := newAffinityTestSelector(candidates, biz.TraceStickyPreferPreviousChannel, nil, nil, cache)
		selector.loadBalancer = NewLoadBalancer(nil, nil, strategy)
		selector.rateLimitStrategy = strategy

		result, err := selector.Select(context.Background(), req)
		require.NoError(t, err)
		require.Equal(t, 1, result[0].Channel.ID)
		require.False(t, result[0].TraceSticky)
		// The rate limited channel stays in the candidate set as a fallback.
		require.Len(t, result, 2)
		require.Equal(t, 2, result[1].Channel.ID)
	})

	t.Run("an available affinity channel still pins with the limit check enabled", func(t *testing.T) {
		strategy := NewRateLimitAwareStrategy(NewChannelRequestTracker(), nil)

		cache := newPromptAffinityCache(0, 0)
		cache.set(mustAffinityKey(t, 0, req), 2)

		candidates := []*ChannelModelsCandidate{stickyTestCandidate(1, 0), stickyTestCandidate(2, 0)}
		selector := newAffinityTestSelector(candidates, biz.TraceStickyPreferPreviousChannel, nil, nil, cache)
		selector.loadBalancer = NewLoadBalancer(nil, nil, strategy)
		selector.rateLimitStrategy = strategy

		result, err := selector.Select(context.Background(), req)
		require.NoError(t, err)
		require.Equal(t, 2, result[0].Channel.ID)
		require.True(t, result[0].TraceSticky)
	})

	t.Run("a new prefix falls back to normal load balancing", func(t *testing.T) {
		cache := newPromptAffinityCache(0, 0)
		cache.set(mustAffinityKey(t, 0, req), 2)

		candidates := []*ChannelModelsCandidate{stickyTestCandidate(1, 0), stickyTestCandidate(2, 0)}
		selector := newAffinityTestSelector(candidates, biz.TraceStickyPreferPreviousChannel, nil, nil, cache)

		result, err := selector.Select(context.Background(), affinityChatRequest("model-a", "sys", "user-2"))
		require.NoError(t, err)
		require.Equal(t, 1, result[0].Channel.ID)
		require.False(t, result[0].TraceSticky)
	})

	t.Run("affinity channel outside the candidate set is ignored", func(t *testing.T) {
		cache := newPromptAffinityCache(0, 0)
		cache.set(mustAffinityKey(t, 0, req), 99)

		candidates := []*ChannelModelsCandidate{stickyTestCandidate(1, 0), stickyTestCandidate(2, 0)}
		selector := newAffinityTestSelector(candidates, biz.TraceStickyPreferPreviousChannel, nil, nil, cache)

		result, err := selector.Select(context.Background(), req)
		require.NoError(t, err)
		require.Equal(t, 1, result[0].Channel.ID)
		require.False(t, result[0].TraceSticky)
	})

	t.Run("trace sticky wins over prompt affinity", func(t *testing.T) {
		cache := newPromptAffinityCache(0, 0)
		cache.set(mustAffinityKey(t, 0, req), 3)

		ctx := contexts.WithTrace(context.Background(), &ent.Trace{ID: 10})
		provider := &fakePreviousChannelProvider{traceChannelIDs: map[int]int{10: 2}}
		candidates := []*ChannelModelsCandidate{stickyTestCandidate(1, 0), stickyTestCandidate(2, 0), stickyTestCandidate(3, 0)}
		selector := newAffinityTestSelector(candidates, biz.TraceStickyPreferPreviousChannel, provider, nil, cache)

		result, err := selector.Select(ctx, req)
		require.NoError(t, err)
		require.Equal(t, 2, result[0].Channel.ID)
		require.True(t, result[0].TraceSticky)
	})

	t.Run("disabled sticky mode skips prompt affinity", func(t *testing.T) {
		cache := newPromptAffinityCache(0, 0)
		cache.set(mustAffinityKey(t, 0, req), 2)

		candidates := []*ChannelModelsCandidate{stickyTestCandidate(1, 0), stickyTestCandidate(2, 0)}
		selector := newAffinityTestSelector(candidates, biz.TraceStickyDisabled, nil, nil, cache)

		result, err := selector.Select(context.Background(), req)
		require.NoError(t, err)
		require.Equal(t, 1, result[0].Channel.ID)
		require.False(t, result[0].TraceSticky)
	})

	t.Run("affinities learned by another API key do not leak", func(t *testing.T) {
		cache := newPromptAffinityCache(0, 0)
		cache.set(mustAffinityKey(t, 7, req), 2)

		candidates := []*ChannelModelsCandidate{stickyTestCandidate(1, 0), stickyTestCandidate(2, 0)}
		selector := newAffinityTestSelector(candidates, biz.TraceStickyPreferPreviousChannel, nil, &ent.APIKey{ID: 8}, cache)

		result, err := selector.Select(context.Background(), req)
		require.NoError(t, err)
		require.Equal(t, 1, result[0].Channel.ID)
		require.False(t, result[0].TraceSticky)
	})

	t.Run("nil cache keeps the existing behavior", func(t *testing.T) {
		candidates := []*ChannelModelsCandidate{stickyTestCandidate(1, 0), stickyTestCandidate(2, 0)}
		selector := newAffinityTestSelector(candidates, biz.TraceStickyPreferPreviousChannel, nil, nil, nil)

		result, err := selector.Select(context.Background(), req)
		require.NoError(t, err)
		require.Equal(t, 1, result[0].Channel.ID)
		require.False(t, result[0].TraceSticky)
	})
}

func TestLearnPromptAffinity(t *testing.T) {
	t.Run("completed requests learn the serving channel", func(t *testing.T) {
		cache := newPromptAffinityCache(0, 0)
		key, source, ok := computePromptAffinityKey(0, affinityChatRequest("model-a", "sys", "user-1"))
		require.True(t, ok)

		state := &PersistenceState{
			PromptAffinityCache:  cache,
			Request:              &ent.Request{ChannelID: 2},
			promptAffinityKey:    key,
			promptAffinitySource: source,
		}
		publishRequestCompleted(context.Background(), state, nil, false, nil)

		channelID, hit := cache.get(key)
		require.True(t, hit)
		require.Equal(t, 2, channelID)
	})

	t.Run("requests without an affinity snapshot learn nothing", func(t *testing.T) {
		cache := newPromptAffinityCache(0, 0)
		state := &PersistenceState{
			PromptAffinityCache: cache,
			Request:             &ent.Request{ChannelID: 2},
		}
		learnPromptAffinity(context.Background(), state)
		require.Zero(t, cache.lru.Len())
	})

	t.Run("requests without a final channel learn nothing", func(t *testing.T) {
		cache := newPromptAffinityCache(0, 0)
		state := &PersistenceState{
			PromptAffinityCache: cache,
			Request:             &ent.Request{},
			promptAffinityKey:   "key",
		}
		learnPromptAffinity(context.Background(), state)
		require.Zero(t, cache.lru.Len())
	})
}
