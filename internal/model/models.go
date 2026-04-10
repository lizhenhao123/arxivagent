package model

import "time"

type PaperListItem struct {
	ID              int64     `json:"id"`
	ArxivID         string    `json:"arxiv_id"`
	Title           string    `json:"title"`
	PrimaryCategory string    `json:"primary_category"`
	PaperStatus     string    `json:"paper_status"`
	IsRecommended   bool      `json:"is_recommended"`
	RecommendedOn   *string   `json:"recommended_on,omitempty"`
	SourceUpdatedAt time.Time `json:"source_updated_at"`
}

type PaperScore struct {
	ID                   int64     `json:"id"`
	ScoreDate            string    `json:"score_date"`
	TopicScore           int       `json:"topic_score"`
	FoundationModelScore int       `json:"foundation_model_score"`
	NoveltyScore         int       `json:"novelty_score"`
	PracticalityScore    int       `json:"practicality_score"`
	EvidenceScore        int       `json:"evidence_score"`
	TotalScore           int       `json:"total_score"`
	Recommendation       string    `json:"recommendation"`
	CreatedAt            time.Time `json:"created_at"`
}

type PaperAsset struct {
	ID                 int64   `json:"id"`
	AssetType          string  `json:"asset_type"`
	AssetRole          string  `json:"asset_role"`
	FileName           *string `json:"file_name,omitempty"`
	MimeType           *string `json:"mime_type,omitempty"`
	PageNo             *int    `json:"page_no,omitempty"`
	FigureIndex        *int    `json:"figure_index,omitempty"`
	Caption            *string `json:"caption,omitempty"`
	DisplayOrder       *int    `json:"display_order,omitempty"`
	IsExperimentFigure bool    `json:"is_experiment_figure"`
}

type PaperContent struct {
	ID                int64   `json:"id"`
	ParseStatus       string  `json:"parse_status"`
	PDFLocalPath      *string `json:"pdf_local_path,omitempty"`
	PDFFileName       *string `json:"pdf_file_name,omitempty"`
	PDFPageCount      *int    `json:"pdf_page_count,omitempty"`
	AbstractCN        *string `json:"abstract_cn,omitempty"`
	InnovationsCN     *string `json:"innovations_cn,omitempty"`
	MethodsCN         *string `json:"methods_cn,omitempty"`
	ExperimentsCN     *string `json:"experiments_cn,omitempty"`
	ConclusionCN      *string `json:"conclusion_cn,omitempty"`
	LimitationsCN     *string `json:"limitations_cn,omitempty"`
	StructuredSummary []byte  `json:"structured_summary"`
}

type PaperDetail struct {
	ID              int64         `json:"id"`
	ArxivID         string        `json:"arxiv_id"`
	Title           string        `json:"title"`
	Abstract        string        `json:"abstract"`
	PrimaryCategory string        `json:"primary_category"`
	PaperStatus     string        `json:"paper_status"`
	IsRecommended   bool          `json:"is_recommended"`
	PDFURL          string        `json:"pdf_url"`
	SourceURL       string        `json:"source_url"`
	Scores          []PaperScore  `json:"scores"`
	Content         *PaperContent `json:"content,omitempty"`
	Assets          []PaperAsset  `json:"assets"`
}

type DraftListItem struct {
	ID           int64     `json:"id"`
	PaperID      int64     `json:"paper_id"`
	Title        string    `json:"title"`
	ReviewStatus string    `json:"review_status"`
	SiteSlug     *string   `json:"site_slug,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type DraftDetail struct {
	ID              int64        `json:"id"`
	PaperID         int64        `json:"paper_id"`
	DraftVersion    int          `json:"draft_version"`
	Title           string       `json:"title"`
	AltTitles       []string     `json:"alt_titles"`
	Summary         *string      `json:"summary,omitempty"`
	IntroText       *string      `json:"intro_text,omitempty"`
	MarkdownContent string       `json:"markdown_content"`
	RenderedHTML    *string      `json:"rendered_html,omitempty"`
	CoverText       *string      `json:"cover_text,omitempty"`
	Tags            []string     `json:"tags"`
	ReviewStatus    string       `json:"review_status"`
	ReviewComment   *string      `json:"review_comment,omitempty"`
	SiteSlug        *string      `json:"site_slug,omitempty"`
	SitePath        *string      `json:"site_path,omitempty"`
	UpdatedAt       time.Time    `json:"updated_at"`
	Assets          []PaperAsset `json:"assets"`
}

type SiteDraft struct {
	DraftID       int64     `json:"draft_id"`
	Title         string    `json:"title"`
	Summary       *string   `json:"summary,omitempty"`
	SiteSlug      *string   `json:"site_slug,omitempty"`
	SitePath      *string   `json:"site_path,omitempty"`
	ReviewStatus  string    `json:"review_status"`
	RecommendedOn *string   `json:"recommended_on,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type SitePost struct {
	DraftID         int64        `json:"draft_id"`
	PaperID         int64        `json:"paper_id"`
	ArxivID         string       `json:"arxiv_id"`
	Title           string       `json:"title"`
	Summary         *string      `json:"summary,omitempty"`
	MarkdownContent string       `json:"markdown_content"`
	RenderedHTML    *string      `json:"rendered_html,omitempty"`
	Tags            []string     `json:"tags"`
	ReviewStatus    string       `json:"review_status"`
	SourceURL       string       `json:"source_url"`
	Assets          []PaperAsset `json:"assets"`
}

type SystemConfig struct {
	Key         string    `json:"config_key"`
	Value       []byte    `json:"config_value"`
	Description *string   `json:"description,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TaskRun struct {
	ID            int64      `json:"id"`
	TaskType      string     `json:"task_type"`
	BizDate       string     `json:"biz_date"`
	Status        string     `json:"status"`
	TriggerSource string     `json:"trigger_source"`
	StartedAt     time.Time  `json:"started_at"`
	EndedAt       *time.Time `json:"ended_at,omitempty"`
	ErrorMessage  *string    `json:"error_message,omitempty"`
}
