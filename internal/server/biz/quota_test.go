package biz

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/objects"
)

func TestQuotaService_AllTime_RequestCountExceeded(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	p, err := client.Project.Create().
		SetName("p").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	now := time.Now().UTC()
	apiKeyID := 1

	_, err = client.Request.Create().
		SetProjectID(p.ID).
		SetAPIKeyID(apiKeyID).
		SetModelID("m").
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		SetCreatedAt(now.Add(-2 * time.Hour)).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.Request.Create().
		SetProjectID(p.ID).
		SetAPIKeyID(apiKeyID).
		SetModelID("m").
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		SetCreatedAt(now.Add(-1 * time.Hour)).
		Save(ctx)
	require.NoError(t, err)

	svc := NewQuotaService(client)

	quota := &objects.APIKeyQuota{
		Requests: lo.ToPtr(int64(2)),
		Period: objects.APIKeyQuotaPeriod{
			Type: objects.APIKeyQuotaPeriodTypeAllTime,
		},
	}

	res, err := svc.CheckAPIKeyQuota(ctx, apiKeyID, quota)
	require.NoError(t, err)
	require.False(t, res.Allowed)
	require.Contains(t, res.Message, "requests quota exceeded")
}

func TestQuotaService_PastDuration_TotalTokensExceeded(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	p, err := client.Project.Create().
		SetName("p").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	now := time.Now().UTC()
	apiKeyID := 2

	reqInWindow, err := client.Request.Create().
		SetProjectID(p.ID).
		SetAPIKeyID(apiKeyID).
		SetModelID("m").
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		SetCreatedAt(now.Add(-30 * time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.UsageLog.Create().
		SetRequestID(reqInWindow.ID).
		SetAPIKeyID(apiKeyID).
		SetProjectID(p.ID).
		SetModelID("m").
		SetSource(usagelog.SourceAPI).
		SetFormat("openai/chat_completions").
		SetPromptTokens(50).
		SetCompletionTokens(100).
		SetTotalTokens(150).
		SetTotalCost(1.0).
		SetCreatedAt(now.Add(-29 * time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	reqOutWindow, err := client.Request.Create().
		SetProjectID(p.ID).
		SetAPIKeyID(apiKeyID).
		SetModelID("m").
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		SetCreatedAt(now.Add(-3 * time.Hour)).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.UsageLog.Create().
		SetRequestID(reqOutWindow.ID).
		SetAPIKeyID(apiKeyID).
		SetProjectID(p.ID).
		SetModelID("m").
		SetSource(usagelog.SourceAPI).
		SetFormat("openai/chat_completions").
		SetPromptTokens(10).
		SetCompletionTokens(10).
		SetTotalTokens(20).
		SetTotalCost(1.0).
		SetCreatedAt(now.Add(-3 * time.Hour)).
		Save(ctx)
	require.NoError(t, err)

	svc := NewQuotaService(client)
	quota := &objects.APIKeyQuota{
		TotalTokens: lo.ToPtr(int64(100)),
		Period: objects.APIKeyQuotaPeriod{
			Type: objects.APIKeyQuotaPeriodTypePastDuration,
			PastDuration: &objects.APIKeyQuotaPastDuration{
				Value: 1,
				Unit:  objects.APIKeyQuotaPastDurationUnitHour,
			},
		},
	}

	res, err := svc.CheckAPIKeyQuota(ctx, apiKeyID, quota)
	require.NoError(t, err)
	require.False(t, res.Allowed)
	require.Contains(t, res.Message, "total_tokens quota exceeded")
}

func TestQuotaService_CalendarDay_CostExceeded(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	p, err := client.Project.Create().
		SetName("p").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	now := time.Now().UTC()
	apiKeyID := 3

	req, err := client.Request.Create().
		SetProjectID(p.ID).
		SetAPIKeyID(apiKeyID).
		SetModelID("m").
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		SetCreatedAt(now).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.UsageLog.Create().
		SetRequestID(req.ID).
		SetAPIKeyID(apiKeyID).
		SetProjectID(p.ID).
		SetModelID("m").
		SetSource(usagelog.SourceAPI).
		SetFormat("openai/chat_completions").
		SetPromptTokens(1).
		SetCompletionTokens(1).
		SetTotalTokens(2).
		SetTotalCost(11.0).
		SetCreatedAt(now).
		Save(ctx)
	require.NoError(t, err)

	svc := NewQuotaService(client)
	quota := &objects.APIKeyQuota{
		Cost: lo.ToPtr(decimal.NewFromFloat(10.0)),
		Period: objects.APIKeyQuotaPeriod{
			Type: objects.APIKeyQuotaPeriodTypeCalendarDuration,
			CalendarDuration: &objects.APIKeyQuotaCalendarDuration{
				Unit: objects.APIKeyQuotaCalendarDurationUnitDay,
			},
		},
	}

	res, err := svc.CheckAPIKeyQuota(ctx, apiKeyID, quota)
	require.NoError(t, err)
	require.False(t, res.Allowed)
	require.Contains(t, res.Message, "cost quota exceeded")
}
