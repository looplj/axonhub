package biz

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/providerquotastatus"
)

type quotaCheckGroup struct {
	channels   []*ent.Channel
	accountKey string
}

func quotaAccountKey(providerType string, ch *ent.Channel) string {
	if providerType != "zenmux" {
		return ""
	}

	managementKey := strings.TrimSpace(ch.Credentials.ManagementAPIKey)
	if managementKey == "" {
		return ""
	}

	digest := sha256.Sum256([]byte(providerType + ":" + managementKey))
	return hex.EncodeToString(digest[:])[:16]
}

func (svc *ProviderQuotaService) groupChannelsByQuotaAccount(channels []*ent.Channel) []quotaCheckGroup {
	groups := make([]quotaCheckGroup, 0, len(channels))
	groupIndexes := make(map[string]int)

	for _, ch := range channels {
		accountKey := quotaAccountKey(svc.getProviderType(ch), ch)
		if accountKey == "" {
			groups = append(groups, quotaCheckGroup{channels: []*ent.Channel{ch}})
			continue
		}

		groupKey := quotaCheckGroupKey(accountKey, ch)
		if index, ok := groupIndexes[groupKey]; ok {
			groups[index].channels = append(groups[index].channels, ch)
			continue
		}

		groupIndexes[groupKey] = len(groups)
		groups = append(groups, quotaCheckGroup{
			channels:   []*ent.Channel{ch},
			accountKey: accountKey,
		})
	}

	return groups
}

func quotaCheckGroupKey(accountKey string, ch *ent.Channel) string {
	proxyKey := ""
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		proxy, err := json.Marshal(ch.Settings.Proxy)
		if err == nil {
			proxyKey = string(proxy)
		}
	}
	return accountKey + "\x00" + proxyKey
}

func quotaCheckGroupIsDue(group quotaCheckGroup, now time.Time) bool {
	for _, ch := range group.channels {
		status := ch.Edges.ProviderQuotaStatus
		if status == nil || !status.NextCheckAt.After(now) {
			return true
		}
	}
	return false
}

func nextQuotaGroupErrorCount(channels []*ent.Channel, providerType string) int {
	provider := providerquotastatus.ProviderType(providerType)
	previousFailures := 0

	for _, ch := range channels {
		existing := ch.Edges.ProviderQuotaStatus
		if existing == nil || existing.ProviderType != provider {
			continue
		}

		previousFailures = max(previousFailures, quotaErrorCount(existing.QuotaData))
	}

	return nextQuotaErrorCount(previousFailures)
}
