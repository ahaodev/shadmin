package usecase

import (
	"context"
	"shadmin/domain"
	"time"
)

type apiResourceUsecase struct {
	apiResourceRepository domain.ApiResourceRepository
	contextTimeout        time.Duration
}

func NewApiResourceUsecase(apiResourceRepository domain.ApiResourceRepository, timeout time.Duration) domain.ApiResourceUseCase {
	return &apiResourceUsecase{
		apiResourceRepository: apiResourceRepository,
		contextTimeout:        timeout,
	}
}

func (aru *apiResourceUsecase) FetchPaged(ctx context.Context, params domain.ApiResourceQueryParams) (*domain.ApiResourcePagedResult, error) {
	ctx, cancel := context.WithTimeout(ctx, aru.contextTimeout)
	defer cancel()

	return aru.apiResourceRepository.FetchPaged(ctx, params)
}
