CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE papers (
    id BIGSERIAL PRIMARY KEY,
    arxiv_id VARCHAR(64) NOT NULL UNIQUE,
    latest_version_no INTEGER NOT NULL DEFAULT 1 CHECK (latest_version_no >= 1),
    title TEXT NOT NULL,
    authors JSONB NOT NULL DEFAULT '[]'::jsonb,
    abstract TEXT NOT NULL,
    primary_category VARCHAR(128) NOT NULL,
    categories JSONB NOT NULL DEFAULT '[]'::jsonb,
    published_at TIMESTAMPTZ NOT NULL,
    source_updated_at TIMESTAMPTZ NOT NULL,
    pdf_url TEXT NOT NULL,
    source_url TEXT NOT NULL,
    paper_status VARCHAR(32) NOT NULL CHECK (
        paper_status IN (
            'DISCOVERED',
            'FILTERED',
            'SCORED',
            'RECOMMENDED',
            'PDF_DOWNLOADED',
            'PARSED',
            'CONTENT_GENERATED',
            'DRAFT_READY',
            'REVIEWING',
            'APPROVED',
            'ARCHIVED',
            'PUBLISH_PENDING',
            'PUBLISHING',
            'PUBLISHED',
            'FILTER_FAILED',
            'SCORE_FAILED',
            'PARSE_FAILED',
            'GENERATE_FAILED',
            'REVIEW_REJECTED',
            'PUBLISH_FAILED'
        )
    ),
    is_candidate BOOLEAN NOT NULL DEFAULT TRUE,
    is_recommended BOOLEAN NOT NULL DEFAULT FALSE,
    recommended_on DATE,
    last_score_id BIGINT,
    last_content_id BIGINT,
    last_draft_id BIGINT,
    failure_reason TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0 CHECK (retry_count >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_papers_status ON papers (paper_status);
CREATE INDEX idx_papers_recommended_on ON papers (recommended_on);
CREATE INDEX idx_papers_source_updated_at_desc ON papers (source_updated_at DESC);
CREATE INDEX idx_papers_recommended_flag_day ON papers (is_recommended, recommended_on DESC);

CREATE TABLE paper_versions (
    id BIGSERIAL PRIMARY KEY,
    paper_id BIGINT NOT NULL REFERENCES papers(id) ON DELETE CASCADE,
    version_no INTEGER NOT NULL CHECK (version_no >= 1),
    title TEXT NOT NULL,
    authors JSONB NOT NULL DEFAULT '[]'::jsonb,
    abstract TEXT NOT NULL,
    primary_category VARCHAR(128) NOT NULL,
    categories JSONB NOT NULL DEFAULT '[]'::jsonb,
    published_at TIMESTAMPTZ NOT NULL,
    source_updated_at TIMESTAMPTZ NOT NULL,
    pdf_url TEXT NOT NULL,
    source_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_paper_versions_paper_version UNIQUE (paper_id, version_no)
);

CREATE INDEX idx_paper_versions_paper_version_desc ON paper_versions (paper_id, version_no DESC);

CREATE TABLE paper_scores (
    id BIGSERIAL PRIMARY KEY,
    paper_id BIGINT NOT NULL REFERENCES papers(id) ON DELETE CASCADE,
    score_date DATE NOT NULL,
    topic_score INTEGER NOT NULL DEFAULT 0 CHECK (topic_score >= 0),
    foundation_model_score INTEGER NOT NULL DEFAULT 0 CHECK (foundation_model_score >= 0),
    novelty_score INTEGER NOT NULL DEFAULT 0 CHECK (novelty_score >= 0),
    practicality_score INTEGER NOT NULL DEFAULT 0 CHECK (practicality_score >= 0),
    evidence_score INTEGER NOT NULL DEFAULT 0 CHECK (evidence_score >= 0),
    total_score INTEGER NOT NULL DEFAULT 0 CHECK (total_score >= 0),
    recommendation VARCHAR(32) NOT NULL,
    score_reasons JSONB NOT NULL DEFAULT '[]'::jsonb,
    risk_notes JSONB NOT NULL DEFAULT '[]'::jsonb,
    score_detail JSONB NOT NULL DEFAULT '{}'::jsonb,
    rank_in_day INTEGER,
    rule_version VARCHAR(64) NOT NULL,
    model_name VARCHAR(128),
    prompt_version VARCHAR(64) NOT NULL,
    raw_llm_response JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_paper_scores_daily UNIQUE (paper_id, score_date, prompt_version)
);

CREATE INDEX idx_paper_scores_date_total_desc ON paper_scores (score_date, total_score DESC);
CREATE INDEX idx_paper_scores_paper_created_desc ON paper_scores (paper_id, created_at DESC);

CREATE TABLE paper_contents (
    id BIGSERIAL PRIMARY KEY,
    paper_id BIGINT NOT NULL UNIQUE REFERENCES papers(id) ON DELETE CASCADE,
    parse_status VARCHAR(32) NOT NULL CHECK (
        parse_status IN ('PENDING', 'DOWNLOADING', 'DOWNLOADED', 'PARSING', 'PARSED', 'FAILED')
    ),
    pdf_local_path TEXT,
    pdf_file_name VARCHAR(512),
    pdf_size_bytes BIGINT,
    pdf_sha256 VARCHAR(64),
    pdf_page_count INTEGER,
    pdf_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    section_outline JSONB NOT NULL DEFAULT '[]'::jsonb,
    parsed_sections JSONB NOT NULL DEFAULT '{}'::jsonb,
    abstract_cn TEXT,
    innovations_cn TEXT,
    methods_cn TEXT,
    experiments_cn TEXT,
    conclusion_cn TEXT,
    limitations_cn TEXT,
    structured_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    raw_parser_output JSONB NOT NULL DEFAULT '{}'::jsonb,
    raw_generation_output JSONB NOT NULL DEFAULT '{}'::jsonb,
    parser_version VARCHAR(64) NOT NULL,
    prompt_version VARCHAR(64) NOT NULL,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_paper_contents_parse_status ON paper_contents (parse_status);

CREATE TABLE paper_assets (
    id BIGSERIAL PRIMARY KEY,
    paper_id BIGINT NOT NULL REFERENCES papers(id) ON DELETE CASCADE,
    asset_type VARCHAR(16) NOT NULL CHECK (asset_type IN ('PDF', 'FIGURE')),
    asset_role VARCHAR(64) NOT NULL,
    source_url TEXT,
    local_path TEXT,
    file_name VARCHAR(512),
    mime_type VARCHAR(128),
    size_bytes BIGINT,
    sha256 VARCHAR(64),
    page_no INTEGER,
    figure_index INTEGER,
    caption TEXT,
    width INTEGER,
    height INTEGER,
    display_order INTEGER,
    is_experiment_figure BOOLEAN NOT NULL DEFAULT FALSE,
    binary_data BYTEA,
    extra_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_paper_assets_role UNIQUE (paper_id, asset_role)
);

CREATE UNIQUE INDEX uq_paper_assets_figure_index
    ON paper_assets (paper_id, figure_index)
    WHERE figure_index IS NOT NULL;

CREATE INDEX idx_paper_assets_paper_type ON paper_assets (paper_id, asset_type);
CREATE INDEX idx_paper_assets_experiment_order ON paper_assets (paper_id, is_experiment_figure, display_order);

CREATE TABLE article_drafts (
    id BIGSERIAL PRIMARY KEY,
    paper_id BIGINT NOT NULL REFERENCES papers(id) ON DELETE CASCADE,
    draft_version INTEGER NOT NULL DEFAULT 1 CHECK (draft_version >= 1),
    is_primary BOOLEAN NOT NULL DEFAULT TRUE,
    title TEXT NOT NULL,
    alt_titles JSONB NOT NULL DEFAULT '[]'::jsonb,
    summary TEXT,
    intro_text TEXT,
    markdown_content TEXT NOT NULL,
    rendered_html TEXT,
    cover_text TEXT,
    tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    review_status VARCHAR(32) NOT NULL DEFAULT 'DRAFT' CHECK (
        review_status IN ('DRAFT', 'REVIEWING', 'APPROVED', 'REJECTED')
    ),
    reviewer_id BIGINT,
    review_comment TEXT,
    approved_at TIMESTAMPTZ,
    template_version VARCHAR(64) NOT NULL,
    prompt_version VARCHAR(64) NOT NULL,
    source_content_id BIGINT REFERENCES paper_contents(id) ON DELETE SET NULL,
    site_slug VARCHAR(256),
    site_path TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_article_drafts_paper_version UNIQUE (paper_id, draft_version)
);

CREATE UNIQUE INDEX uq_article_drafts_site_slug
    ON article_drafts (site_slug)
    WHERE site_slug IS NOT NULL;

CREATE INDEX idx_article_drafts_paper_created_desc ON article_drafts (paper_id, created_at DESC);
CREATE INDEX idx_article_drafts_review_status_created_desc ON article_drafts (review_status, created_at DESC);

CREATE TABLE prompt_templates (
    id BIGSERIAL PRIMARY KEY,
    template_type VARCHAR(64) NOT NULL,
    template_name VARCHAR(128) NOT NULL,
    template_content TEXT NOT NULL,
    template_variables JSONB NOT NULL DEFAULT '[]'::jsonb,
    version VARCHAR(64) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    remark TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_prompt_templates_type_version UNIQUE (template_type, version)
);

CREATE INDEX idx_prompt_templates_type_active ON prompt_templates (template_type, is_active);

CREATE TABLE system_configs (
    id BIGSERIAL PRIMARY KEY,
    config_key VARCHAR(128) NOT NULL UNIQUE,
    config_value JSONB NOT NULL DEFAULT '{}'::jsonb,
    description TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE task_runs (
    id BIGSERIAL PRIMARY KEY,
    task_type VARCHAR(64) NOT NULL,
    biz_date DATE NOT NULL,
    status VARCHAR(32) NOT NULL,
    trigger_source VARCHAR(32) NOT NULL CHECK (trigger_source IN ('scheduler', 'manual')),
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    duration_ms BIGINT,
    result_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_message TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0 CHECK (retry_count >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_task_runs_type_biz_date_desc ON task_runs (task_type, biz_date DESC);
CREATE INDEX idx_task_runs_status_started_desc ON task_runs (status, started_at DESC);

ALTER TABLE papers
    ADD CONSTRAINT fk_papers_last_score
        FOREIGN KEY (last_score_id) REFERENCES paper_scores(id) ON DELETE SET NULL,
    ADD CONSTRAINT fk_papers_last_content
        FOREIGN KEY (last_content_id) REFERENCES paper_contents(id) ON DELETE SET NULL,
    ADD CONSTRAINT fk_papers_last_draft
        FOREIGN KEY (last_draft_id) REFERENCES article_drafts(id) ON DELETE SET NULL;

CREATE TRIGGER trg_papers_set_updated_at
    BEFORE UPDATE ON papers
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_paper_contents_set_updated_at
    BEFORE UPDATE ON paper_contents
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_article_drafts_set_updated_at
    BEFORE UPDATE ON article_drafts
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_prompt_templates_set_updated_at
    BEFORE UPDATE ON prompt_templates
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_system_configs_set_updated_at
    BEFORE UPDATE ON system_configs
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

INSERT INTO system_configs (config_key, config_value, description) VALUES
    (
        'paper_selection',
        '{
          "daily_recommendation_count": 1,
          "top_n_pool": 10
        }'::jsonb,
        '论文筛选相关参数'
    ),
    (
        'site_rendering',
        '{
          "enabled": true,
          "draft_path_prefix": "/drafts",
          "default_markdown_template": "v1"
        }'::jsonb,
        '站内渲染参数'
    ),
    (
        'discovery',
        '{
          "time_window_hours": 24,
          "request_interval_seconds": 3
        }'::jsonb,
        '论文发现参数'
    );
