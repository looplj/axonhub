package biz

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/objects"
)

func BenchmarkQuotaService_CheckAPIKeyQuota_PastDurationMinute_RequestsOnly(b *testing.B) {
	client := enttest.NewEntClient(b, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	p, err := client.Project.Create().
		SetName("p").
		SetStatus(project.StatusActive).
		Save(ctx)
	if err != nil {
		b.Fatal(err)
	}

	now := time.Now().UTC()
	apiKeyID := 100

	for i := 0; i < 1000; i++ {
		req, err := client.Request.Create().
			SetProjectID(p.ID).
			SetAPIKeyID(apiKeyID).
			SetModelID("m").
			SetFormat("openai/chat_completions").
			SetStatus(request.StatusCompleted).
			SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
			SetCreatedAt(now.Add(-time.Duration(i%60) * time.Second)).
			Save(ctx)
		if err != nil {
			b.Fatal(err)
		}

		_, err = client.UsageLog.Create().
			SetRequestID(req.ID).
			SetAPIKeyID(apiKeyID).
			SetProjectID(p.ID).
			SetChannelID(1).
			SetModelID("m").
			SetSource(usagelog.SourceAPI).
			SetFormat("openai/chat_completions").
			SetPromptTokens(1).
			SetCompletionTokens(1).
			SetTotalTokens(2).
			SetTotalCost(0.001).
			SetCreatedAt(req.CreatedAt).
			Save(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}

	systemService := NewSystemService(SystemServiceParams{Ent: client})
	svc := NewQuotaService(client, systemService)

	quota := &objects.APIKeyQuota{
		Requests: lo.ToPtr(int64(1_000_000)),
		Period: objects.APIKeyQuotaPeriod{
			Type: objects.APIKeyQuotaPeriodTypePastDuration,
			PastDuration: &objects.APIKeyQuotaPastDuration{
				Value: 10,
				Unit:  objects.APIKeyQuotaPastDurationUnitMinute,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := svc.CheckAPIKeyQuota(ctx, apiKeyID, quota)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkQuotaService_CheckAPIKeyQuota_PastDurationMinute_TokensAndCost(b *testing.B) {
	client := enttest.NewEntClient(b, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	p, err := client.Project.Create().
		SetName("p").
		SetStatus(project.StatusActive).
		Save(ctx)
	if err != nil {
		b.Fatal(err)
	}

	now := time.Now().UTC()
	apiKeyID := 101

	for i := 0; i < 1000; i++ {
		req, err := client.Request.Create().
			SetProjectID(p.ID).
			SetAPIKeyID(apiKeyID).
			SetModelID("m").
			SetFormat("openai/chat_completions").
			SetStatus(request.StatusCompleted).
			SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
			SetCreatedAt(now.Add(-time.Duration(i%60) * time.Second)).
			Save(ctx)
		if err != nil {
			b.Fatal(err)
		}

		_, err = client.UsageLog.Create().
			SetRequestID(req.ID).
			SetAPIKeyID(apiKeyID).
			SetProjectID(p.ID).
			SetChannelID(1).
			SetModelID("m").
			SetSource(usagelog.SourceAPI).
			SetFormat("openai/chat_completions").
			SetPromptTokens(1).
			SetCompletionTokens(1).
			SetTotalTokens(2).
			SetTotalCost(0.001).
			SetCreatedAt(req.CreatedAt).
			Save(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}

	systemService := NewSystemService(SystemServiceParams{Ent: client})
	svc := NewQuotaService(client, systemService)

	quota := &objects.APIKeyQuota{
		TotalTokens: lo.ToPtr(int64(1_000_000)),
		Cost:        lo.ToPtr(decimal.NewFromFloat(10_000)),
		Period: objects.APIKeyQuotaPeriod{
			Type: objects.APIKeyQuotaPeriodTypePastDuration,
			PastDuration: &objects.APIKeyQuotaPastDuration{
				Value: 10,
				Unit:  objects.APIKeyQuotaPastDurationUnitMinute,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := svc.CheckAPIKeyQuota(ctx, apiKeyID, quota)
		if err != nil {
			b.Fatal(err)
		}
	}
}

