package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DiscoveryRepository struct {
	pool *pgxpool.Pool
}

type UpsertPaperInput struct {
	ArxivID         string
	VersionNo       int
	Title           string
	Authors         interface{}
	Abstract        string
	PrimaryCategory string
	Categories      interface{}
	PublishedAt     time.Time
	UpdatedAt       time.Time
	PDFURL          string
	SourceURL       string
	SourcePayload   interface{}
}

type InsertScoreInput struct {
	PaperID              int64
	ScoreDate            time.Time
	TopicScore           int
	FoundationModelScore int
	NoveltyScore         int
	PracticalityScore    int
	EvidenceScore        int
	TotalScore           int
	Recommendation       string
	ScoreReasons         []string
	RiskNotes            []string
	ScoreDetail          map[string]interface{}
	RankInDay            *int
	RuleVersion          string
	ModelName            *string
	PromptVersion        string
	RawLLMResponse       interface{}
}

type RecommendedPaper struct {
	PaperID   int64
	ScoreID   int64
	RankInDay int
}

type ConfigValue struct {
	Key   string
	Value []byte
}

func (r *DiscoveryRepository) UpsertPaperAndVersion(ctx context.Context, input UpsertPaperInput) (int64, error) {
	authorsJSON, err := json.Marshal(input.Authors)
	if err != nil {
		return 0, err
	}
	categoriesJSON, err := json.Marshal(input.Categories)
	if err != nil {
		return 0, err
	}
	sourcePayloadJSON, err := json.Marshal(input.SourcePayload)
	if err != nil {
		return 0, err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var paperID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO papers (
			arxiv_id, latest_version_no, title, authors, abstract, primary_category, categories,
			published_at, source_updated_at, pdf_url, source_url, paper_status
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, 'DISCOVERED'
		)
		ON CONFLICT (arxiv_id)
		DO UPDATE SET
			latest_version_no = GREATEST(papers.latest_version_no, EXCLUDED.latest_version_no),
			title = EXCLUDED.title,
			authors = EXCLUDED.authors,
			abstract = EXCLUDED.abstract,
			primary_category = EXCLUDED.primary_category,
			categories = EXCLUDED.categories,
			source_updated_at = EXCLUDED.source_updated_at,
			pdf_url = EXCLUDED.pdf_url,
			source_url = EXCLUDED.source_url,
			updated_at = NOW()
		RETURNING id
	`, input.ArxivID, input.VersionNo, input.Title, authorsJSON, input.Abstract, input.PrimaryCategory, categoriesJSON, input.PublishedAt, input.UpdatedAt, input.PDFURL, input.SourceURL).Scan(&paperID)
	if err != nil {
		return 0, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO paper_versions (
			paper_id, version_no, title, authors, abstract, primary_category, categories,
			published_at, source_updated_at, pdf_url, source_payload
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11
		)
		ON CONFLICT (paper_id, version_no)
		DO UPDATE SET
			title = EXCLUDED.title,
			authors = EXCLUDED.authors,
			abstract = EXCLUDED.abstract,
			primary_category = EXCLUDED.primary_category,
			categories = EXCLUDED.categories,
			published_at = EXCLUDED.published_at,
			source_updated_at = EXCLUDED.source_updated_at,
			pdf_url = EXCLUDED.pdf_url,
			source_payload = EXCLUDED.source_payload
	`, paperID, input.VersionNo, input.Title, authorsJSON, input.Abstract, input.PrimaryCategory, categoriesJSON, input.PublishedAt, input.UpdatedAt, input.PDFURL, sourcePayloadJSON)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return paperID, nil
}

func (r *DiscoveryRepository) UpdatePaperStatus(ctx context.Context, paperID int64, status string, failureReason *string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE papers
		SET paper_status = $2,
			failure_reason = $3
		WHERE id = $1
	`, paperID, status, failureReason)
	return err
}

func (r *DiscoveryRepository) InsertScore(ctx context.Context, input InsertScoreInput) (int64, error) {
	reasonsJSON, err := json.Marshal(input.ScoreReasons)
	if err != nil {
		return 0, err
	}
	risksJSON, err := json.Marshal(input.RiskNotes)
	if err != nil {
		return 0, err
	}
	detailJSON, err := json.Marshal(input.ScoreDetail)
	if err != nil {
		return 0, err
	}

	var rawLLMJSON []byte
	if input.RawLLMResponse != nil {
		rawLLMJSON, err = json.Marshal(input.RawLLMResponse)
		if err != nil {
			return 0, err
		}
	}

	var scoreID int64
	err = r.pool.QueryRow(ctx, `
		INSERT INTO paper_scores (
			paper_id, score_date, topic_score, foundation_model_score, novelty_score,
			practicality_score, evidence_score, total_score, recommendation,
			score_reasons, risk_notes, score_detail, rank_in_day,
			rule_version, model_name, prompt_version, raw_llm_response
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13,
			$14, $15, $16, $17
		)
		ON CONFLICT (paper_id, score_date, prompt_version)
		DO UPDATE SET
			topic_score = EXCLUDED.topic_score,
			foundation_model_score = EXCLUDED.foundation_model_score,
			novelty_score = EXCLUDED.novelty_score,
			practicality_score = EXCLUDED.practicality_score,
			evidence_score = EXCLUDED.evidence_score,
			total_score = EXCLUDED.total_score,
			recommendation = EXCLUDED.recommendation,
			score_reasons = EXCLUDED.score_reasons,
			risk_notes = EXCLUDED.risk_notes,
			score_detail = EXCLUDED.score_detail,
			rank_in_day = EXCLUDED.rank_in_day,
			rule_version = EXCLUDED.rule_version,
			model_name = EXCLUDED.model_name,
			raw_llm_response = EXCLUDED.raw_llm_response
		RETURNING id
	`, input.PaperID, input.ScoreDate.Format("2006-01-02"), input.TopicScore, input.FoundationModelScore, input.NoveltyScore, input.PracticalityScore, input.EvidenceScore, input.TotalScore, input.Recommendation, reasonsJSON, risksJSON, detailJSON, input.RankInDay, input.RuleVersion, input.ModelName, input.PromptVersion, rawLLMJSON).Scan(&scoreID)
	if err != nil {
		return 0, err
	}

	_, err = r.pool.Exec(ctx, `
		UPDATE papers
		SET paper_status = 'SCORED',
			last_score_id = $2
		WHERE id = $1
	`, input.PaperID, scoreID)
	if err != nil {
		return 0, err
	}

	return scoreID, nil
}

func (r *DiscoveryRepository) ReplaceRecommendations(ctx context.Context, recommendationDate time.Time, items []RecommendedPaper) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		UPDATE papers
		SET is_recommended = FALSE,
			recommended_on = NULL,
			paper_status = CASE WHEN paper_status = 'RECOMMENDED' THEN 'SCORED' ELSE paper_status END
		WHERE recommended_on = $1
	`, recommendationDate.Format("2006-01-02"))
	if err != nil {
		return err
	}

	for idx, item := range items {
		_, err = tx.Exec(ctx, `
			UPDATE paper_scores
			SET rank_in_day = $2
			WHERE id = $1
		`, item.ScoreID, idx+1)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
			UPDATE papers
			SET is_recommended = TRUE,
				recommended_on = $2,
				paper_status = 'RECOMMENDED',
				last_score_id = $3
			WHERE id = $1
		`, item.PaperID, recommendationDate.Format("2006-01-02"), item.ScoreID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *DiscoveryRepository) GetConfigMap(ctx context.Context, keys ...string) (map[string][]byte, error) {
	if len(keys) == 0 {
		return map[string][]byte{}, nil
	}

	placeholders := make([]string, 0, len(keys))
	args := make([]interface{}, 0, len(keys))
	for i, key := range keys {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
		args = append(args, key)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT config_key, config_value
		FROM system_configs
		WHERE config_key IN (`+strings.Join(placeholders, ",")+`)
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]byte, len(keys))
	for rows.Next() {
		var key string
		var value []byte
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, rows.Err()
}

func (r *DiscoveryRepository) CreateDraftPlaceholder(ctx context.Context, paperID int64, title string) (int64, error) {
	var draftID int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO article_drafts (
			paper_id, draft_version, is_primary, title, markdown_content,
			review_status, template_version, prompt_version
		) VALUES (
			$1, 1, TRUE, $2, $3,
			'DRAFT', 'placeholder-v1', 'placeholder-v1'
		)
		ON CONFLICT (paper_id, draft_version)
		DO UPDATE SET
			title = EXCLUDED.title,
			updated_at = NOW()
		RETURNING id
	`, paperID, title, "# "+title+"\n\n待生成正文。").Scan(&draftID)
	if err != nil {
		return 0, err
	}

	_, err = r.pool.Exec(ctx, `
		UPDATE papers
		SET last_draft_id = $2,
			paper_status = 'DRAFT_READY'
		WHERE id = $1
	`, paperID, draftID)
	return draftID, err
}

func (r *DiscoveryRepository) ExistsTaskRun(ctx context.Context, taskType string, bizDate time.Time) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM task_runs
			WHERE task_type = $1 AND biz_date = $2 AND status = 'SUCCESS'
		)
	`, taskType, bizDate.Format("2006-01-02")).Scan(&exists)
	return exists, err
}

func NewDiscoveryRepository(pool *pgxpool.Pool) *DiscoveryRepository {
	return &DiscoveryRepository{pool: pool}
}

var _ = pgx.ErrNoRows
