package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime"
	"strings"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// OnAttemptRawRequest is deliberately not OnOutboundRawRequest: the orchestrator
// schedules it after pass-through and transport adaptation, before overrides.
func (h *requestHookMiddleware) OnAttemptRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	if h == nil || h.session == nil || request == nil {
		return request, nil
	}
	s := h.session
	if s.released || s.gen == nil {
		return request, nil
	}
	contentType := request.ContentType
	if contentType == "" {
		contentType = request.Headers.Get("Content-Type")
	}
	if contentType != "" {
		media, _, err := mime.ParseMediaType(contentType)
		if err != nil || (media != "application/json" && !strings.HasSuffix(media, "+json")) {
			return request, nil //nolint:nilerr // Unsupported or malformed content types skip attempt hooks.
		}
	}
	if !isJSONObject(request.Body) {
		return request, nil
	}
	var attempt attemptContextJSON
	if err := json.Unmarshal(s.attemptJSON, &attempt); err != nil {
		return request, nil //nolint:nilerr // Invalid hook metadata must not block the upstream request.
	}
	// The orchestrator resolves missing wire formats before raw middleware runs.
	attempt.APIFormat = request.APIFormat
	attemptJSON, err := json.Marshal(attempt)
	if err != nil {
		return request, nil //nolint:nilerr // Hook metadata serialization failures leave the request unchanged.
	}
	for _, item := range s.gen.Plugins() {
		if !hookPlugin(item, onAttemptExport) {
			continue
		}
		if !item.Manifest.Hooks.Attempt.Matches(llm.APIFormat(attempt.APIFormat)) {
			reason := "api_format_not_matched"
			if attempt.APIFormat == "" {
				reason = "api_format_unresolved"
			}
			log.Debug(ctx, "plugin attempt hook skipped",
				log.String("plugin_id", item.PluginID),
				log.Int("channel_id", attempt.Channel.ID),
				log.String("api_format", attempt.APIFormat),
				log.String("reason", reason),
			)
			continue
		}
		before := request.Body
		if len(before) > hookInputLimit {
			s.manager.noteCallFailure(item.ID)
			continue
		}
		args, err := json.Marshal([]json.RawMessage{before})
		if err != nil {
			s.manager.noteCallFailure(item.ID)
			continue
		}
		var output json.RawMessage
		var protected []string
		result, err := s.manager.Invoke(ctx, InvokeRequest{
			InstallationID: item.ID, Export: onAttemptExport, Args: args,
			State: s.states[item.ID], Config: item.Config, Timeout: s.manager.hookBudget(),
			Generation: s.gen, BorrowWait: hookBorrowWait,
			HostRequest: s.requestJSON, HostAttempt: attemptJSON,
			ValidateReturn: func(value json.RawMessage) error {
				var err error
				output, err = applyHookReturn(before, value)
				if err != nil {
					return err
				}
				if len(output) > 0 {
					protected = protectedAttemptFields(request, output, attempt.Model)
					if len(protected) > 0 {
						return errProtectedRewrite
					}
				}
				return nil
			},
		})
		if errors.Is(err, errProtectedRewrite) {
			s.manager.noteProtectedDrop(ctx, item, onAttemptExport, attempt.APIFormat, protected)
			continue
		}
		if err != nil {
			_, failure := s.failCall(ctx, item, nil, onAttemptExport, err)
			if failure != nil {
				return nil, failure
			}
			continue
		}
		if len(output) > 0 {
			copy := *request
			copy.Body = bytes.Clone(output)
			if len(copy.JSONBody) > 0 {
				copy.JSONBody = bytes.Clone(output)
			}
			request = &copy
		}
		s.commitState(item.ID, result.State)
	}
	return request, nil
}
