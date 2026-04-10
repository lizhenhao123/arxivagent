package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"arxivagent/internal/model"
	"arxivagent/internal/repository"
)

type SiteService struct {
	repo *repository.SiteRepository
}

func (s *SiteService) TodayDrafts(ctx context.Context) ([]model.SiteDraft, error) {
	return s.repo.TodayDrafts(ctx)
}

func (s *SiteService) GetBySlug(ctx context.Context, slug string) (*model.SitePost, error) {
	post, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return post, nil
}

func (s *SiteService) GetAssetBinary(ctx context.Context, id int64) ([]byte, string, error) {
	data, contentType, err := s.repo.GetAssetBinary(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, "", ErrNotFound
		}
		return nil, "", err
	}
	if len(data) == 0 {
		return nil, "", fmt.Errorf("asset binary not found")
	}
	return data, contentType, nil
}
