package orchestrator

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

// ChannelSelector defines the interface for selecting channels.
type ChannelSelector interface {
	Select(ctx context.Context, req *llm.Request) ([]*biz.Channel, error)
}

// DefaultSelector directly calls ChannelService.ChooseChannels to get enabled channels.
type DefaultSelector struct {
	ChannelService *biz.ChannelService
}

// NewDefaultSelector creates a basic selector that returns all enabled channels supporting the model.
func NewDefaultSelector(channelService *biz.ChannelService) *DefaultSelector {
	return &DefaultSelector{
		ChannelService: channelService,
	}
}

func (s *DefaultSelector) Select(ctx context.Context, req *llm.Request) ([]*biz.Channel, error) {
	return s.ChannelService.ChooseChannels(ctx, req)
}

// SelectedChannelsSelector is a decorator that filters channels by allowed channel IDs.
type SelectedChannelsSelector struct {
	wrapped           ChannelSelector
	allowedChannelIDs []int
}

// NewSelectedChannelsSelector creates a selector that filters by allowed channel IDs.
// If allowedChannelIDs is nil or empty, all channels from the wrapped selector are returned.
func NewSelectedChannelsSelector(wrapped ChannelSelector, allowedChannelIDs []int) *SelectedChannelsSelector {
	return &SelectedChannelsSelector{
		wrapped:           wrapped,
		allowedChannelIDs: allowedChannelIDs,
	}
}

func (s *SelectedChannelsSelector) Select(ctx context.Context, req *llm.Request) ([]*biz.Channel, error) {
	channels, err := s.wrapped.Select(ctx, req)
	if err != nil {
		return nil, err
	}

	// If no allowed IDs specified, return all channels
	if len(s.allowedChannelIDs) == 0 {
		return channels, nil
	}

	// Build allowed set for O(1) lookup
	allowedSet := lo.SliceToMap(s.allowedChannelIDs, func(id int) (int, struct{}) {
		return id, struct{}{}
	})

	// Filter channels by allowed IDs
	filtered := lo.Filter(channels, func(ch *biz.Channel, _ int) bool {
		_, ok := allowedSet[ch.ID]
		return ok
	})

	return filtered, nil
}

// LoadBalancedSelector is a decorator that sorts channels using load balancing strategies.
type LoadBalancedSelector struct {
	wrapped      ChannelSelector
	loadBalancer *LoadBalancer
}

// NewLoadBalancedSelector creates a selector that applies load balancing to sort channels.
func NewLoadBalancedSelector(wrapped ChannelSelector, loadBalancer *LoadBalancer) *LoadBalancedSelector {
	return &LoadBalancedSelector{
		wrapped:      wrapped,
		loadBalancer: loadBalancer,
	}
}

func (s *LoadBalancedSelector) Select(ctx context.Context, req *llm.Request) ([]*biz.Channel, error) {
	channels, err := s.wrapped.Select(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(channels) <= 1 {
		return channels, nil
	}

	// Apply load balancing to sort channels by priority
	sortedChannels := s.loadBalancer.Sort(ctx, channels, req.Model)

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "Load balanced channels for model",
			log.String("model", req.Model),
			log.Int("total_channels", len(channels)),
			log.Int("selected_channels", len(sortedChannels)))
	}

	return sortedChannels, nil
}

// TagsFilterSelector 是一个装饰器，根据允许的标签过滤渠道。
// 采用 OR 逻辑：渠道包含任意一个允许的标签即可被选中。
type TagsFilterSelector struct {
	wrapped     ChannelSelector
	allowedTags []string
}

// NewTagsFilterSelector 创建根据标签过滤的选择器。
// 如果 allowedTags 为空，返回所有来自 wrapped selector 的渠道。
func NewTagsFilterSelector(wrapped ChannelSelector, allowedTags []string) *TagsFilterSelector {
	return &TagsFilterSelector{
		wrapped:     wrapped,
		allowedTags: allowedTags,
	}
}

func (s *TagsFilterSelector) Select(ctx context.Context, req *llm.Request) ([]*biz.Channel, error) {
	channels, err := s.wrapped.Select(ctx, req)
	if err != nil {
		return nil, err
	}

	// 如果没有指定标签，返回所有渠道
	if len(s.allowedTags) == 0 {
		return channels, nil
	}

	// 构建标签集合用于 O(1) 查找
	allowedSet := lo.SliceToMap(s.allowedTags, func(tag string) (string, struct{}) {
		return tag, struct{}{}
	})

	// 过滤渠道：只保留至少包含一个允许标签的渠道（OR 逻辑）
	// 空标签的渠道不会被匹配
	filtered := lo.Filter(channels, func(ch *biz.Channel, _ int) bool {
		for _, tag := range ch.Tags {
			if _, ok := allowedSet[tag]; ok {
				return true
			}
		}

		return false
	})

	return filtered, nil
}

// SpecifiedChannelSelector allows selecting specific channels (including disabled ones) for testing.
type SpecifiedChannelSelector struct {
	ChannelService *biz.ChannelService
	ChannelID      objects.GUID
}

func NewSpecifiedChannelSelector(channelService *biz.ChannelService, channelID objects.GUID) *SpecifiedChannelSelector {
	return &SpecifiedChannelSelector{
		ChannelService: channelService,
		ChannelID:      channelID,
	}
}

func (s *SpecifiedChannelSelector) Select(ctx context.Context, req *llm.Request) ([]*biz.Channel, error) {
	channel, err := s.ChannelService.GetChannelForTest(ctx, s.ChannelID.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel for test: %w", err)
	}

	if !channel.IsModelSupported(req.Model) {
		return nil, fmt.Errorf("model %s not supported in channel %s", req.Model, channel.Name)
	}

	return []*biz.Channel{channel}, nil
}

// GoogleNativeToolsSelector is a decorator that prioritizes channels supporting Google native tools.
// When a request contains Google native tools (google_search, google_url_context, google_code_execution),
// this selector filters out channels that don't support these tools (e.g., gemini_openai).
// If no compatible channels are found, it falls back to all channels (allowing downstream fallback logic).
type GoogleNativeToolsSelector struct {
	wrapped ChannelSelector
}

// NewGoogleNativeToolsSelector creates a selector that prioritizes Google native tool compatible channels.
func NewGoogleNativeToolsSelector(wrapped ChannelSelector) *GoogleNativeToolsSelector {
	return &GoogleNativeToolsSelector{
		wrapped: wrapped,
	}
}

func (s *GoogleNativeToolsSelector) Select(ctx context.Context, req *llm.Request) ([]*biz.Channel, error) {
	channels, err := s.wrapped.Select(ctx, req)
	if err != nil {
		return nil, err
	}

	// 如果请求不包含 Google 原生工具，直接返回所有渠道
	if !llm.ContainsGoogleNativeTools(req.Tools) {
		return channels, nil
	}

	// 过滤：只保留支持 Google 原生工具的渠道
	compatible := lo.Filter(channels, func(ch *biz.Channel, _ int) bool {
		return ch.Type.SupportsGoogleNativeTools()
	})

	if len(compatible) > 0 {
		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "Filtered channels for Google native tools",
				log.Int("total_channels", len(channels)),
				log.Int("compatible_channels", len(compatible)))
		}

		return compatible, nil
	}

	// 没有兼容渠道时，返回所有渠道（由下游 outbound 进行降级处理）
	log.Warn(ctx, "No channels support Google native tools, falling back to all channels",
		log.Int("total_channels", len(channels)))

	return channels, nil
}

// AnthropicNativeToolsSelector is a decorator that prioritizes channels supporting Anthropic native tools.
// When a request contains Anthropic native tools (web_search -> web_search_20250305),
// this selector filters out channels that don't support these tools (e.g., deepseek_anthropic).
// If no compatible channels are found, it falls back to all channels (allowing downstream fallback logic).
type AnthropicNativeToolsSelector struct {
	wrapped ChannelSelector
}

// NewAnthropicNativeToolsSelector creates a selector that prioritizes Anthropic native tool compatible channels.
func NewAnthropicNativeToolsSelector(wrapped ChannelSelector) *AnthropicNativeToolsSelector {
	return &AnthropicNativeToolsSelector{
		wrapped: wrapped,
	}
}

func (s *AnthropicNativeToolsSelector) Select(ctx context.Context, req *llm.Request) ([]*biz.Channel, error) {
	channels, err := s.wrapped.Select(ctx, req)
	if err != nil {
		return nil, err
	}

	// 如果请求不包含 Anthropic 原生工具，直接返回所有渠道
	if !llm.ContainsAnthropicNativeTools(req.Tools) {
		return channels, nil
	}

	// 过滤：只保留支持 Anthropic 原生工具的渠道
	compatible := lo.Filter(channels, func(ch *biz.Channel, _ int) bool {
		return ch.Type.SupportsAnthropicNativeTools()
	})

	if len(compatible) > 0 {
		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "Filtered channels for Anthropic native tools",
				log.Int("total_channels", len(channels)),
				log.Int("compatible_channels", len(compatible)))
		}

		return compatible, nil
	}

	// 没有兼容渠道时，返回所有渠道（由下游 outbound 进行降级处理）
	log.Warn(ctx, "No channels support Anthropic native tools, falling back to all channels",
		log.Int("total_channels", len(channels)))

	return channels, nil
}
