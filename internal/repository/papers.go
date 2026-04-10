package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"arxivagent/internal/model"
)

type PaperRepository struct {
	pool *pgxpool.Pool
}

func (r *PaperRepository) List(ctx context.Context, page, pageSize int, status, recommendedOn string) ([]model.PaperListItem, int64, error) {
	where := "WHERE 1=1"
	args := make([]interface{}, 0, 4)
	argPos := 1

	if status != "" {
		where += fmt.Sprintf(" AND paper_status = $%d", argPos)
		args = append(args, status)
		argPos++
	}
	if recommendedOn != "" {
		where += fmt.Sprintf(" AND recommended_on = $%d", argPos)
		args = append(args, recommendedOn)
		argPos++
	}

	var total int64
	countSQL := "SELECT COUNT(1) FROM papers " + where
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)
	limitPos := argPos
	offsetPos := argPos + 1

	sql := fmt.Sprintf(`
		SELECT id, arxiv_id, title, primary_category, paper_status, is_recommended, recommended_on::text, source_updated_at
		FROM papers
		%s
		ORDER BY source_updated_at DESC
		LIMIT $%d OFFSET $%d
	`, where, limitPos, offsetPos)

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.PaperListItem
	for rows.Next() {
		var item model.PaperListItem
		if err := rows.Scan(&item.ID, &item.ArxivID, &item.Title, &item.PrimaryCategory, &item.PaperStatus, &item.IsRecommended, &item.RecommendedOn, &item.SourceUpdatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}

	return items, total, rows.Err()
}

func (r *PaperRepository) GetDetail(ctx context.Context, id int64) (*model.PaperDetail, error) {
	var detail model.PaperDetail
	err := r.pool.QueryRow(ctx, `
		SELECT id, arxiv_id, title, abstract, primary_category, paper_status, is_recommended, pdf_url, source_url
		FROM papers
		WHERE id = $1
	`, id).Scan(&detail.ID, &detail.ArxivID, &detail.Title, &detail.Abstract, &detail.PrimaryCategory, &detail.PaperStatus, &detail.IsRecommended, &detail.PDFURL, &detail.SourceURL)
	if err != nil {
		return nil, err
	}

	scores, err := r.listScores(ctx, id)
	if err != nil {
		return nil, err
	}
	detail.Scores = scores

	content, err := r.getContent(ctx, id)
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	if err == nil {
		detail.Content = content
	}

	assets, err := r.listAssets(ctx, id)
	if err != nil {
		return nil, err
	}
	detail.Assets = assets
	return &detail, nil
}

func (r *PaperRepository) listScores(ctx context.Context, paperID int64) ([]model.PaperScore, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, score_date::text, topic_score, foundation_model_score, novelty_score, practicality_score, evidence_score, total_score, recommendation, created_at
		FROM paper_scores
		WHERE paper_id = $1
		ORDER BY created_at DESC
	`, paperID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.PaperScore
	for rows.Next() {
		var item model.PaperScore
		if err := rows.Scan(&item.ID, &item.ScoreDate, &item.TopicScore, &item.FoundationModelScore, &item.NoveltyScore, &item.PracticalityScore, &item.EvidenceScore, &item.TotalScore, &item.Recommendation, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PaperRepository) getContent(ctx context.Context, paperID int64) (*model.PaperContent, error) {
	var item model.PaperContent
	var summaryRaw []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, parse_status, pdf_local_path, pdf_file_name, pdf_page_count, abstract_cn, innovations_cn, methods_cn, experiments_cn, conclusion_cn, limitations_cn, structured_summary
		FROM paper_contents
		WHERE paper_id = $1
	`, paperID).Scan(&item.ID, &item.ParseStatus, &item.PDFLocalPath, &item.PDFFileName, &item.PDFPageCount, &item.AbstractCN, &item.InnovationsCN, &item.MethodsCN, &item.ExperimentsCN, &item.ConclusionCN, &item.LimitationsCN, &summaryRaw)
	if err != nil {
		return nil, err
	}
	item.StructuredSummary = normalizeJSON(summaryRaw)
	return &item, nil
}

func (r *PaperRepository) listAssets(ctx context.Context, paperID int64) ([]model.PaperAsset, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, asset_type, asset_role, file_name, mime_type, page_no, figure_index, caption, display_order, is_experiment_figure
		FROM paper_assets
		WHERE paper_id = $1
		ORDER BY asset_type ASC, display_order ASC NULLS LAST, figure_index ASC NULLS LAST, id ASC
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

func normalizeJSON(raw []byte) []byte {
	if len(raw) == 0 {
		return []byte("{}")
	}
	var tmp interface{}
	if err := json.Unmarshal(raw, &tmp); err != nil {
		return raw
	}
	b, err := json.Marshal(tmp)
	if err != nil {
		return raw
	}
	return b
}
