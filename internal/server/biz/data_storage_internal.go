package biz

import (
	"context"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/log"
)

func (s *DataStorageService) refreshFileSystemsPeriodic(ctx context.Context) {
	ctx = authz.WithSystemBypass(ctx, "refresh data storage filesystems")

	if err := s.refreshFileSystems(ctx); err != nil {
		log.Error(ctx, "failed to refresh data storage filesystems", log.Cause(err))
	}
}
