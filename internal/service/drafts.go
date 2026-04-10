package service

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"

	"arxivagent/internal/config"
	"arxivagent/internal/model"
	"arxivagent/internal/repository"
)

const (
	ReviewStatusDraft     = "DRAFT"
	ReviewStatusReviewing = "REVIEWING"
	ReviewStatusApproved  = "APPROVED"
	ReviewStatusRejected  = "REJECTED"
)

type DraftService struct {
	repo    *repository.DraftRepository
	siteCfg config.SiteConfig
}

type ListDraftsInput struct {
	Page         int
	PageSize     int
	ReviewStatus string
}

type ListDraftsResult struct {
	Items []model.DraftListItem
	Total int64
}

type UpdateDraftInput struct {
	ID              int64
	Title           string
	Summary         string
	IntroText       string
	MarkdownContent string
	CoverText       string
	Tags            []string
	ReviewComment   string
}

type UpdateDraftStatusInput struct {
	ID            int64
	ReviewStatus  string
	ReviewComment string
}

func (s *DraftService) List(ctx context.Context, input ListDraftsInput) (*ListDraftsResult, error) {
	items, total, err := s.repo.List(ctx, input.Page, input.PageSize, input.ReviewStatus)
	if err != nil {
		return nil, err
	}
	return &ListDraftsResult{Items: items, Total: total}, nil
}

func (s *DraftService) GetDetail(ctx context.Context, id int64) (*model.DraftDetail, error) {
	detail, err := s.repo.GetDetail(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return detail, nil
}

func (s *DraftService) Update(ctx context.Context, input UpdateDraftInput) (*model.DraftDetail, error) {
	if strings.TrimSpace(input.Title) == "" || strings.TrimSpace(input.MarkdownContent) == "" {
		return nil, ErrValidation
	}

	detail, err := s.GetDetail(ctx, input.ID)
	if err != nil {
		return nil, err
	}
	if detail.ReviewStatus == ReviewStatusApproved {
		return nil, ErrInvalidState
	}

	detail.Title = input.Title
	detail.Summary = stringPtrIfPresent(input.Summary)
	detail.IntroText = stringPtrIfPresent(input.IntroText)
	detail.MarkdownContent = input.MarkdownContent
	detail.CoverText = stringPtrIfPresent(input.CoverText)
	detail.Tags = input.Tags
	detail.ReviewComment = stringPtrIfPresent(input.ReviewComment)

	slug := buildSlug(input.Title)
	detail.SiteSlug = &slug
	path := strings.TrimRight(s.siteCfg.DraftPathPrefix, "/") + "/" + slug
	detail.SitePath = &path

	if err := s.repo.Update(ctx, detail); err != nil {
		return nil, err
	}
	return s.GetDetail(ctx, input.ID)
}

func (s *DraftService) UpdateStatus(ctx context.Context, input UpdateDraftStatusInput) (*model.DraftDetail, error) {
	detail, err := s.GetDetail(ctx, input.ID)
	if err != nil {
		return nil, err
	}

	switch input.ReviewStatus {
	case ReviewStatusApproved, ReviewStatusRejected:
		if detail.ReviewStatus != ReviewStatusDraft && detail.ReviewStatus != ReviewStatusReviewing {
			return nil, ErrInvalidState
		}
	default:
		return nil, ErrValidation
	}

	comment := stringPtrIfPresent(input.ReviewComment)
	if err := s.repo.UpdateStatus(ctx, input.ID, input.ReviewStatus, comment, nil); err != nil {
		return nil, err
	}
	return s.GetDetail(ctx, input.ID)
}

func (s *DraftService) Render(ctx context.Context, id int64) (*model.DraftDetail, error) {
	detail, err := s.GetDetail(ctx, id)
	if err != nil {
		return nil, err
	}

	rendered := renderMarkdown(detail.MarkdownContent)
	if err := s.repo.UpdateStatus(ctx, id, detail.ReviewStatus, detail.ReviewComment, &rendered); err != nil {
		return nil, err
	}
	return s.GetDetail(ctx, id)
}
