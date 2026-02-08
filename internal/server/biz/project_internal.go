package biz

import (
	"context"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
)

func (s *ProjectService) getProjectByIDWithBypass(ctx context.Context, id int) (*ent.Project, error) {
	return authz.RunWithSystemBypass(ctx, "project-get-by-id", func(ctx context.Context) (*ent.Project, error) {
		return s.getProjectByID(ctx, id)
	})
}
