// 职责: 验证实时翻译的按分钟费用及价格校验。
// 关联文件:
//   internal/server/biz/realtime_translation_billing.go

package biz

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
)

func TestRealtimeTranslationCost(t *testing.T) {
	rate := decimal.RequireFromString("0.034")
	item := objects.ModelPriceItem{
		ItemCode: objects.PriceItemCodeRealtimeTranslationAudioSamples,
		Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &rate},
	}
	snapshot := &RealtimePriceSnapshot{Price: objects.ModelPrice{Items: []objects.ModelPriceItem{item}}}
	require.NoError(t, validateTranslationPrice(snapshot.Price))
	for _, tc := range []struct {
		samples int64
		want    string
	}{{0, "0"}, {24000, "0.0005666666666667"}, {720000, "0.017"}, {1440000, "0.034"}} {
		subtotal, err := RealtimeTranslationCost(snapshot, tc.samples)
		require.NoError(t, err)
		require.Equal(t, tc.want, subtotal.String())
	}
	_, err := RealtimeTranslationCost(snapshot, -1)
	require.Error(t, err)
	require.Error(t, validateTranslationPrice(objects.ModelPrice{}))
	require.Error(t, validateTranslationPrice(objects.ModelPrice{Items: []objects.ModelPriceItem{item, item}}))
}

func TestRealtimeTranslationUsageWithoutPrice(t *testing.T) {
	fixture := newRealtimeBillingFixture(t, realtimePrice("1", "2", "3", "4"))
	snapshot, err := fixture.svc.RealtimeTranslationPrice(fixture.ctx, fixture.channelID, fixture.model)
	require.NoError(t, err)
	require.Nil(t, snapshot, "ordinary Realtime rates do not price translation")
	row, err := fixture.svc.CreateRealtimeTranslationUsage(fixture.ctx, fixture.createParams(nil), snapshot, 24000)
	require.NoError(t, err)
	require.Nil(t, row.TotalCost)
	require.Empty(t, row.CostItems)
	require.Empty(t, row.CostPriceReferenceID)
}
