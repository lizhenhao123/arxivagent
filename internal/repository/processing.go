package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProcessingRepository struct {
	pool *pgxpool.Pool
}

type PromptTemplate struct {
	TemplateContent string
	Version         string
}

type ProcessingPaper struct {
	ID        int64
	ArxivID   string
	Title     string
	Abstract  string
	PDFURL    string
	SourceURL string
}

type UpsertContentInput struct {
	PaperID             int64
	ParseStatus         string
	PDFLocalPath        string
	PDFFileName         string
	PDFSizeBytes        int64
	PDFSHA256           string
	PDFPageCount        int
	PDFMetadata         interface{}
	SectionOutline      interface{}
	ParsedSections      interface{}
	AbstractCN          string
	InnovationsCN       string
	MethodsCN           string
	ExperimentsCN       string
	ConclusionCN        string
	LimitationsCN       string
	StructuredSummary   interface{}
	RawParserOutput     interface{}
	RawGenerationOutput interface{}
	ParserVersion       string
	PromptVersion       string
	ErrorMessage        *string
}

type AssetRecordInput struct {
	PaperID            int64
	AssetType          string
	AssetRole          string
	SourceURL          *string
	LocalPath          *string
	FileName           *string
	MimeType           *string
	SizeBytes          *int64
	SHA256             *string
	PageNo             *int
	FigureIndex        *int
	Caption            *string
	Width              *int
	Height             *int
	DisplayOrder       *int
	IsExperimentFigure bool
	BinaryData         []byte
	ExtraMetadata      interface{}
}

func NewProcessingRepository(pool *pgxpool.Pool) *ProcessingRepository {
	return &ProcessingRepository{pool: pool}
}

func (r *ProcessingRepository) GetPaperForProcessing(ctx context.Context, paperID int64) (*ProcessingPaper, error) {
	var item ProcessingPaper
	err := r.pool.QueryRow(ctx, `
		SELECT id, arxiv_id, title, abstract, pdf_url, source_url
		FROM papers
		WHERE id = $1
	`, paperID).Scan(&item.ID, &item.ArxivID, &item.Title, &item.Abstract, &item.PDFURL, &item.SourceURL)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *ProcessingRepository) ListRecommendedPaperIDs(ctx context.Context, bizDate time.Time) ([]int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id
		FROM papers
		WHERE recommended_on = $1 AND is_recommended = TRUE
		ORDER BY id ASC
	`, bizDate.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *ProcessingRepository) UpsertPaperContent(ctx context.Context, input UpsertContentInput) (int64, error) {
	pdfMetadata, _ := json.Marshal(sanitizeJSONValue(input.PDFMetadata))
	outline, _ := json.Marshal(sanitizeJSONValue(input.SectionOutline))
	sections, _ := json.Marshal(sanitizeJSONValue(input.ParsedSections))
	summary, _ := json.Marshal(sanitizeJSONValue(input.StructuredSummary))
	rawParser, _ := json.Marshal(sanitizeJSONValue(input.RawParserOutput))
	rawGen, _ := json.Marshal(sanitizeJSONValue(input.RawGenerationOutput))

	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO paper_contents (
			paper_id, parse_status, pdf_local_path, pdf_file_name, pdf_size_bytes, pdf_sha256, pdf_page_count,
			pdf_metadata, section_outline, parsed_sections,
			abstract_cn, innovations_cn, methods_cn, experiments_cn, conclusion_cn, limitations_cn,
			structured_summary, raw_parser_output, raw_generation_output,
			parser_version, prompt_version, error_message
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10,
			$11, $12, $13, $14, $15, $16,
			$17, $18, $19,
			$20, $21, $22
		)
		ON CONFLICT (paper_id)
		DO UPDATE SET
			parse_status = EXCLUDED.parse_status,
			pdf_local_path = EXCLUDED.pdf_local_path,
			pdf_file_name = EXCLUDED.pdf_file_name,
			pdf_size_bytes = EXCLUDED.pdf_size_bytes,
			pdf_sha256 = EXCLUDED.pdf_sha256,
			pdf_page_count = EXCLUDED.pdf_page_count,
			pdf_metadata = EXCLUDED.pdf_metadata,
			section_outline = EXCLUDED.section_outline,
			parsed_sections = EXCLUDED.parsed_sections,
			abstract_cn = EXCLUDED.abstract_cn,
			innovations_cn = EXCLUDED.innovations_cn,
			methods_cn = EXCLUDED.methods_cn,
			experiments_cn = EXCLUDED.experiments_cn,
			conclusion_cn = EXCLUDED.conclusion_cn,
			limitations_cn = EXCLUDED.limitations_cn,
			structured_summary = EXCLUDED.structured_summary,
			raw_parser_output = EXCLUDED.raw_parser_output,
			raw_generation_output = EXCLUDED.raw_generation_output,
			parser_version = EXCLUDED.parser_version,
			prompt_version = EXCLUDED.prompt_version,
			error_message = EXCLUDED.error_message,
			updated_at = NOW()
		RETURNING id
	`, input.PaperID, input.ParseStatus, input.PDFLocalPath, input.PDFFileName, input.PDFSizeBytes, input.PDFSHA256, input.PDFPageCount,
		pdfMetadata, outline, sections,
		input.AbstractCN, input.InnovationsCN, input.MethodsCN, input.ExperimentsCN, input.ConclusionCN, input.LimitationsCN,
		summary, rawParser, rawGen,
		input.ParserVersion, input.PromptVersion, input.ErrorMessage).Scan(&id)
	if err != nil {
		return 0, err
	}

	_, err = r.pool.Exec(ctx, `
		UPDATE papers
		SET last_content_id = $2,
			paper_status = CASE WHEN $3 = 'PARSED' THEN 'CONTENT_GENERATED' ELSE paper_status END
		WHERE id = $1
	`, input.PaperID, id, input.ParseStatus)
	return id, err
}

func (r *ProcessingRepository) ReplaceAssets(ctx context.Context, paperID int64, assets []AssetRecordInput) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `DELETE FROM paper_assets WHERE paper_id = $1`, paperID)
	if err != nil {
		return err
	}

	for _, asset := range assets {
		extra, _ := json.Marshal(sanitizeJSONValue(asset.ExtraMetadata))
		_, err = tx.Exec(ctx, `
			INSERT INTO paper_assets (
				paper_id, asset_type, asset_role, source_url, local_path, file_name, mime_type,
				size_bytes, sha256, page_no, figure_index, caption, width, height,
				display_order, is_experiment_figure, binary_data, extra_metadata
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7,
				$8, $9, $10, $11, $12, $13, $14,
				$15, $16, $17, $18
			)
		`, asset.PaperID, asset.AssetType, asset.AssetRole, asset.SourceURL, asset.LocalPath, asset.FileName, asset.MimeType,
			asset.SizeBytes, asset.SHA256, asset.PageNo, asset.FigureIndex, asset.Caption, asset.Width, asset.Height,
			asset.DisplayOrder, asset.IsExperimentFigure, asset.BinaryData, extra)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *ProcessingRepository) UpsertGeneratedDraft(ctx context.Context, paperID, sourceContentID int64, title string, altTitles []string, summary, introText, markdownContent, renderedHTML, coverText string, tags []string, templateVersion, promptVersion, siteSlug, sitePath string) (int64, error) {
	altTitlesJSON, _ := json.Marshal(sanitizeJSONValue(altTitles))
	tagsJSON, _ := json.Marshal(sanitizeJSONValue(tags))
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO article_drafts (
			paper_id, draft_version, is_primary, title, alt_titles, summary, intro_text, markdown_content,
			rendered_html, cover_text, tags, review_status, template_version, prompt_version,
			source_content_id, site_slug, site_path
		) VALUES (
			$1, 1, TRUE, $2, $3, $4, $5, $6,
			$7, $8, $9, 'DRAFT', $10, $11,
			$12, $13, $14
		)
		ON CONFLICT (paper_id, draft_version)
		DO UPDATE SET
			title = EXCLUDED.title,
			alt_titles = EXCLUDED.alt_titles,
			summary = EXCLUDED.summary,
			intro_text = EXCLUDED.intro_text,
			markdown_content = EXCLUDED.markdown_content,
			rendered_html = EXCLUDED.rendered_html,
			cover_text = EXCLUDED.cover_text,
			tags = EXCLUDED.tags,
			template_version = EXCLUDED.template_version,
			prompt_version = EXCLUDED.prompt_version,
			source_content_id = EXCLUDED.source_content_id,
			site_slug = EXCLUDED.site_slug,
			site_path = EXCLUDED.site_path,
			updated_at = NOW()
		RETURNING id
	`, paperID, title, altTitlesJSON, summary, introText, markdownContent, renderedHTML, coverText, tagsJSON, templateVersion, promptVersion, sourceContentID, siteSlug, sitePath).Scan(&id)
	if err != nil {
		return 0, err
	}

	_, err = r.pool.Exec(ctx, `
		UPDATE papers
		SET last_draft_id = $2,
			paper_status = 'DRAFT_READY'
		WHERE id = $1
	`, paperID, id)
	return id, err
}

func (r *ProcessingRepository) GetActivePromptTemplate(ctx context.Context, templateType string) (*PromptTemplate, error) {
	var item PromptTemplate
	err := r.pool.QueryRow(ctx, `
		SELECT template_content, version
		FROM prompt_templates
		WHERE template_type = $1 AND is_active = TRUE
		ORDER BY updated_at DESC, id DESC
		LIMIT 1
	`, templateType).Scan(&item.TemplateContent, &item.Version)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

var _ = pgx.ErrNoRows
