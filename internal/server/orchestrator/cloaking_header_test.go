package orchestrator

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zhenzou/executors"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestCloakingHeaderMiddleware(t *testing.T) {
	ctx := context.Background()

	// Test 1: Override wins over autofill
	t.Run("Override wins over autofill", func(t *testing.T) {
		testCtx, outbound := createTestOutboundWithConfig(ctx, t, nil, nil)

		middleware := applyCloakingHeaders(outbound)

		// User sets their own User-Agent which should NOT be overwritten
		rawRequest := &httpclient.Request{
			Headers: http.Header{
				"User-Agent": []string{"my-custom-agent/1.0"},
			},
		}

		processedRequest, err := middleware.OnOutboundRawRequest(testCtx, rawRequest)
		require.NoError(t, err)
		require.Equal(t, "my-custom-agent/1.0", processedRequest.Headers.Get("User-Agent"))
	})

	// Test 2: claude-cli UA in auto mode does not trigger autofill
	t.Run("claude-cli UA in auto mode does not trigger autofill", func(t *testing.T) {
		autoMode := "auto"
		globalConfig := &biz.GlobalCloakingConfig{
			Mode:           &autoMode,
			HeaderAutoFill: nil, // nil defaults to true, but claude-cli should bypass
		}
		testCtx, outbound := createTestOutboundWithConfig(ctx, t, nil, globalConfig)

		middleware := applyCloakingHeaders(outbound)

		rawRequest := &httpclient.Request{
			Headers: http.Header{
				"User-Agent": []string{"claude-cli/1.2.3"},
			},
		}

		processedRequest, err := middleware.OnOutboundRawRequest(testCtx, rawRequest)
		require.NoError(t, err)

		// claude-cli UA should NOT be overwritten, so the default cloaking headers should not be set
		ua := processedRequest.Headers.Get("User-Agent")
		require.Equal(t, "claude-cli/1.2.3", ua)

		// Accept header should also not be set (it's not claude-cli)
		// because the middleware should return early
		accept := processedRequest.Headers.Get("Accept")
		require.Empty(t, accept, "Accept should not be set for claude-cli")
	})

	// Test 3: curl UA in auto mode triggers autofill
	t.Run("curl UA in auto mode triggers autofill", func(t *testing.T) {
		autoMode := "auto"
		globalConfig := &biz.GlobalCloakingConfig{
			Mode:           &autoMode,
			HeaderAutoFill: nil, // nil defaults to true
		}
		testCtx, outbound := createTestOutboundWithConfig(ctx, t, nil, globalConfig)

		middleware := applyCloakingHeaders(outbound)

		rawRequest := &httpclient.Request{
			Headers: http.Header{
				"User-Agent": []string{"curl/8.1.2"},
			},
		}

		processedRequest, err := middleware.OnOutboundRawRequest(testCtx, rawRequest)
		require.NoError(t, err)

		// curl should be overwritten with default cloaking User-Agent
		ua := processedRequest.Headers.Get("User-Agent")
		require.Contains(t, ua, "Mozilla/5.0", "User-Agent should be cloaked")
		require.NotEqual(t, "curl/8.1.2", ua)

		// Accept header should be set
		accept := processedRequest.Headers.Get("Accept")
		require.Equal(t, "text/event-stream", accept)
	})

	// Test 4: Proxy headers are removed
	t.Run("Proxy headers are removed", func(t *testing.T) {
		alwaysMode := "always"
		globalConfig := &biz.GlobalCloakingConfig{
			Mode:           &alwaysMode,
			HeaderAutoFill: nil, // nil defaults to true
		}
		testCtx, outbound := createTestOutboundWithConfig(ctx, t, nil, globalConfig)

		middleware := applyCloakingHeaders(outbound)

		rawRequest := &httpclient.Request{
			Headers: http.Header{
				"User-Agent":         []string{"test-agent"},
				"X-Forwarded-For":    []string{"1.2.3.4"},
				"Via":                []string{"proxy/1.0"},
				"Referer":            []string{"https://example.com"},
				"Sec-CH-UA":          []string{"\"Chromium\";v=\"120\""},
				"Sec-CH-UA-Platform": []string{"\"macOS\""},
			},
		}

		processedRequest, err := middleware.OnOutboundRawRequest(testCtx, rawRequest)
		require.NoError(t, err)

		// Proxy headers should be removed
		require.Empty(t, processedRequest.Headers.Get("X-Forwarded-For"))
		require.Empty(t, processedRequest.Headers.Get("Via"))
		require.Empty(t, processedRequest.Headers.Get("Referer"))
		require.Empty(t, processedRequest.Headers.Get("Sec-CH-UA"))
		require.Empty(t, processedRequest.Headers.Get("Sec-CH-UA-Platform"))
	})

	// Test 5: mode=nil + header_auto_fill=true does not run cloaking
	t.Run("mode=nil + header_auto_fill=true does not run cloaking", func(t *testing.T) {
		// Global config with header_auto_fill=true but no mode
		globalConfig := &biz.GlobalCloakingConfig{
			Mode:           nil, // no global mode
			HeaderAutoFill: testBoolPtr(true),
		}
		testCtx, outbound := createTestOutboundWithConfig(ctx, t, nil, globalConfig)

		middleware := applyCloakingHeaders(outbound)

		rawRequest := &httpclient.Request{
			Headers: http.Header{
				"User-Agent": []string{"curl/8.1.2"},
			},
		}

		processedRequest, err := middleware.OnOutboundRawRequest(testCtx, rawRequest)
		require.NoError(t, err)

		// With no mode, cloaking should NOT run even if header_auto_fill=true
		ua := processedRequest.Headers.Get("User-Agent")
		require.Equal(t, "curl/8.1.2", ua, "User-Agent should not be cloaked when mode is nil")
	})

	// Test 6: mode=auto + header_auto_fill=nil enables autofill by default
	t.Run("mode=auto + header_auto_fill=nil enables autofill by default", func(t *testing.T) {
		autoMode := "auto"
		globalConfig := &biz.GlobalCloakingConfig{
			Mode:           &autoMode,
			HeaderAutoFill: nil, // nil defaults to true in new processor path
		}
		testCtx, outbound := createTestOutboundWithConfig(ctx, t, nil, globalConfig)

		middleware := applyCloakingHeaders(outbound)

		rawRequest := &httpclient.Request{
			Headers: http.Header{
				"User-Agent": []string{"curl/8.1.2"},
			},
		}

		processedRequest, err := middleware.OnOutboundRawRequest(testCtx, rawRequest)
		require.NoError(t, err)

		// With mode=auto and header_auto_fill=nil (defaults to true), cloaking should run
		ua := processedRequest.Headers.Get("User-Agent")
		require.Contains(t, ua, "Mozilla/5.0", "User-Agent should be cloaked when mode=auto and header_auto_fill=nil")
	})

	// Test 7: Always mode applies cloaking regardless of UA
	t.Run("Always mode applies cloaking regardless of UA", func(t *testing.T) {
		alwaysMode := "always"
		globalConfig := &biz.GlobalCloakingConfig{
			Mode:           &alwaysMode,
			HeaderAutoFill: nil, // defaults to true
		}
		testCtx, outbound := createTestOutboundWithConfig(ctx, t, nil, globalConfig)

		middleware := applyCloakingHeaders(outbound)

		rawRequest := &httpclient.Request{
			Headers: http.Header{
				"User-Agent": []string{"claude-cli/1.2.3"}, // even claude-cli
			},
		}

		processedRequest, err := middleware.OnOutboundRawRequest(testCtx, rawRequest)
		require.NoError(t, err)

		// In always mode, even claude-cli UA should be cloaked
		ua := processedRequest.Headers.Get("User-Agent")
		require.Contains(t, ua, "Mozilla/5.0", "User-Agent should be cloaked in always mode")
		require.NotEqual(t, "claude-cli/1.2.3", ua)
	})

	// Test 8: header_auto_fill=false disables cloaking
	t.Run("header_auto_fill=false disables cloaking", func(t *testing.T) {
		autoMode := "auto"
		globalConfig := &biz.GlobalCloakingConfig{
			Mode:           &autoMode,
			HeaderAutoFill: testBoolPtr(false), // explicitly disabled
		}
		testCtx, outbound := createTestOutboundWithConfig(ctx, t, nil, globalConfig)

		middleware := applyCloakingHeaders(outbound)

		rawRequest := &httpclient.Request{
			Headers: http.Header{
				"User-Agent": []string{"curl/8.1.2"},
			},
		}

		processedRequest, err := middleware.OnOutboundRawRequest(testCtx, rawRequest)
		require.NoError(t, err)

		// With header_auto_fill=false, no cloaking should happen
		ua := processedRequest.Headers.Get("User-Agent")
		require.Equal(t, "curl/8.1.2", ua, "User-Agent should not be cloaked when header_auto_fill=false")
	})

	// Test 9: Channel mode overrides global mode
	t.Run("Channel mode overrides global mode", func(t *testing.T) {
		// Global mode is always
		alwaysMode := "always"
		// Channel mode is never
		neverMode := "never"
		globalConfig := &biz.GlobalCloakingConfig{
			Mode:           &alwaysMode,
			HeaderAutoFill: nil, // defaults to true
		}
		channelSettings := &objects.ChannelSettings{
			CloakingMode: &neverMode,
		}
		testCtx, outbound := createTestOutboundWithConfig(ctx, t, channelSettings, globalConfig)

		middleware := applyCloakingHeaders(outbound)

		rawRequest := &httpclient.Request{
			Headers: http.Header{
				"User-Agent": []string{"curl/8.1.2"},
			},
		}

		processedRequest, err := middleware.OnOutboundRawRequest(testCtx, rawRequest)
		require.NoError(t, err)

		// Channel mode=never should override global mode=always
		ua := processedRequest.Headers.Get("User-Agent")
		require.Equal(t, "curl/8.1.2", ua, "User-Agent should not be cloaked when channel mode=never")
	})

	// Test 10: follow_global uses global mode
	t.Run("follow_global uses global mode", func(t *testing.T) {
		// Global mode is always
		alwaysMode := "always"
		// Channel mode is follow_global
		followGlobal := "follow_global"
		globalConfig := &biz.GlobalCloakingConfig{
			Mode:           &alwaysMode,
			HeaderAutoFill: nil, // defaults to true
		}
		channelSettings := &objects.ChannelSettings{
			CloakingMode: &followGlobal,
		}
		testCtx, outbound := createTestOutboundWithConfig(ctx, t, channelSettings, globalConfig)

		middleware := applyCloakingHeaders(outbound)

		rawRequest := &httpclient.Request{
			Headers: http.Header{
				"User-Agent": []string{"curl/8.1.2"},
			},
		}

		processedRequest, err := middleware.OnOutboundRawRequest(testCtx, rawRequest)
		require.NoError(t, err)

		// follow_global should use global mode=always
		ua := processedRequest.Headers.Get("User-Agent")
		require.Contains(t, ua, "Mozilla/5.0", "User-Agent should be cloaked when follow_global with global mode=always")
	})

	t.Run("header delete override blocks cloaking autofill", func(t *testing.T) {
		autoMode := "auto"
		globalConfig := &biz.GlobalCloakingConfig{
			Mode:           &autoMode,
			HeaderAutoFill: nil,
		}
		channelSettings := &objects.ChannelSettings{
			HeaderOverrideOperations: []objects.OverrideOperation{
				{Op: objects.OverrideOpDelete, Path: "Accept"},
			},
		}

		testCtx, outbound := createTestOutboundWithConfig(ctx, t, channelSettings, globalConfig)
		middleware := applyCloakingHeaders(outbound)

		rawRequest := &httpclient.Request{
			Headers: http.Header{
				"User-Agent": []string{"curl/8.1.2"},
			},
		}

		processedRequest, err := middleware.OnOutboundRawRequest(testCtx, rawRequest)
		require.NoError(t, err)

		require.Empty(t, processedRequest.Headers.Get("Accept"), "Accept should remain deleted by user override")
		require.Contains(t, processedRequest.Headers.Get("User-Agent"), "Mozilla/5.0")
	})
}

// testBoolPtr is a helper function for tests to create bool pointers.
func testBoolPtr(b bool) *bool {
	return &b
}

// Helper function to create a test outbound transformer with mocked config
// Returns both the context (with ent client and bypass) and the outbound transformer
func createTestOutboundWithConfig(
	parentCtx context.Context,
	t *testing.T,
	channelSettings *objects.ChannelSettings,
	globalConfig *biz.GlobalCloakingConfig,
) (context.Context, *PersistentOutboundTransformer) {
	if channelSettings == nil {
		channelSettings = &objects.ChannelSettings{}
	}

	// Create ent client for testing
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	ctx := ent.NewContext(parentCtx, client)
	// Add test bypass for ent privacy rules
	ctx = authz.WithTestBypass(ctx)

	// Create system service with proper ent client and set the global config
	systemService := biz.NewSystemService(biz.SystemServiceParams{
		Ent: client,
	})
	if globalConfig != nil {
		err := systemService.SetGlobalCloakingConfig(ctx, globalConfig)
		require.NoError(t, err)
	}

	// Create channel service that uses the same system service
	// (cannot use NewChannelServiceForTest as it creates its own mock SystemService)
	channelService := biz.NewChannelService(biz.ChannelServiceParams{
		CacheConfig:   xcache.Config{Mode: xcache.ModeMemory},
		Executor:      executors.NewPoolScheduleExecutor(),
		Ent:           client,
		SystemService: systemService,
		HttpClient:    httpclient.NewHttpClient(),
	})
	channelService.SetEnabledChannelsForTest([]*biz.Channel{})

	// Create the channel
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:       1,
			Name:     "test-channel",
			Type:     "anthropic",
			Settings: channelSettings,
		},
	}

	// Create persistence state with the channel service
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{
			Channel: channel,
		},
		ChannelService: channelService,
	}

	outbound := &PersistentOutboundTransformer{
		wrapped: nil,
		state:   state,
	}

	return ctx, outbound
}

// Test helper functions directly
func TestRemoveProxyHeaders(t *testing.T) {
	t.Run("removes all proxy headers", func(t *testing.T) {
		req := &httpclient.Request{
			Headers: http.Header{
				"User-Agent":         []string{"test"},
				"X-Forwarded-For":    []string{"1.2.3.4"},
				"Via":                []string{"proxy"},
				"Referer":            []string{"https://example.com"},
				"Sec-CH-UA":          []string{"\"Chromium\""},
				"Sec-CH-UA-Platform": []string{"\"macOS\""},
			},
		}

		result := removeProxyHeaders(req)

		require.Empty(t, result.Headers.Get("X-Forwarded-For"))
		require.Empty(t, result.Headers.Get("Via"))
		require.Empty(t, result.Headers.Get("Referer"))
		require.Empty(t, result.Headers.Get("Sec-CH-UA"))
		require.Empty(t, result.Headers.Get("Sec-CH-UA-Platform"))
		// User-Agent should remain
		require.Equal(t, "test", result.Headers.Get("User-Agent"))
	})
}

func TestApplyDefaultCloakingHeaders(t *testing.T) {
	t.Run("applies headers when empty", func(t *testing.T) {
		req := &httpclient.Request{
			Headers: http.Header{},
		}

		result := applyDefaultCloakingHeaders(req, make(map[string]bool))

		require.Equal(t, "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", result.Headers.Get("User-Agent"))
		require.Equal(t, "text/event-stream", result.Headers.Get("Accept"))
	})

	t.Run("does not overwrite user override", func(t *testing.T) {
		req := &httpclient.Request{
			Headers: http.Header{
				"User-Agent": []string{"custom-agent"},
			},
		}

		// User override for User-Agent means it should NOT be overwritten
		userOverrides := map[string]bool{"user-agent": true}
		result := applyDefaultCloakingHeaders(req, userOverrides)

		// Should keep the user override
		require.Equal(t, "custom-agent", result.Headers.Get("User-Agent"))
	})

	t.Run("overwrites existing headers when no user override", func(t *testing.T) {
		req := &httpclient.Request{
			Headers: http.Header{
				"User-Agent": []string{"curl/8.1.2"},
			},
		}

		// No user override - headers should be overwritten
		userOverrides := make(map[string]bool)
		result := applyDefaultCloakingHeaders(req, userOverrides)

		// Should be overwritten with cloaking header
		require.Equal(t, "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", result.Headers.Get("User-Agent"))
	})
}

func TestGetEffectiveCloakingMode(t *testing.T) {
	ctx := context.Background()

	t.Run("channel mode takes precedence", func(t *testing.T) {
		alwaysMode := "always"
		neverMode := "never"
		channelSettings := &objects.ChannelSettings{
			CloakingMode: &neverMode,
		}
		globalConfig := &biz.GlobalCloakingConfig{
			Mode: &alwaysMode,
		}
		testCtx, outbound := createTestOutboundWithConfig(ctx, t, channelSettings, globalConfig)
		channel := outbound.state.CurrentCandidate.Channel

		mode := getEffectiveCloakingMode(testCtx, channel, outbound)

		require.NotNil(t, mode)
		require.Equal(t, "never", *mode)
	})

	t.Run("follow_global uses global mode", func(t *testing.T) {
		alwaysMode := "always"
		followGlobal := "follow_global"
		channelSettings := &objects.ChannelSettings{
			CloakingMode: &followGlobal,
		}
		globalConfig := &biz.GlobalCloakingConfig{
			Mode: &alwaysMode,
		}
		testCtx, outbound := createTestOutboundWithConfig(ctx, t, channelSettings, globalConfig)
		channel := outbound.state.CurrentCandidate.Channel

		mode := getEffectiveCloakingMode(testCtx, channel, outbound)

		require.NotNil(t, mode)
		require.Equal(t, "always", *mode)
	})
}
