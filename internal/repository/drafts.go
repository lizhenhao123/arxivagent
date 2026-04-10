package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"arxivagent/internal/model"
)

type DraftRepository struct {
	pool *pgxpool.Pool
}

func (r *DraftRepository) List(ctx context.Context, page, pageSize int, reviewStatus string) ([]model.DraftListItem, int64, error) {
	where := "WHERE 1=1"
	args := make([]interface{}, 0, 3)
	argPos := 1

	if reviewStatus != "" {
		where += fmt.Sprintf(" AND review_status = $%d", argPos)
		args = append(args, reviewStatus)
		argPos++
	}

	var total int64
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(1) FROM article_drafts "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)

	sql := fmt.Sprintf(`
		SELECT id, paper_id, title, review_status, site_slug, updated_at
		FROM article_drafts
		%s
		ORDER BY updated_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argPos, argPos+1)

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.DraftListItem
	for rows.Next() {
		var item model.DraftListItem
		if err := rows.Scan(&item.ID, &item.PaperID, &item.Title, &item.ReviewStatus, &item.SiteSlug, &item.UpdatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}

	return items, total, rows.Err()
}

func (r *DraftRepository) GetDetail(ctx context.Context, id int64) (*model.DraftDetail, error) {
	var detail model.DraftDetail
	var altTitlesRaw []byte
	var tagsRaw []byte

	err := r.pool.QueryRow(ctx, `
		SELECT id, paper_id, draft_version, title, alt_titles, summary, intro_text, markdown_content, rendered_html, cover_text, tags, review_status, review_comment, site_slug, site_path, updated_at
		FROM article_drafts
		WHERE id = $1
	`, id).Scan(
		&detail.ID,
		&detail.PaperID,
		&detail.DraftVersion,
		&detail.Title,
		&altTitlesRaw,
		&detail.Summary,
		&detail.IntroText,
		&detail.MarkdownContent,
		&detail.RenderedHTML,
		&detail.CoverText,
		&tagsRaw,
		&detail.ReviewStatus,
		&detail.ReviewComment,
		&detail.SiteSlug,
		&detail.SitePath,
		&detail.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(altTitlesRaw, &detail.AltTitles)
	_ = json.Unmarshal(tagsRaw, &detail.Tags)

	assets, err := r.paperAssets(ctx, detail.PaperID)
	if err != nil {
		return nil, err
	}
	detail.Assets = assets
	return &detail, nil
}

func (r *DraftRepository) Update(ctx context.Context, detail *model.DraftDetail) error {
	tagsRaw, err := json.Marshal(detail.Tags)
	if err != nil {
		return err
	}

	_, err = r.pool.Exec(ctx, `
		UPDATE article_drafts
		SET title = $2,
			summary = $3,
			intro_text = $4,
			markdown_content = $5,
			cover_text = $6,
			tags = $7,
			review_comment = $8,
			site_slug = $9,
			site_path = $10
		WHERE id = $1
	`, detail.ID, detail.Title, detail.Summary, detail.IntroText, detail.MarkdownContent, detail.CoverText, tagsRaw, detail.ReviewComment, detail.SiteSlug, detail.SitePath)
	return err
}

func (r *DraftRepository) UpdateStatus(ctx context.Context, id int64, status string, reviewComment *string, renderedHTML *string) error {
	if renderedHTML == nil {
		_, err := r.pool.Exec(ctx, `
			UPDATE article_drafts
			SET review_status = $2,
				review_comment = COALESCE($3, review_comment),
				approved_at = CASE WHEN $2 = 'APPROVED' THEN NOW() ELSE approved_at END
			WHERE id = $1
		`, id, status, reviewComment)
		return err
	}

	_, err := r.pool.Exec(ctx, `
		UPDATE article_drafts
		SET review_status = $2,
			review_comment = COALESCE($3, review_comment),
			rendered_html = $4,
			approved_at = CASE WHEN $2 = 'APPROVED' THEN NOW() ELSE approved_at END
		WHERE id = $1
	`, id, status, reviewComment, renderedHTML)
	return err
}

func (r *DraftRepository) paperAssets(ctx context.Context, paperID int64) ([]model.PaperAsset, error) {
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
