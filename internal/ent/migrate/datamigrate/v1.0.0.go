package datamigrate

import (
	"context"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/agentruntime"
	"github.com/looplj/axonhub/internal/log"
)

type V1_0_0 struct{}

func NewV1_0_0() DataMigrator {
	return &V1_0_0{}
}

func (v *V1_0_0) Version() string {
	return "v1.0.0"
}

func (v *V1_0_0) Migrate(ctx context.Context, client *ent.Client) (err error) {
	ctx = authz.WithSystemBypass(context.Background(), "database-migrate")

	exists, err := client.AgentRuntime.Query().
		Where(agentruntime.TypeEQ(agentruntime.TypeLocal)).
		Exist(ctx)
	if err != nil {
		return err
	}

	if exists {
		log.Info(ctx, "local agent runtime already exists, skip migration")
		return nil
	}

	runtime, err := client.AgentRuntime.Create().
		SetName("Local").
		SetType(agentruntime.TypeLocal).
		SetStatus(agentruntime.StatusActive).
		Save(ctx)
	if err != nil {
		return err
	}

	log.Info(ctx, "created local agent runtime", log.Int("agent_runtime_id", runtime.ID))

	return nil
}
