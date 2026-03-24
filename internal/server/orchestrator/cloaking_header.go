package orchestrator

import (
	"context"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
)

// defaultCloakingHeaders are the default headers to set for cloaking.
var defaultCloakingHeaders = map[string]string{
	"User-Agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Accept":          "text/event-stream",
	"Accept-Language": "en-US,en;q=0.9",
	"Accept-Encoding": "gzip, deflate, br",
	"Sec-Fetch-Dest":  "empty",
	"Sec-Fetch-Mode":  "cors",
	"Sec-Fetch-Site":  "same-origin",
}

// proxyHeadersToRemove are headers that identify proxy usage and must be removed.
var proxyHeadersToRemove = []string{
	"X-Forwarded-For",
	"X-Forwarded-Host",
	"X-Forwarded-Proto",
	"Via",
	"Referer",
	"Referrer",
}

// secCHUAHeadersToRemove are Sec-CH-UA headers that reveal client fingerprinting.
var secCHUAHeadersToRemove = []string{
	"Sec-CH-UA",
	"Sec-CH-UA-Arch",
	"Sec-CH-UA-Bitness",
	"Sec-CH-UA-Full-Version",
	"Sec-CH-UA-Full-Version-List",
	"Sec-CH-UA-Mobile",
	"Sec-CH-UA-Model",
	"Sec-CH-UA-Platform",
	"Sec-CH-UA-Platform-Version",
	"Sec-CH-UA-WoAW-Info",
}

// applyCloakingHeaders creates a middleware that applies cloaking headers.
func applyCloakingHeaders(outbound *PersistentOutboundTransformer) pipeline.Middleware {
	return pipeline.OnRawRequest("cloaking-headers", func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
		if request == nil {
			return request, nil
		}

		if request.Headers == nil {
			request.Headers = make(http.Header)
		}
		if outbound == nil || outbound.state == nil {
			return request, nil
		}

		channel := outbound.GetCurrentChannel()
		if channel == nil {
			return request, nil
		}

		// Get effective mode: channel mode > global mode
		effectiveMode := getEffectiveCloakingMode(ctx, channel, outbound)
		if effectiveMode == nil || *effectiveMode == "never" {
			return request, nil
		}

		// Only apply cloaking for auto or always mode
		if *effectiveMode != "auto" && *effectiveMode != "always" {
			return request, nil
		}

		// Get global config for sub-switches
		globalConfig := getGlobalCloakingConfig(ctx, outbound)

		// Check HeaderAutoFill sub-switch: nil defaults to true in new processor path
		headerAutoFill := true // default to true in new processor path
		if globalConfig.HeaderAutoFill != nil {
			headerAutoFill = *globalConfig.HeaderAutoFill
		}

		if !headerAutoFill {
			return request, nil
		}

		// In auto mode, if User-Agent starts with "claude-cli/", do NOT autofill
		if *effectiveMode == "auto" {
			ua := request.Headers.Get("User-Agent")
			if strings.HasPrefix(ua, "claude-cli/") {
				return request, nil
			}
		}

		// Get user override headers to protect them from cloaking
		userOverrideKeys := getUserOverrideHeaderKeys(channel)

		// Remove proxy-identifying headers first
		request = removeProxyHeaders(request)

		// Apply cloaking headers with precedence: user override > cloaking autofill > client original
		request = applyDefaultCloakingHeaders(request, userOverrideKeys)

		return request, nil
	})
}

// getEffectiveCloakingMode determines the effective cloaking mode.
func getEffectiveCloakingMode(ctx context.Context, channel *biz.Channel, outbound *PersistentOutboundTransformer) *string {
	return resolveEffectiveCloakingMode(ctx, outbound.state, channel)
}

// getGlobalCloakingConfig retrieves the global cloaking configuration.
func getGlobalCloakingConfig(ctx context.Context, outbound *PersistentOutboundTransformer) *biz.GlobalCloakingConfig {
	if outbound == nil {
		return &biz.GlobalCloakingConfig{}
	}
	return resolveGlobalCloakingConfig(ctx, outbound.state)
}

// getUserOverrideHeaderKeys returns a map of header keys that are explicitly
// overridden by the user in channel settings.
func getUserOverrideHeaderKeys(channel *biz.Channel) map[string]bool {
	result := make(map[string]bool)
	if channel == nil || channel.Settings == nil {
		return result
	}

	for _, op := range channel.GetHeaderOverrideOperations() {
		switch op.Op {
		case "set", "delete":
			if op.Path != "" {
				result[strings.ToLower(op.Path)] = true
			}
		case "rename":
			if op.From != "" {
				result[strings.ToLower(op.From)] = true
			}
			if op.To != "" {
				result[strings.ToLower(op.To)] = true
			}
		case "copy":
			if op.To != "" {
				result[strings.ToLower(op.To)] = true
			}
		}
	}

	return result
}

// removeProxyHeaders removes headers that identify proxy usage.
func removeProxyHeaders(request *httpclient.Request) *httpclient.Request {
	headers := request.Headers

	// Remove standard proxy headers
	for _, header := range proxyHeadersToRemove {
		headers.Del(header)
	}

	// Remove Sec-CH-UA-* headers
	for _, header := range secCHUAHeadersToRemove {
		headers.Del(header)
	}

	request.Headers = headers
	return request
}

// applyDefaultCloakingHeaders applies cloaking headers with proper precedence.
// Priority: user override > cloaking autofill > client original
// userOverrideKeys are the header keys that should NOT be overwritten by cloaking
func applyDefaultCloakingHeaders(request *httpclient.Request, userOverrideKeys map[string]bool) *httpclient.Request {
	headers := request.Headers

	// Apply cloaking headers, but skip if user explicitly override this header
	for key, value := range defaultCloakingHeaders {
		if userOverrideKeys[strings.ToLower(key)] {
			// User has explicitly set this header, preserve it
			continue
		}
		// Overwrite with cloaking header
		headers.Set(key, value)
	}

	request.Headers = headers
	return request
}
