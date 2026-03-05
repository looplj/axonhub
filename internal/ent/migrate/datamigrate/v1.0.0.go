package datamigrate

import (
	"context"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/agenthost"
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

	exists, err := client.AgentHost.Query().
		Where(agenthost.TypeEQ(agenthost.TypeLocal)).
		Exist(ctx)
	if err != nil {
		return err
	}

	if exists {
		log.Info(ctx, "local agent host already exists, skip migration")
		return nil
	}

	host, err := client.AgentHost.Create().
		SetName("Local").
		SetType(agenthost.TypeLocal).
		SetStatus(agenthost.StatusActive).
		Save(ctx)
	if err != nil {
		return err
	}

	log.Info(ctx, "created local agent host", log.Int("agent_host_id", host.ID))

	return nil
}
