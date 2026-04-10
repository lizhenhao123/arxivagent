package service

import (
	"context"

	"arxivagent/internal/model"
	"arxivagent/internal/repository"
)

type ConfigService struct {
	repo *repository.ConfigRepository
}

func (s *ConfigService) List(ctx context.Context) ([]model.SystemConfig, error) {
	return s.repo.List(ctx)
}
