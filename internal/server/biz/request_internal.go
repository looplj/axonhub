package biz

import (
	"context"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
)

// getDataStorage is a host read used only by request-body loaders, after the caller has been
// authorized for the request itself.
func (s *RequestService) getDataStorage(ctx context.Context, dataStorageID int) (*ent.DataStorage, error) {
	return authz.RunAsHost(ctx, s.entFromContext(ctx), "request-get-datastorage", func(hostCtx context.Context) (*ent.DataStorage, error) {
		return s.lookupDataStorage(hostCtx, dataStorageID)
	})
}

func (s *RequestService) lookupDataStorage(ctx context.Context, dataStorageID int) (*ent.DataStorage, error) {
	if dataStorageID == 0 {
		return s.DataStorageService.GetPrimaryDataStorage(ctx)
	}

	return s.DataStorageService.GetDataStorageByID(ctx, dataStorageID)
}
