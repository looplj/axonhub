package orchestrator

import (
	"context"
	"encoding/base64"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

const promptProtectionRejectedMessage = "request blocked by prompt protection policy"

// Unique tag constants for prompt protection restoration.
// Each matched text is replaced with a unique tag that encodes the original content,
// enabling precise restoration even when multiple distinct originals map to the same
// fixed placeholder (e.g. a regex like "荣昌|开州|梁平" all replaced with "[REDACTED]").
//
// Tag format: «axh:replacement:uuid:base64url_original»
// Example: «axh:[REDACTED]:a1b2c3d4-e5f6-7890-abcd-ef1234567890:6I2j5p2l5YWz»
//
// The replacement is the user-configured placeholder (visible to AI for context).
// The UUID guarantees uniqueness across all matches.
// The base64url-encoded original allows deterministic restoration without an external mapping dict.
const (
	axhTagPrefix = "\u00abaxh:" // «
	axhTagSuffix = "\u00bb"     // »
)

// genMaskedTagWithReplacement generates a unique replacement tag that includes the user-configured
// replacement text, so AI models can see meaningful placeholders (e.g. [REDACTED]) while still
// allowing precise per-match restoration via UUID and base64-encoded original.
func genMaskedTagWithReplacement(original, replacement string) string {
	id := uuid.New().String()
	encoded := base64.RawURLEncoding.EncodeToString([]byte(original))
	return axhTagPrefix + replacement + ":" + id + ":" + encoded + axhTagSuffix
}

// containsMaskedContent checks if any message contains unique masked tags.
func containsMaskedContent(messages []llm.Message) bool {
	for _, msg := range messages {
		if msg.Content.Content != nil && strings.Contains(*msg.Content.Content, axhTagPrefix) {
			return true
		}

		for _, part := range msg.Content.MultipleContent {
			if part.Text != nil && strings.Contains(*part.Text, axhTagPrefix) {
				return true
			}
		}
	}

	return false
}

// restoreMaskedContent replaces all unique tags in text with their original content.
func restoreMaskedContent(text string) string {
	if !strings.Contains(text, axhTagPrefix) {
		return text
	}

	var (
		result strings.Builder
		rest   = text
	)

	for {
		start := strings.Index(rest, axhTagPrefix)
		if start == -1 {
			result.WriteString(rest)
			break
		}

		result.WriteString(rest[:start])

		end := strings.Index(rest[start:], axhTagSuffix)
		if end == -1 {
			// Malformed tag without closing bracket — keep as-is.
			result.WriteString(rest[start:])
			break
		}

		tag := rest[start : start+end+1]
		original, ok := parseMaskedTag(tag)
		if ok {
			result.WriteString(original)
		} else {
			result.WriteString(tag)
		}

		rest = rest[start+end+1:]
	}

	return result.String()
}

// parseMaskedTag extracts the original text from a unique tag.
// Supports both formats:
//   - New: «axh:replacement:uuid:base64_original» (3 colons)
//   - Old: «axh:uuid:base64_original» (2 colons, backward compatibility)
func parseMaskedTag(tag string) (string, bool) {
	inner := tag[len(axhTagPrefix) : len(tag)-len(axhTagSuffix)]

	// Find the last colon — separates the base64-encoded original.
	lastColon := strings.LastIndex(inner, ":")
	if lastColon <= 0 {
		return "", false
	}

	// Find the second-to-last colon — separates the UUID.
	secondLastColon := strings.LastIndex(inner[:lastColon], ":")
	if secondLastColon <= 0 {
		// Old format: «axh:uuid:base64»
		decoded, err := base64.RawURLEncoding.DecodeString(inner[lastColon+1:])
		if err != nil {
			return "", false
		}

		return string(decoded), true
	}

	// New format: «axh:replacement:uuid:base64»
	// Validate UUID segment is 36 characters.
	uuidSegment := inner[secondLastColon+1 : lastColon]
	if len(uuidSegment) != 36 {
		// Not a valid UUID — treat as old format.
		decoded, err := base64.RawURLEncoding.DecodeString(inner[lastColon+1:])
		if err != nil {
			return "", false
		}

		return string(decoded), true
	}

	decoded, err := base64.RawURLEncoding.DecodeString(inner[lastColon+1:])
	if err != nil {
		return "", false
	}

	return string(decoded), true
}

// MaskMessageWithUniqueTags applies prompt protection rules to messages, replacing each
// regex match with a unique tag that embeds the original text. This ensures that
// distinct matches (e.g. different location names matched by "荣昌|开州|梁平")
// can be individually restored, unlike fixed-placeholder replacement where all
// matches merge into the same string.
func MaskMessageWithUniqueTags(messages []llm.Message, rules []*ent.PromptProtectionRule) {
	for _, rule := range rules {
		if rule == nil || rule.Settings == nil {
			continue
		}

		if rule.Settings.Action != objects.PromptProtectionActionMask {
			continue
		}

		for i := range messages {
			if !promptProtectionRuleAppliesToRole(rule.Settings.Scopes, messages[i].Role) {
				continue
			}

			if messages[i].Content.Content != nil && *messages[i].Content.Content != "" {
				content := *messages[i].Content.Content
				if biz.MatchPromptProtectionRule(rule.Pattern, content) {
					replacement := rule.Settings.Replacement
					masked := biz.ReplaceWithUniqueTags(rule.Pattern, content, func(original string) string {
						return genMaskedTagWithReplacement(original, replacement)
					})
					messages[i].Content = llm.MessageContent{Content: &masked}
				}
			}

			for j := range messages[i].Content.MultipleContent {
				part := &messages[i].Content.MultipleContent[j]
				if !strings.EqualFold(part.Type, "text") || part.Text == nil || *part.Text == "" {
					continue
				}

				text := *part.Text
				if biz.MatchPromptProtectionRule(rule.Pattern, text) {
					replacement := rule.Settings.Replacement
					masked := biz.ReplaceWithUniqueTags(rule.Pattern, text, func(original string) string {
						return genMaskedTagWithReplacement(original, replacement)
					})
					part.Text = &masked
				}
			}
		}
	}
}

// promptProtectionRuleAppliesToRole checks whether the rule scopes include the message role.
func promptProtectionRuleAppliesToRole(scopes []objects.PromptProtectionScope, role string) bool {
	if len(scopes) == 0 {
		return true
	}

	return slices.Contains(scopes, objects.PromptProtectionScope(strings.ToLower(role)))
}

// listEnabledRuleProvider is an optional interface that PromptProtecter implementations
// may satisfy to expose the raw rules list directly, enabling the orchestrator to apply
// per-match unique tags instead of the standard fixed-replacement masking.
type listEnabledRuleProvider interface {
	ListEnabledRules(ctx context.Context) ([]*ent.PromptProtectionRule, error)
}

// protectPrompts creates the request-phase middleware that masks sensitive content with unique tags.
//
// Flow:
//  1. Load enabled rules via ListEnabledRules (falls back to standard Protect if unavailable).
//  2. First pass: call ApplyPromptProtectionRules to check for reject rules.
//  3. Second pass: re-apply mask rules with unique per-match tags for later restoration.
//  4. Returns the request with masked messages (unique tags); downstream pipeline and AI
//     see the masked content, while responses can be precisely restored.
func protectPrompts(inbound *PersistentInboundTransformer) pipeline.Middleware {
	return pipeline.OnLlmRequest("protect-prompts", func(ctx context.Context, llmRequest *llm.Request) (retReq *llm.Request, retErr error) {
		// Recover from any panic in masking logic — never block the main pipeline.
		defer func() {
			if r := recover(); r != nil {
				log.Warn(ctx, "prompt protection masking panicked, passing through original request", log.Any("panic", r))
				retReq = llmRequest
				retErr = nil
			}
		}()

		if inbound.state.PromptProtecter == nil {
			return llmRequest, nil
		}

		// Try to get raw rules for unique-tag masking.
		provider, canList := inbound.state.PromptProtecter.(listEnabledRuleProvider)
		if !canList {
			// Fallback: use standard Protect (fixed replacement, no per-match uniqueness).
			return fallbackProtect(inbound.state.PromptProtecter, llmRequest)
		}

		rules, err := provider.ListEnabledRules(ctx)
		if err != nil {
			log.Warn(ctx, "failed to load enabled prompt protection rules, passing through", log.Cause(err))
			return llmRequest, nil
		}

		if len(rules) == 0 {
			return llmRequest, nil
		}

		// First pass: rejection check using standard replacement (all matches → same placeholder).
		rejectionCheck := biz.ApplyPromptProtectionRules(llmRequest, rules)
		if rejectionCheck.Rejected {
			return nil, fmt.Errorf("%w: %s", transformer.ErrInvalidRequest, promptProtectionRejectedMessage)
		}

		// Second pass: apply mask rules with unique per-match tags.
		MaskMessageWithUniqueTags(llmRequest.Messages, rules)

		return llmRequest, nil
	})
}

// fallbackProtect handles the case where PromptProtecter doesn't expose ListEnabledRules.
func fallbackProtect(protecter PromptProtecter, llmRequest *llm.Request) (*llm.Request, error) {
	protected, err := protecter.Protect(context.Background(), llmRequest)
	if err != nil {
		return llmRequest, err
	}

	if protected == nil {
		return llmRequest, nil
	}

	return protected, nil
}

// restorePromptProtection creates the response-phase middleware that restores unique tags
// to their original text before the response reaches the client.
func restorePromptProtection(inbound *PersistentInboundTransformer) pipeline.Middleware {
	return &promptProtectionRestoreMiddleware{
		DummyMiddleware: pipeline.DummyMiddleware{},
		inbound:         inbound,
	}
}

type promptProtectionRestoreMiddleware struct {
	pipeline.DummyMiddleware
	inbound *PersistentInboundTransformer
}

func (m *promptProtectionRestoreMiddleware) Name() string {
	return "restore-prompt-protection"
}

func (m *promptProtectionRestoreMiddleware) OnInboundRawResponse(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error) {
	if len(response.Body) > 0 {
		body := string(response.Body)
		if strings.Contains(body, axhTagPrefix) {
			defer func() {
				if r := recover(); r != nil {
					log.Warn(ctx, "prompt protection response restore panicked, returning original response", log.Any("panic", r))
				}
			}()

			restored := restoreMaskedContent(body)
			if restored != body {
				response.Body = []byte(restored)
			}
		}
	}

	return response, nil
}

func (m *promptProtectionRestoreMiddleware) OnInboundRawStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error) {
	return streams.Map(stream, func(event *httpclient.StreamEvent) *httpclient.StreamEvent {
		if event != nil && len(event.Data) > 0 {
			data := string(event.Data)
			if strings.Contains(data, axhTagPrefix) {
				event.Data = []byte(restoreMaskedContent(data))
			}
		}

		return event
	}), nil
}
