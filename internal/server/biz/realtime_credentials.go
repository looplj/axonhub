// 职责: 为独立实时会话选择一次渠道凭证并复用既有密钥策略。
// 关联文件:
//
//	internal/server/biz/channel_llm.go
//	internal/server/orchestrator/realtime.go

package biz

import (
	"context"
	"errors"
)

func (c *Channel) RealtimeAPIKey(ctx context.Context) (string, error) {
	provider, err := getAPIKeyProvider(c)
	if err != nil {
		return "", err
	}
	key := provider.Get(ctx)
	if key == "" {
		return "", errors.New("no enabled realtime credential")
	}
	return key, nil
}

// RecordRealtimeCredentialSuccess advances the existing successful-request rotation cursor.
// It affects subsequent connections; the current session keeps its original key.
func (s *ChannelService) RecordRealtimeCredentialSuccess(channelID int, key string) {
	s.clearAutoDisableCountsOnSuccess(&PerformanceRecord{ChannelID: channelID, APIKey: key})
	s.onAPIKeySuccess(channelID, key)
}
