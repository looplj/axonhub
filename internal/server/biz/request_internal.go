package biz

import (
	"context"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
)

func (s *RequestService) getDataStorage(ctx context.Context, dataStorageID int) (*ent.DataStorage, error) {
	ctx, err := authz.WithSystemBypass(ctx, "request-get-datastorage")
	if err != nil {
		return nil, err
	}
	if dataStorageID == 0 {
		return s.DataStorageService.GetPrimaryDataStorage(ctx)
	}

	dataStorage, err := s.DataStorageService.GetDataStorageByID(ctx, dataStorageID)
	if err != nil {
		return nil, err
	}

	return dataStorage, nil
}
