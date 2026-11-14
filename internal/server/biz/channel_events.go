package biz

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/events"
)

const (
	credentialTriggerManual  = "manual"
	credentialTriggerExpired = "expired"
	// credentialTriggerQuota marks a credential event caused by a proactive
	// provider quota observation rather than by an operator or an HTTP rule.
	credentialTriggerQuota = "quota"
)

func (svc *ChannelService) publishChannelAutoEnabled(ctx context.Context, channelID int) {
	if svc == nil || svc.EventBus == nil || !svc.EventBus.HasSubscribers(events.TypeChannelAutoEnabled) {
		return
	}

	ch, err := svc.entFromContext(ctx).Channel.Get(ctx, channelID)
	if err != nil {
		log.Warn(ctx, "failed to load channel for channel.auto_enabled",
			log.Int("channel_id", channelID),
			log.Cause(err),
		)
		return
	}

	svc.EventBus.Publish(ctx, newPluginEvent(ctx, events.TypeChannelAutoEnabled, events.SeverityInfo, events.Subject{
		Channel: entChannelRef(ch, ch.Status),
	}, nil))
}

func (svc *ChannelService) publishCredentialDisabled(
	ctx context.Context,
	ch *ent.Channel,
	key string,
	errorCode int,
	reason string,
	expiresAt *time.Time,
	status channel.Status,
) {
	if svc == nil || svc.EventBus == nil || !svc.EventBus.HasSubscribers(events.TypeChannelCredentialDisabled) {
		return
	}

	oauth := channelUsesOAuth(ch) || key == objects.OAuthCredentialRef
	svc.EventBus.Publish(ctx, newPluginEvent(ctx, events.TypeChannelCredentialDisabled, events.SeverityWarning, events.Subject{
		Channel:    entChannelRef(ch, status),
		Credential: credentialEventRef(oauth, key),
	}, map[string]any{
		"errorCode": errorCode,
		"reason":    reason,
		"expiresAt": rfc3339OrNil(expiresAt),
	}))
}

func (svc *ChannelService) publishCredentialEnabled(
	ctx context.Context,
	ch *ent.Channel,
	keys []string,
	remaining []objects.DisabledAPIKey,
	trigger string,
) {
	if svc == nil || svc.EventBus == nil || len(keys) == 0 || !svc.EventBus.HasSubscribers(events.TypeChannelCredentialEnabled) {
		return
	}

	oauth := channelUsesOAuth(ch)
	channelRef := entChannelRef(ch, channelStatusAfterCredentialChange(ch, remaining, false))
	for _, key := range keys {
		if key == "" && !oauth {
			continue
		}
		svc.EventBus.Publish(ctx, newPluginEvent(ctx, events.TypeChannelCredentialEnabled, events.SeverityInfo, events.Subject{
			Channel:    channelRef,
			Credential: credentialEventRef(oauth, key),
		}, map[string]any{
			"trigger": trigger,
		}))
	}
}

type pluginEventMeta struct {
	origin string
	depth  int
}

type pluginEventMetaKey struct{}

// WithPluginEventMeta stamps bus events published by channel credential helpers.
// Callers that do not set it keep Origin empty and Depth 0.
func WithPluginEventMeta(ctx context.Context, origin string, depth int) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, pluginEventMetaKey{}, pluginEventMeta{origin: origin, depth: depth})
}

func pluginEventMetaFrom(ctx context.Context) (string, int) {
	if ctx == nil {
		return "", 0
	}
	meta, ok := ctx.Value(pluginEventMetaKey{}).(pluginEventMeta)
	if !ok {
		return "", 0
	}
	return meta.origin, meta.depth
}

func newPluginEvent(ctx context.Context, eventType, severity string, subject events.Subject, data map[string]any) events.Event {
	origin, depth := pluginEventMetaFrom(ctx)
	return events.Event{
		ID:         uuid.NewString(),
		Type:       eventType,
		Severity:   severity,
		OccurredAt: time.Now().UTC(),
		Subject:    subject,
		Data:       data,
		Origin:     origin,
		Depth:      depth,
	}
}

func entChannelRef(ch *ent.Channel, status channel.Status) *events.ChannelRef {
	if ch == nil {
		return nil
	}
	if status == "" {
		status = ch.Status
	}

	return &events.ChannelRef{
		ID:      ch.ID,
		Name:    ch.Name,
		Type:    ch.Type.String(),
		BaseURL: ch.BaseURL,
		Status:  status.String(),
	}
}

func channelUsesOAuth(ch *ent.Channel) bool {
	return ch != nil && ch.Credentials.IsOAuth()
}

// credentialEventRef identifies a credential without its plaintext.
// OAuth has no key string: both fields are events.OAuthFingerprint, and the
// token is not hashed.
func credentialEventRef(oauth bool, key string) *events.CredentialRef {
	if oauth || key == objects.OAuthCredentialRef {
		return &events.CredentialRef{
			Fingerprint: events.OAuthFingerprint,
			Masked:      events.OAuthFingerprint,
		}
	}
	if key == "" {
		return nil
	}

	return &events.CredentialRef{
		Fingerprint: events.Fingerprint(key),
		Masked:      events.Mask(key),
	}
}

func channelStatusAfterCredentialChange(ch *ent.Channel, disabledKeys []objects.DisabledAPIKey, forceDisabled bool) channel.Status {
	if ch == nil {
		return ""
	}
	if forceDisabled {
		return channel.StatusDisabled
	}
	// AutoDisabledAt is cleared by every manual status change, so requiring it
	// keeps a recovered credential from undoing a later manual channel disable
	// — or reporting the channel as enabled.
	if ch.Status == channel.StatusDisabled && ch.AutoDisabledAt != nil && ch.ErrorMessage != nil &&
		strings.HasPrefix(*ch.ErrorMessage, allKeysDisabledErrorPrefix) &&
		len(ch.Credentials.GetEnabledCredentialRefs(disabledKeys)) > 0 {
		return channel.StatusEnabled
	}

	return ch.Status
}

// channelIsAllKeysDisabled reports whether the channel is in the state this
// package enters when its last usable credential disappears.
func channelIsAllKeysDisabled(ch *ent.Channel) bool {
	return ch != nil && ch.Status == channel.StatusDisabled && ch.AutoDisabledAt != nil &&
		ch.ErrorMessage != nil && strings.HasPrefix(*ch.ErrorMessage, allKeysDisabledErrorPrefix)
}

func rfc3339OrNil(value *time.Time) any {
	if value == nil {
		return nil
	}

	return value.UTC().Format(time.RFC3339)
}

func disabledKeyValues(keys []objects.DisabledAPIKey) []string {
	if len(keys) == 0 {
		return nil
	}
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, key.Key)
	}

	return values
}
