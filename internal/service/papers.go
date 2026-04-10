package service

import (
	"context"

	"github.com/jackc/pgx/v5"

	"arxivagent/internal/model"
	"arxivagent/internal/repository"
)

type PaperService struct {
	repo *repository.PaperRepository
}

type ListPapersInput struct {
	Page          int
	PageSize      int
	Status        string
	RecommendedOn string
}

type ListPapersResult struct {
	Items []model.PaperListItem
	Total int64
}

func (s *PaperService) List(ctx context.Context, input ListPapersInput) (*ListPapersResult, error) {
	items, total, err := s.repo.List(ctx, input.Page, input.PageSize, input.Status, input.RecommendedOn)
	if err != nil {
		return nil, err
	}
	return &ListPapersResult{Items: items, Total: total}, nil
}

func (s *PaperService) GetDetail(ctx context.Context, id int64) (*model.PaperDetail, error) {
	detail, err := s.repo.GetDetail(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return detail, nil
}
