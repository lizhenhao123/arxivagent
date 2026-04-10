package repository

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"arxivagent/internal/model"
)

type SiteRepository struct {
	pool *pgxpool.Pool
}

func (r *SiteRepository) TodayDrafts(ctx context.Context) ([]model.SiteDraft, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT d.id, d.title, d.summary, d.site_slug, d.site_path, d.review_status, p.recommended_on::text, d.updated_at
		FROM article_drafts d
		JOIN papers p ON p.id = d.paper_id
		WHERE p.recommended_on = CURRENT_DATE
		ORDER BY d.updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.SiteDraft
	for rows.Next() {
		var item model.SiteDraft
		if err := rows.Scan(&item.DraftID, &item.Title, &item.Summary, &item.SiteSlug, &item.SitePath, &item.ReviewStatus, &item.RecommendedOn, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SiteRepository) GetBySlug(ctx context.Context, slug string) (*model.SitePost, error) {
	var post model.SitePost
	var tagsRaw []byte
	err := r.pool.QueryRow(ctx, `
		SELECT d.id, d.paper_id, p.arxiv_id, d.title, d.summary, d.markdown_content, d.rendered_html, d.tags, d.review_status, p.source_url
		FROM article_drafts d
		JOIN papers p ON p.id = d.paper_id
		WHERE d.site_slug = $1
	`, slug).Scan(&post.DraftID, &post.PaperID, &post.ArxivID, &post.Title, &post.Summary, &post.MarkdownContent, &post.RenderedHTML, &tagsRaw, &post.ReviewStatus, &post.SourceURL)
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(tagsRaw, &post.Tags)
	assets, err := r.paperAssets(ctx, post.PaperID)
	if err != nil {
		return nil, err
	}
	post.Assets = assets
	return &post, nil
}

func (r *SiteRepository) GetAssetBinary(ctx context.Context, id int64) ([]byte, string, error) {
	var binary []byte
	var mimeType *string
	err := r.pool.QueryRow(ctx, `
		SELECT binary_data, mime_type
		FROM paper_assets
		WHERE id = $1
	`, id).Scan(&binary, &mimeType)
	if err != nil {
		return nil, "", err
	}

	contentType := "application/octet-stream"
	if mimeType != nil && *mimeType != "" {
		contentType = *mimeType
	}
	return binary, contentType, nil
}

func (r *SiteRepository) paperAssets(ctx context.Context, paperID int64) ([]model.PaperAsset, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, asset_type, asset_role, file_name, mime_type, page_no, figure_index, caption, display_order, is_experiment_figure
		FROM paper_assets
		WHERE paper_id = $1
		ORDER BY is_experiment_figure DESC, display_order ASC NULLS LAST, figure_index ASC NULLS LAST, id ASC
	`, paperID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.PaperAsset
	for rows.Next() {
		var item model.PaperAsset
		if err := rows.Scan(&item.ID, &item.AssetType, &item.AssetRole, &item.FileName, &item.MimeType, &item.PageNo, &item.FigureIndex, &item.Caption, &item.DisplayOrder, &item.IsExperimentFigure); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
