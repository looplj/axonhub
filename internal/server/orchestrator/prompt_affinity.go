// 职责: 提示词前缀软亲和缓存：把成功请求的提示词身份映射到上次成功的渠道，供候选选择优先回粘。
// 关联文件:
//
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
//	缓存只保留 SHA-256 摘要派生的 key，原始 prompt_cache_key 与提示词内容不得进入缓存、日志或数据库。
//	只在请求成功终态学习；失败、取消、处理中的请求不得改写映射。
//	get 的读-判-删必须与 set 同锁：否则一个读到过期条目的 goroutine 会删掉另一个刚刷新的映射。
package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm"
)

const (
	// promptAffinityCacheCapacity bounds the process-wide affinity cache. Each
	// entry is a small struct, so 4096 entries cost well under a megabyte.
	promptAffinityCacheCapacity = 4096
	// promptAffinityCacheTTL matches the previous-channel cache TTL so all
	// routing affinities expire together.
	promptAffinityCacheTTL = 30 * time.Minute

	// promptAffinitySegmentMaxBytes caps each prompt segment feeding the
	// affinity hash. Requests sharing a stable prefix beyond the cap still map
	// to the same identity, which is the intended approximation.
	promptAffinitySegmentMaxBytes = 8 * 1024

	// promptAffinitySourceCacheKey marks an affinity identity derived from the
	// client-provided prompt_cache_key.
	promptAffinitySourceCacheKey = "prompt_cache_key"
	// promptAffinitySourcePrefix marks an affinity identity derived from the
	// stable prompt prefix.
	promptAffinitySourcePrefix = "prompt_prefix"
)

// promptAffinityEntry is one learned prompt identity to channel mapping.
type promptAffinityEntry struct {
	channelID int
	expiresAt time.Time
}

// promptAffinityCache is a process-wide, goroutine-safe LRU with per-entry
// TTL. It never persists; a restart simply relearns affinities.
type promptAffinityCache struct {
	// mu serializes lookups, lazy expiry deletes, and refreshes, so a stale
	// entry observed by one goroutine cannot remove a mapping that a
	// concurrent set just renewed.
	mu  sync.Mutex
	lru *lru.Cache[string, promptAffinityEntry]
	ttl time.Duration
	now func() time.Time
}

func newPromptAffinityCache(capacity int, ttl time.Duration) *promptAffinityCache {
	if capacity <= 0 {
		capacity = promptAffinityCacheCapacity
	}

	if ttl <= 0 {
		ttl = promptAffinityCacheTTL
	}

	return &promptAffinityCache{
		lru: lo.Must(lru.New[string, promptAffinityEntry](capacity)),
		ttl: ttl,
		now: time.Now,
	}
}

// get returns the channel learned for key, expiring stale entries lazily.
func (c *promptAffinityCache) get(key string) (int, bool) {
	if c == nil {
		return 0, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.lru.Get(key)
	if !ok {
		return 0, false
	}

	if c.now().After(entry.expiresAt) {
		c.lru.Remove(key)
		return 0, false
	}

	return entry.channelID, true
}

// set learns that the request identity key succeeded on channelID.
func (c *promptAffinityCache) set(key string, channelID int) {
	if c == nil || key == "" || channelID == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.lru.Add(key, promptAffinityEntry{
		channelID: channelID,
		expiresAt: c.now().Add(c.ttl),
	})
}

// computePromptAffinityKey builds the routing identity for a request from the
// API key, API format, model, request type, and a prompt signal. The signal is
// the client-provided prompt cache key when present, otherwise the stable
// prompt prefix; the two namespaces never collide. Only the SHA-256 digest of
// the signal is retained, so raw prompts and keys never enter the cache.
func computePromptAffinityKey(apiKeyID int, req *llm.Request) (key, source string, ok bool) {
	if req == nil {
		return "", "", false
	}

	var (
		namespace string
		digestHex string
	)

	if explicit := strings.TrimSpace(lo.FromPtr(req.PromptCacheKey)); explicit != "" {
		namespace = "pck"
		source = promptAffinitySourceCacheKey
		sum := sha256.Sum256([]byte(explicit))
		digestHex = hex.EncodeToString(sum[:])
	} else {
		segments := promptPrefixSegments(req.Messages)
		if len(segments) == 0 {
			return "", "", false
		}

		h := sha256.New()
		for _, segment := range segments {
			writeHashPart(h, segment.role)
			writeHashPart(h, segment.text)
		}

		namespace = "pfx"
		source = promptAffinitySourcePrefix
		digestHex = hex.EncodeToString(h.Sum(nil))
	}

	key = fmt.Sprintf(
		"axonhub:routing:prompt-affinity:v1:%d:%s:%s:%s:%s:%s",
		apiKeyID,
		string(req.APIFormat),
		req.Model,
		string(req.RequestType),
		namespace,
		digestHex,
	)

	return key, source, true
}

// promptPrefixSegment is one stable prompt segment feeding the affinity hash.
type promptPrefixSegment struct {
	role string
	text string
}

// promptPrefixSegments collects the stable prefix of a chat request: every
// system/developer message plus the first user message. Responses API requests
// reach the selector already converted to this shape (instructions become a
// developer message, the first input item becomes the first user message).
func promptPrefixSegments(messages []llm.Message) []promptPrefixSegment {
	segments := make([]promptPrefixSegment, 0, 4)
	userSeen := false

	for i := range messages {
		message := &messages[i]

		switch message.Role {
		case "system", "developer":
			if text := truncateAffinitySegment(messageText(message)); text != "" {
				segments = append(segments, promptPrefixSegment{role: message.Role, text: text})
			}
		case "user":
			if userSeen {
				continue
			}

			userSeen = true

			if text := truncateAffinitySegment(messageText(message)); text != "" {
				segments = append(segments, promptPrefixSegment{role: message.Role, text: text})
			}
		}
	}

	return segments
}

// messageText flattens a message to plain text, ignoring non-text parts.
func messageText(message *llm.Message) string {
	if message.Content.Content != nil {
		return *message.Content.Content
	}

	var b strings.Builder
	for _, part := range message.Content.MultipleContent {
		if part.Type == "text" && part.Text != nil {
			b.WriteString(*part.Text)
		}
	}

	return b.String()
}

func truncateAffinitySegment(text string) string {
	if len(text) > promptAffinitySegmentMaxBytes {
		return text[:promptAffinitySegmentMaxBytes]
	}

	return text
}

// writeHashPart writes a length-prefixed part so segment boundaries are
// unambiguous inside the digest.
func writeHashPart(h hash.Hash, part string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(part)))
	_, _ = h.Write(length[:])
	_, _ = h.Write([]byte(part))
}

// learnPromptAffinity records the channel that served a successful request.
// Only the request-completed path calls it; failure and cancel paths never
// reach it, so unsuccessful requests never rewrite the mapping.
func learnPromptAffinity(ctx context.Context, state *PersistenceState) {
	if state == nil || state.PromptAffinityCache == nil || state.promptAffinityKey == "" || state.Request == nil {
		return
	}

	channelID := state.Request.ChannelID
	if channelID == 0 {
		return
	}

	state.PromptAffinityCache.set(state.promptAffinityKey, channelID)

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "learned prompt affinity channel",
			log.String("source", state.promptAffinitySource),
			log.Int("channel_id", channelID),
		)
	}
}
