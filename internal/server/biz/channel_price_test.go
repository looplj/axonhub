package biz

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channelmodelprice"
	"github.com/looplj/axonhub/internal/ent/channelmodelpriceversion"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/objects"
)

func TestChannelService_SaveChannelModelPrices(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a test channel
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key1"}).
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	price1 := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeUsage,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: loToDecimalPtr("0.01"),
				},
			},
		},
	}

	price2 := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeCompletion,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: loToDecimalPtr("0.02"),
				},
			},
		},
	}

	t.Run("batch create", func(t *testing.T) {
		inputs := []SaveChannelModelPriceInput{
			{
				ModelID: "gpt-4",
				Price:   price1,
			},
			{
				ModelID: "gpt-3.5-turbo",
				Price:   price2,
			},
		}

		results, err := svc.SaveChannelModelPrices(ctx, ch.ID, inputs)
		require.NoError(t, err)
		require.Len(t, results, 2)

		for _, res := range results {
			// Check if version is created
			version, err := client.ChannelModelPriceVersion.Query().
				Where(channelmodelpriceversion.ChannelModelPriceID(res.ID)).
				Only(ctx)
			require.NoError(t, err)
			require.Equal(t, channelmodelpriceversion.StatusActive, version.Status)
			require.Equal(t, res.ReferenceID, version.ReferenceID)
			require.Len(t, res.ReferenceID, 8)
		}
	})

	t.Run("batch update and archive old version", func(t *testing.T) {
		// First update gpt-4
		newPrice1 := price1
		newPrice1.Items[0].Pricing.UsagePerUnit = loToDecimalPtr("0.015")

		inputs := []SaveChannelModelPriceInput{
			{
				ModelID: "gpt-4",
				Price:   newPrice1,
			},
		}

		// Store old ref id
		oldPrice, err := client.ChannelModelPrice.Query().
			Where(
				channelmodelprice.ChannelID(ch.ID),
				channelmodelprice.ModelID("gpt-4"),
			).Only(ctx)
		require.NoError(t, err)

		oldRefID := oldPrice.ReferenceID

		// Wait a bit to ensure time difference
		time.Sleep(10 * time.Millisecond)

		results, err := svc.SaveChannelModelPrices(ctx, ch.ID, inputs)
		require.NoError(t, err)
		require.Len(t, results, 1)

		updatedPrice := results[0]
		require.NotEqual(t, oldRefID, updatedPrice.ReferenceID)
		require.Equal(t, newPrice1, updatedPrice.Price)

		// Check versions
		versions, err := client.ChannelModelPriceVersion.Query().
			Where(channelmodelpriceversion.ChannelModelPriceID(updatedPrice.ID)).
			Order(ent.Asc(channelmodelpriceversion.FieldEffectiveStartAt)).
			All(ctx)
		require.NoError(t, err)
		require.Len(t, versions, 2)

		// Old version should be archived
		require.Equal(t, channelmodelpriceversion.StatusArchived, versions[0].Status)
		require.NotNil(t, versions[0].EffectiveEndAt)
		require.Equal(t, oldRefID, versions[0].ReferenceID)

		// New version should be active
		require.Equal(t, channelmodelpriceversion.StatusActive, versions[1].Status)
		require.Nil(t, versions[1].EffectiveEndAt)
		require.Equal(t, updatedPrice.ReferenceID, versions[1].ReferenceID)
	})

	t.Run("delete missing models", func(t *testing.T) {
		// Only send gpt-3.5-turbo, gpt-4 should be deleted
		inputs := []SaveChannelModelPriceInput{
			{
				ModelID: "gpt-3.5-turbo",
				Price:   price2,
			},
		}

		// Verify gpt-4 exists before delete
		exists, err := client.ChannelModelPrice.Query().
			Where(
				channelmodelprice.ChannelID(ch.ID),
				channelmodelprice.ModelID("gpt-4"),
			).Exist(ctx)
		require.NoError(t, err)
		require.True(t, exists)

		results, err := svc.SaveChannelModelPrices(ctx, ch.ID, inputs)
		require.NoError(t, err)
		require.Len(t, results, 1) // Only gpt-3.5-turbo remains (as skip/update)

		// Verify gpt-4 is deleted
		exists, err = client.ChannelModelPrice.Query().
			Where(
				channelmodelprice.ChannelID(ch.ID),
				channelmodelprice.ModelID("gpt-4"),
			).Exist(ctx)
		require.NoError(t, err)
		require.False(t, exists)

		// Verify gpt-4 versions are archived
		versions, err := client.ChannelModelPriceVersion.Query().
			Where(
				channelmodelpriceversion.ChannelID(ch.ID),
				channelmodelpriceversion.ModelID("gpt-4"),
			).All(ctx)
		require.NoError(t, err)

		for _, v := range versions {
			require.Equal(t, channelmodelpriceversion.StatusArchived, v.Status)
			require.NotNil(t, v.EffectiveEndAt)
		}
	})

	t.Run("duplicate model id should error", func(t *testing.T) {
		inputs := []SaveChannelModelPriceInput{
			{
				ModelID: "gpt-4",
				Price:   price1,
			},
			{
				ModelID: "gpt-4",
				Price:   price2,
			},
		}

		_, err := svc.SaveChannelModelPrices(ctx, ch.ID, inputs)
		require.Error(t, err)
		require.Contains(t, err.Error(), "duplicate model price input")
		require.Contains(t, err.Error(), "model_id=gpt-4")
	})
}

func loToDecimalPtr(s string) *decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return &d
}

func TestCalculatePriceChanges(t *testing.T) {
	price1 := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{ItemCode: objects.PriceItemCodeUsage},
		},
	}
	price2 := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{ItemCode: objects.PriceItemCodeCompletion},
		},
	}

	existingPrices := []*ent.ChannelModelPrice{
		{
			ID:      1,
			ModelID: "gpt-4",
			Price:   price1,
		},
	}

	tests := []struct {
		name           string
		existingPrices []*ent.ChannelModelPrice
		inputs         []SaveChannelModelPriceInput
		want           []PriceChangeAction
	}{
		{
			name:           "create and update",
			existingPrices: existingPrices,
			inputs: []SaveChannelModelPriceInput{
				{
					ModelID: "gpt-4",
					Price:   price1,
				},
				{
					ModelID: "gpt-3.5-turbo",
					Price:   price2,
				},
			},
			want: []PriceChangeAction{
				{
					Type:          ActionTypeSkip,
					ModelID:       "gpt-4",
					Price:         price1,
					ExistingPrice: existingPrices[0],
				},
				{
					Type:          ActionTypeCreate,
					ModelID:       "gpt-3.5-turbo",
					Price:         price2,
					ExistingPrice: nil,
				},
			},
		},
		{
			name:           "all create",
			existingPrices: []*ent.ChannelModelPrice{},
			inputs: []SaveChannelModelPriceInput{
				{
					ModelID: "gpt-4",
					Price:   price1,
				},
			},
			want: []PriceChangeAction{
				{
					Type:          ActionTypeCreate,
					ModelID:       "gpt-4",
					Price:         price1,
					ExistingPrice: nil,
				},
			},
		},
		{
			name:           "all update",
			existingPrices: existingPrices,
			inputs: []SaveChannelModelPriceInput{
				{
					ModelID: "gpt-4",
					Price:   price2,
				},
			},
			want: []PriceChangeAction{
				{
					Type:          ActionTypeUpdate,
					ModelID:       "gpt-4",
					Price:         price2,
					ExistingPrice: existingPrices[0],
				},
			},
		},
		{
			name:           "delete missing",
			existingPrices: existingPrices,
			inputs:         []SaveChannelModelPriceInput{},
			want: []PriceChangeAction{
				{
					Type:          ActionTypeDelete,
					ModelID:       "gpt-4",
					ExistingPrice: existingPrices[0],
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculatePriceChanges(tt.existingPrices, tt.inputs)
			assert.Equal(t, tt.want, got)
		})
	}
}
