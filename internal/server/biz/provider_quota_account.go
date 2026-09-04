package biz

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

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

		if index, ok := groupIndexes[accountKey]; ok {
			groups[index].channels = append(groups[index].channels, ch)
			continue
		}

		groupIndexes[accountKey] = len(groups)
		groups = append(groups, quotaCheckGroup{
			channels:   []*ent.Channel{ch},
			accountKey: accountKey,
		})
	}

	return groups
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
