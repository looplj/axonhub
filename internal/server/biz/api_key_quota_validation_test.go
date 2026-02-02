package biz

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
)

func TestValidateProfileQuota_PastDurationMinuteAccepted(t *testing.T) {
	err := validateProfileQuota([]objects.APIKeyProfile{
		{
			Name: "p",
			Quota: &objects.APIKeyQuota{
				Requests: lo.ToPtr(int64(1)),
				Period: objects.APIKeyQuotaPeriod{
					Type: objects.APIKeyQuotaPeriodTypePastDuration,
					PastDuration: &objects.APIKeyQuotaPastDuration{
						Value: 1,
						Unit:  objects.APIKeyQuotaPastDurationUnitMinute,
					},
				},
			},
		},
	})
	require.NoError(t, err)
}

