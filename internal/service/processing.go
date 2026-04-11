package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"arxivagent/internal/config"
	"arxivagent/internal/llm"
	"arxivagent/internal/pdfworker"
	"arxivagent/internal/repository"
)

const ParseGenerateTaskType = "parse_generate"

type ProcessingService struct {
	repo   *repository.ProcessingRepository
	tasks  *repository.TaskRepository
	cfg    *config.Config
	worker *pdfworker.Client
	llm    *llm.Client
}

type ParseGeneratePaperInput struct {
	PaperID int64
}

type ParseGeneratePaperResult struct {
	PaperID      int64  `json:"paper_id"`
	ContentID    int64  `json:"content_id"`
	DraftID      int64  `json:"draft_id"`
	FigureCount  int    `json:"figure_count"`
	MarkdownPath string `json:"markdown_path"`
	ParseStatus  string `json:"parse_status"`
}

type BatchParseGenerateInput struct {
	BizDate       time.Time
	TriggerSource string
}

type BatchParseGenerateResult struct {
	TaskRunID      int64                      `json:"task_run_id"`
	BizDate        string                     `json:"biz_date"`
	ProcessedCount int                        `json:"processed_count"`
	Items          []ParseGeneratePaperResult `json:"items"`
}

func NewProcessingService(repo *repository.ProcessingRepository, tasks *repository.TaskRepository, cfg *config.Config) *ProcessingService {
	return &ProcessingService{
		repo:   repo,
		tasks:  tasks,
		cfg:    cfg,
		worker: pdfworker.NewClient(cfg.Worker),
		llm:    llm.NewClient(cfg.LLM),
	}
}

func (s *ProcessingService) ParseAndGenerate(ctx context.Context, input ParseGeneratePaperInput) (*ParseGeneratePaperResult, error) {
	paper, err := s.repo.GetPaperForProcessing(ctx, input.PaperID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if err := os.MkdirAll(s.cfg.Storage.PDFDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(s.cfg.Storage.ImageDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(s.cfg.Storage.MarkdownDir, 0o755); err != nil {
		return nil, err
	}

	pdfPath, pdfSize, pdfSHA, err := s.downloadPDF(ctx, paper.ArxivID, paper.PDFURL)
	if err != nil {
		return nil, err
	}

	imageDir := filepath.Join(s.cfg.Storage.ImageDir, sanitizeFileName(paper.ArxivID))
	parserOutput, err := s.worker.Parse(ctx, pdfworker.ParseRequest{
		PDFPath:         pdfPath,
		OutputImagesDir: imageDir,
		PaperTitle:      paper.Title,
		ArxivID:         paper.ArxivID,
	})
	if err != nil {
		parseFailed := "parse worker failed: " + err.Error()
		_, _ = s.repo.UpsertPaperContent(ctx, repository.UpsertContentInput{
			PaperID:       paper.ID,
			ParseStatus:   "FAILED",
			PDFLocalPath:  pdfPath,
			PDFFileName:   filepath.Base(pdfPath),
			PDFSizeBytes:  pdfSize,
			PDFSHA256:     pdfSHA,
			ParserVersion: "pymupdf-v1",
			PromptVersion: "heuristic-v1",
			ErrorMessage:  &parseFailed,
		})
		return nil, err
	}

	summary := buildStructuredSummary(parserOutput, paper.Title)
	generation := s.generateDraftContent(ctx, paper, parserOutput, summary)
	markdown := generation.MarkdownContent
	if strings.TrimSpace(markdown) == "" {
		markdown = generateMarkdownDraft(paper, parserOutput, summary)
	}
	renderedHTML := renderMarkdown(markdown)
	slug := buildSlug(paper.Title + "-" + paper.ArxivID)
	sitePath := strings.TrimRight(s.cfg.Site.DraftPathPrefix, "/") + "/" + slug
	markdownPath := filepath.Join(s.cfg.Storage.MarkdownDir, sanitizeFileName(paper.ArxivID)+".md")
	if err := os.WriteFile(markdownPath, []byte(markdown), 0o644); err != nil {
		return nil, err
	}

	contentID, err := s.repo.UpsertPaperContent(ctx, repository.UpsertContentInput{
		PaperID:             paper.ID,
		ParseStatus:         "PARSED",
		PDFLocalPath:        pdfPath,
		PDFFileName:         filepath.Base(pdfPath),
		PDFSizeBytes:        pdfSize,
		PDFSHA256:           pdfSHA,
		PDFPageCount:        parserOutput.PDFPageCount,
		PDFMetadata:         parserOutput.PDFMetadata,
		SectionOutline:      parserOutput.SectionOutline,
		ParsedSections:      parserOutput.ParsedSections,
		AbstractCN:          generation.AbstractCN,
		InnovationsCN:       generation.InnovationsCN,
		MethodsCN:           generation.MethodsCN,
		ExperimentsCN:       generation.ExperimentsCN,
		ConclusionCN:        generation.ConclusionCN,
		LimitationsCN:       generation.LimitationsCN,
		StructuredSummary:   summary,
		RawParserOutput:     parserOutput,
		RawGenerationOutput: generation.RawOutput,
		ParserVersion:       "pymupdf-v1",
		PromptVersion:       generation.PromptVersion,
	})
	if err != nil {
		return nil, err
	}

	assets, err := s.buildAssets(paper.ID, paper.PDFURL, pdfPath, pdfSize, pdfSHA, parserOutput.Figures)
	if err != nil {
		return nil, err
	}
	if err := s.repo.ReplaceAssets(ctx, paper.ID, assets); err != nil {
		return nil, err
	}

	draftID, err := s.repo.UpsertGeneratedDraft(
		ctx, paper.ID, contentID, generation.RecommendedTitle,
		generation.AltTitles,
		generation.Summary,
		generation.IntroText,
		markdown, renderedHTML, generation.CoverText,
		generation.Tags,
		"markdown-v1", generation.PromptVersion, slug, sitePath,
	)
	if err != nil {
		return nil, err
	}

	return &ParseGeneratePaperResult{
		PaperID:      paper.ID,
		ContentID:    contentID,
		DraftID:      draftID,
		FigureCount:  len(parserOutput.Figures),
		MarkdownPath: markdownPath,
		ParseStatus:  "PARSED",
	}, nil
}

type llmDraftOutput struct {
	RecommendedTitle string   `json:"recommended_title"`
	AltTitles        []string `json:"alt_titles"`
	Summary          string   `json:"summary"`
	IntroText        string   `json:"intro_text"`
	CoverText        string   `json:"cover_text"`
	Tags             []string `json:"tags"`
	AbstractCN       string   `json:"abstract_cn"`
	InnovationsCN    string   `json:"innovations_cn"`
	MethodsCN        string   `json:"methods_cn"`
	ExperimentsCN    string   `json:"experiments_cn"`
	ConclusionCN     string   `json:"conclusion_cn"`
	LimitationsCN    string   `json:"limitations_cn"`
	MarkdownContent  string   `json:"markdown_content"`
}

type llmTitleOutput struct {
	RecommendedTitle string   `json:"recommended_title"`
	AltTitles        []string `json:"alt_titles"`
}

type generatedDraft struct {
	RecommendedTitle string
	AltTitles        []string
	Summary          string
	IntroText        string
	CoverText        string
	Tags             []string
	AbstractCN       string
	InnovationsCN    string
	MethodsCN        string
	ExperimentsCN    string
	ConclusionCN     string
	LimitationsCN    string
	MarkdownContent  string
	PromptVersion    string
	RawOutput        interface{}
}

func (s *ProcessingService) generateDraftContent(ctx context.Context, paper *repository.ProcessingPaper, parsed *pdfworker.ParseResponse, summary map[string]interface{}) generatedDraft {
	fallback := generatedDraft{
		RecommendedTitle: paper.Title,
		AltTitles:        nil,
		Summary:          composeExecutiveSummary(summary),
		IntroText:        composeIntro(paper.Title),
		CoverText:        "今日论文推荐",
		Tags:             []string{"arxiv", "remote-sensing", "foundation-model"},
		AbstractCN:       stringFromSummary(summary, "abstract"),
		InnovationsCN:    stringFromSummary(summary, "innovations"),
		MethodsCN:        stringFromSummary(summary, "method"),
		ExperimentsCN:    stringFromSummary(summary, "experiments"),
		ConclusionCN:     stringFromSummary(summary, "conclusion"),
		LimitationsCN:    stringFromSummary(summary, "limitations"),
		MarkdownContent:  generateMarkdownDraft(paper, parsed, summary),
		PromptVersion:    "heuristic-v1",
		RawOutput: map[string]interface{}{
			"mode": "heuristic",
		},
	}

	if !s.llm.Enabled() {
		return fallback
	}

	template, err := s.repo.GetActivePromptTemplate(ctx, "summary")
	if err != nil {
		return fallback
	}

	sectionSummaries, _ := json.Marshal(summary["summary_sections"])
	parsedSections, _ := json.Marshal(parsed.ParsedSections)
	figureCaptions, _ := json.Marshal(parsed.Figures)
	equations, _ := json.Marshal(parsed.Equations)
	userPrompt := template.TemplateContent
	replacements := map[string]string{
		"{{paper_title}}":       paper.Title,
		"{{arxiv_id}}":          paper.ArxivID,
		"{{source_url}}":        paper.SourceURL,
		"{{paper_abstract}}":    paper.Abstract,
		"{{section_summaries}}": string(sectionSummaries),
		"{{parsed_sections}}":   string(parsedSections),
		"{{figure_captions}}":   string(figureCaptions),
		"{{selected_figures}}":  string(figureCaptions),
		"{{equations}}":         string(equations),
		"{{paper_code_url}}":    "无",
	}
	for key, value := range replacements {
		userPrompt = strings.ReplaceAll(userPrompt, key, value)
	}

	systemPrompt := "你输出的是结构化 JSON，不要输出 Markdown 代码块包裹符号，不要编造论文中不存在的事实。"
	content, err := s.llm.Chat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return fallback
	}

	var output llmDraftOutput
	if err := json.Unmarshal([]byte(stripMarkdownCodeFence(content)), &output); err != nil {
		return fallback
	}

	if strings.TrimSpace(output.RecommendedTitle) == "" {
		output.RecommendedTitle = fallback.RecommendedTitle
	}
	output.AltTitles = normalizeAltTitles(output.AltTitles, output.RecommendedTitle)
	if strings.TrimSpace(output.Summary) == "" {
		output.Summary = fallback.Summary
	}
	if utf8.RuneCountInString(strings.TrimSpace(output.Summary)) < 80 {
		output.Summary = fallback.Summary
	}
	if strings.TrimSpace(output.IntroText) == "" {
		output.IntroText = fallback.IntroText
	}
	if strings.TrimSpace(output.CoverText) == "" {
		output.CoverText = fallback.CoverText
	}
	if len(output.Tags) == 0 {
		output.Tags = fallback.Tags
	}
	if strings.TrimSpace(output.AbstractCN) == "" {
		output.AbstractCN = fallback.AbstractCN
	}
	if strings.TrimSpace(output.InnovationsCN) == "" {
		output.InnovationsCN = fallback.InnovationsCN
	}
	if strings.TrimSpace(output.MethodsCN) == "" {
		output.MethodsCN = fallback.MethodsCN
	}
	if strings.TrimSpace(output.ExperimentsCN) == "" {
		output.ExperimentsCN = fallback.ExperimentsCN
	}
	if strings.TrimSpace(output.ConclusionCN) == "" {
		output.ConclusionCN = fallback.ConclusionCN
	}
	if strings.TrimSpace(output.LimitationsCN) == "" {
		output.LimitationsCN = fallback.LimitationsCN
	}
	if strings.TrimSpace(output.MarkdownContent) == "" {
		output.MarkdownContent = fallback.MarkdownContent
	}
	if utf8.RuneCountInString(strings.TrimSpace(output.MarkdownContent)) < 1200 {
		output.MarkdownContent = fallback.MarkdownContent
	}

	titleOutput, titlePromptVersion, err := s.generateTitleOptions(ctx, paper, output.RecommendedTitle, output.AltTitles, output.MethodsCN)
	if err == nil {
		output.RecommendedTitle = titleOutput.RecommendedTitle
		output.AltTitles = titleOutput.AltTitles
		template.Version = joinPromptVersions(template.Version, titlePromptVersion)
	}

	return generatedDraft{
		RecommendedTitle: output.RecommendedTitle,
		AltTitles:        output.AltTitles,
		Summary:          output.Summary,
		IntroText:        output.IntroText,
		CoverText:        output.CoverText,
		Tags:             output.Tags,
		AbstractCN:       output.AbstractCN,
		InnovationsCN:    output.InnovationsCN,
		MethodsCN:        output.MethodsCN,
		ExperimentsCN:    output.ExperimentsCN,
		ConclusionCN:     output.ConclusionCN,
		LimitationsCN:    output.LimitationsCN,
		MarkdownContent:  output.MarkdownContent,
		PromptVersion:    template.Version,
		RawOutput: map[string]interface{}{
			"mode":     "llm",
			"response": output,
		},
	}
}

func (s *ProcessingService) generateTitleOptions(ctx context.Context, paper *repository.ProcessingPaper, recommendedTitle string, altTitles []string, methodSummary string) (*llmTitleOutput, string, error) {
	template, err := s.repo.GetActivePromptTemplate(ctx, "title")
	if err != nil {
		return nil, "", err
	}

	userPrompt := template.TemplateContent
	replacements := map[string]string{
		"{{paper_title}}":    paper.Title,
		"{{paper_abstract}}": paper.Abstract,
		"{{method_summary}}": methodSummary,
	}
	for key, value := range replacements {
		userPrompt = strings.ReplaceAll(userPrompt, key, value)
	}

	systemPrompt := "你输出的是结构化 JSON，不要输出额外说明，不要输出 Markdown 代码块包裹符号。"
	content, err := s.llm.Chat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, "", err
	}

	var output llmTitleOutput
	if err := json.Unmarshal([]byte(stripMarkdownCodeFence(content)), &output); err != nil {
		return nil, "", err
	}

	if strings.TrimSpace(output.RecommendedTitle) == "" {
		output.RecommendedTitle = recommendedTitle
	}
	output.AltTitles = normalizeAltTitles(output.AltTitles, output.RecommendedTitle)
	if len(output.AltTitles) == 0 {
		output.AltTitles = normalizeAltTitles(altTitles, output.RecommendedTitle)
	}

	return &output, template.Version, nil
}

func (s *ProcessingService) ParseAndGenerateRecommended(ctx context.Context, input BatchParseGenerateInput) (*BatchParseGenerateResult, error) {
	bizDate := input.BizDate
	if bizDate.IsZero() {
		bizDate = time.Now()
	}

	taskRunID, err := s.tasks.Create(ctx, ParseGenerateTaskType, bizDate, defaultString(input.TriggerSource, "manual"))
	if err != nil {
		return nil, err
	}

	ids, err := s.repo.ListRecommendedPaperIDs(ctx, bizDate)
	if err != nil {
		errorMessage := err.Error()
		summary, _ := json.Marshal(map[string]string{"status": "FAILED"})
		_ = s.tasks.Finish(ctx, taskRunID, "FAILED", summary, &errorMessage)
		return nil, err
	}

	results := make([]ParseGeneratePaperResult, 0, len(ids))
	for _, id := range ids {
		item, err := s.ParseAndGenerate(ctx, ParseGeneratePaperInput{PaperID: id})
		if err != nil {
			errorMessage := err.Error()
			summary, _ := json.Marshal(map[string]interface{}{
				"processed_count": len(results),
				"failed_paper_id": id,
			})
			_ = s.tasks.Finish(ctx, taskRunID, "FAILED", summary, &errorMessage)
			return nil, err
		}
		results = append(results, *item)
	}

	result := &BatchParseGenerateResult{
		TaskRunID:      taskRunID,
		BizDate:        bizDate.Format("2006-01-02"),
		ProcessedCount: len(results),
		Items:          results,
	}
	summary, _ := json.Marshal(result)
	if err := s.tasks.Finish(ctx, taskRunID, "SUCCESS", summary, nil); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *ProcessingService) downloadPDF(ctx context.Context, arxivID, pdfURL string) (string, int64, string, error) {
	fileName := sanitizeFileName(arxivID) + ".pdf"
	filePath := filepath.Join(s.cfg.Storage.PDFDir, fileName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pdfURL, nil)
	if err != nil {
		return "", 0, "", err
	}
	req.Header.Set("User-Agent", s.cfg.Arxiv.UserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", 0, "", fmt.Errorf("download pdf status %d", resp.StatusCode)
	}

	f, err := os.Create(filePath)
	if err != nil {
		return "", 0, "", err
	}
	defer f.Close()

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(f, hasher), resp.Body)
	if err != nil {
		return "", 0, "", err
	}
	return filePath, written, hex.EncodeToString(hasher.Sum(nil)), nil
}

func (s *ProcessingService) buildAssets(paperID int64, pdfURL, pdfPath string, pdfSize int64, pdfSHA string, figures []pdfworker.Figure) ([]repository.AssetRecordInput, error) {
	pdfSource := pdfURL
	pdfLocal := pdfPath
	pdfName := filepath.Base(pdfPath)
	pdfMime := "application/pdf"
	pdfSizeCopy := pdfSize
	pdfSHACopy := pdfSHA
	assets := []repository.AssetRecordInput{
		{
			PaperID:   paperID,
			AssetType: "PDF",
			AssetRole: "original_pdf",
			SourceURL: &pdfSource,
			LocalPath: &pdfLocal,
			FileName:  &pdfName,
			MimeType:  &pdfMime,
			SizeBytes: &pdfSizeCopy,
			SHA256:    &pdfSHACopy,
			ExtraMetadata: map[string]interface{}{
				"kind": "original_pdf",
			},
		},
	}

	for idx, figure := range figures {
		data, err := os.ReadFile(figure.LocalPath)
		if err != nil {
			return nil, err
		}
		localPath := figure.LocalPath
		fileName := figure.FileName
		mimeType := figure.MimeType
		sizeBytes := int64(len(data))
		pageNo := figure.PageNo
		figureIndex := figure.FigureIndex
		displayOrder := idx + 1
		caption := figure.Caption
		width := figure.Width
		height := figure.Height
		assets = append(assets, repository.AssetRecordInput{
			PaperID:            paperID,
			AssetType:          "FIGURE",
			AssetRole:          fmt.Sprintf("figure_%d", figureIndex),
			LocalPath:          &localPath,
			FileName:           &fileName,
			MimeType:           &mimeType,
			SizeBytes:          &sizeBytes,
			PageNo:             &pageNo,
			FigureIndex:        &figureIndex,
			Caption:            stringPtrIfPresent(caption),
			Width:              &width,
			Height:             &height,
			DisplayOrder:       &displayOrder,
			IsExperimentFigure: true,
			BinaryData:         data,
			ExtraMetadata: map[string]interface{}{
				"caption_detected": caption != "",
				"diagram_score":    figure.DiagramScore,
			},
		})
	}
	return assets, nil
}

func buildStructuredSummary(parsed *pdfworker.ParseResponse, title string) map[string]interface{} {
	return map[string]interface{}{
		"paper_title":    title,
		"paper_venue":    "arXiv",
		"paper_code_url": nil,
		"summary_sections": map[string]interface{}{
			"abstract":    pickSummaryValue(parsed.Summary, "abstract", parsed.RawTextExcerpt),
			"innovations": pickSummaryValue(parsed.Summary, "innovations", ""),
			"method":      pickSummaryValue(parsed.Summary, "method", ""),
			"experiments": pickSummaryValue(parsed.Summary, "experiments", ""),
			"conclusion":  pickSummaryValue(parsed.Summary, "conclusion", ""),
			"limitations": pickSummaryValue(parsed.Summary, "limitations", ""),
		},
		"figures":   parsed.Figures,
		"equations": parsed.Equations,
	}
}

func stringFromSummary(summary map[string]interface{}, key string) string {
	sections, ok := summary["summary_sections"].(map[string]interface{})
	if !ok {
		return ""
	}
	if v, ok := sections[key].(string); ok {
		return v
	}
	return ""
}

func pickSummaryValue(source map[string]interface{}, key, fallback string) string {
	if source == nil {
		return fallback
	}
	if value, ok := source[key].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func generateMarkdownDraft(paper *repository.ProcessingPaper, parsed *pdfworker.ParseResponse, summary map[string]interface{}) string {
	abstract := firstNonEmpty(
		stringFromSummary(summary, "abstract"),
		clipDraftText(paper.Abstract, 2200),
	)
	innovations := firstNonEmpty(
		stringFromSummary(summary, "innovations"),
		clipDraftText(primarySectionText(parsed, "method", "methods", "methodology", "approach"), 2600),
	)
	method := firstNonEmpty(
		mergeDraftSections(
			stringFromSummary(summary, "method"),
			clipDraftText(primarySectionText(parsed, "method", "methods", "methodology", "approach"), 5200),
		),
		"当前 PDF 文本提取到的方法部分较少，建议人工回看原论文方法章节。",
	)
	experiments := firstNonEmpty(
		mergeDraftSections(
			stringFromSummary(summary, "experiments"),
			clipDraftText(primarySectionText(parsed, "experiment", "experiments", "experimental setup", "results"), 5200),
		),
		"当前 PDF 文本提取到的实验部分较少，建议人工补充关键实验设置与指标。",
	)
	conclusion := firstNonEmpty(
		stringFromSummary(summary, "conclusion"),
		clipDraftText(primarySectionText(parsed, "conclusion"), 1800),
	)
	limitations := firstNonEmpty(
		stringFromSummary(summary, "limitations"),
		clipDraftText(primarySectionText(parsed, "limitations"), 1800),
		"论文正文中未明确给出局限性，建议结合实验覆盖范围、泛化能力和标注质量进行人工补充。",
	)
	figures := topFigures(parsed.Figures, 4)
	equations := topEquations(parsed.Equations, 4)

	var b strings.Builder
	b.WriteString("# " + paper.Title + "\n\n")
	b.WriteString("论文标题：" + paper.Title + "\n")
	b.WriteString("论文期刊：arXiv\n")
	b.WriteString("论文代码：无\n")
	b.WriteString("原文链接：" + paper.SourceURL + "\n")
	b.WriteString("arXiv ID：`" + paper.ArxivID + "`\n\n")

	b.WriteString("## 1 摘要\n\n")
	b.WriteString(emptyFallback(abstract, "当前尚未生成可靠摘要，请人工补充。") + "\n\n")

	b.WriteString("## 2 创新点\n\n")
	if len(figures) > 0 {
		b.WriteString("### 图片\n\n")
		b.WriteString(buildFigureGuide(figures))
		b.WriteString("\n")
	}
	b.WriteString(emptyFallback(innovations, "当前尚未抽取出清晰的创新点，请人工结合方法章节整理。") + "\n\n")

	b.WriteString("## 3 方法\n\n")
	b.WriteString(composeDraftIntro(paper.Title) + "\n\n")
	b.WriteString(emptyFallback(method, "当前尚未抽取出清晰的方法细节，请人工回看论文方法部分。") + "\n\n")

	b.WriteString("### 关键公式\n\n")
	if len(equations) > 0 {
		for _, equation := range equations {
			b.WriteString("$$\n" + equation + "\n$$\n\n")
		}
	} else {
		b.WriteString("当前 PDF 文本抽取未能稳定还原公式，建议结合原文方法部分人工补充关键损失函数、注意力计算式或优化目标。\n\n")
	}

	b.WriteString("## 4 实验\n\n")
	b.WriteString(emptyFallback(experiments, "当前尚未抽取出清晰的实验结果，请人工补充实验设置、指标和对比基线。") + "\n\n")

	b.WriteString("## 5 结论\n\n")
	b.WriteString(emptyFallback(conclusion, "当前尚未生成结论总结，请人工补充论文结论。") + "\n\n")

	b.WriteString("## 6 局限性与未来工作\n\n")
	b.WriteString(emptyFallback(limitations, "建议从标注质量、数据覆盖范围、泛化能力与部署成本几个维度补充局限性和未来工作。") + "\n")
	return b.String()
}

func composeDraftIntro(title string) string {
	return "本文围绕论文《" + title + "》整理了一版偏详细的站内审阅稿，重点解释其问题设定、核心模块、方法流程、实验设计以及可复用的技术要点。当前内容基于论文 PDF 文本抽取和结构化总结生成，适合作为人工审阅和二次改写的底稿。"
}

func composeExecutiveSummary(summary map[string]interface{}) string {
	return firstNonEmpty(
		mergeDraftSections(
			stringFromSummary(summary, "abstract"),
			stringFromSummary(summary, "innovations"),
			stringFromSummary(summary, "experiments"),
		),
		stringFromSummary(summary, "abstract"),
	)
}

func primarySectionText(parsed *pdfworker.ParseResponse, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(parsed.ParsedSections[key]); value != "" {
			return value
		}
	}
	return ""
}

func mergeDraftSections(values ...string) string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return strings.Join(result, "\n\n")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func clipDraftText(v string, limit int) string {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" || limit <= 0 {
		return trimmed
	}
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	return strings.TrimSpace(string(runes[:limit])) + " ..."
}

func topFigures(figures []pdfworker.Figure, limit int) []pdfworker.Figure {
	if len(figures) == 0 || limit <= 0 {
		return nil
	}
	if len(figures) <= limit {
		return figures
	}
	return figures[:limit]
}

func buildFigureGuide(figures []pdfworker.Figure) string {
	var b strings.Builder
	for _, fig := range figures {
		b.WriteString(fmt.Sprintf("- 图 %d：第 %d 页，文件 `%s`。", fig.FigureIndex, fig.PageNo, fig.FileName))
		if strings.TrimSpace(fig.Caption) != "" {
			b.WriteString("图注：" + strings.TrimSpace(fig.Caption) + "。")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func topEquations(equations []string, limit int) []string {
	if len(equations) == 0 || limit <= 0 {
		return nil
	}
	if len(equations) <= limit {
		return equations
	}
	return equations[:limit]
}

func legacyGenerateMarkdownDraft(title, arxivID, sourceURL string, summary map[string]interface{}, figures []pdfworker.Figure) string {
	abstract := stringFromSummary(summary, "abstract")
	innovations := stringFromSummary(summary, "innovations")
	method := stringFromSummary(summary, "method")
	experiments := stringFromSummary(summary, "experiments")
	conclusion := stringFromSummary(summary, "conclusion")
	limitations := stringFromSummary(summary, "limitations")

	var b strings.Builder
	b.WriteString("# " + title + "\n\n")
	b.WriteString("## 导语\n\n")
	b.WriteString(composeIntro(title) + "\n\n")
	b.WriteString("## 论文信息卡\n\n")
	b.WriteString("- arXiv ID: `" + arxivID + "`\n")
	b.WriteString("- 原文链接: " + sourceURL + "\n")
	b.WriteString("- 来源: arXiv\n\n")
	b.WriteString("## 摘要解读\n\n")
	b.WriteString(emptyFallback(abstract, "待补充摘要解读。") + "\n\n")
	b.WriteString("## 创新点\n\n")
	b.WriteString(emptyFallback(innovations, "待补充创新点总结。") + "\n\n")
	b.WriteString("## 方法解读\n\n")
	b.WriteString(emptyFallback(method, "待补充方法解读。") + "\n\n")
	b.WriteString("## 实验结果解读\n\n")
	b.WriteString(emptyFallback(experiments, "待补充实验结果解读。") + "\n\n")
	if len(figures) > 0 {
		b.WriteString("## 实验相关图像与图注\n\n")
		for _, fig := range figures {
			b.WriteString(fmt.Sprintf("### Figure %d\n\n", fig.FigureIndex))
			b.WriteString(fmt.Sprintf("- 页码: %d\n", fig.PageNo))
			if strings.TrimSpace(fig.Caption) != "" {
				b.WriteString("- 图注: " + fig.Caption + "\n")
			}
			b.WriteString("- 文件: `" + fig.FileName + "`\n\n")
		}
	}
	b.WriteString("## 结论\n\n")
	b.WriteString(emptyFallback(conclusion, "待补充结论总结。") + "\n\n")
	b.WriteString("## 局限与后续关注\n\n")
	b.WriteString(emptyFallback(limitations, "待补充局限性与未来工作。") + "\n")
	return b.String()
}

func composeIntro(title string) string {
	return "本文围绕论文《" + title + "》生成一版站内审阅稿，重点覆盖研究问题、方法设计、关键模块、公式线索与实验结论，适合作为人工精修前的详细底稿。"
}

func legacyComposeIntro(title string) string {
	return "本文对论文《" + title + "》做一版站内初稿整理，当前内容基于论文 PDF 提取与启发式摘要生成，适合作为人工审阅底稿。"
}

func emptyFallback(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func sanitizeFileName(v string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(v)
}

func stripMarkdownCodeFence(v string) string {
	trimmed := strings.TrimSpace(v)
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSuffix(trimmed, "```")
	}
	return strings.TrimSpace(trimmed)
}

func normalizeAltTitles(values []string, recommendedTitle string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}

	if trimmed := strings.TrimSpace(recommendedTitle); trimmed != "" {
		seen[trimmed] = struct{}{}
	}

	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
		if len(result) == 3 {
			break
		}
	}

	return result
}

func joinPromptVersions(values ...string) string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}

	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}

	return strings.Join(result, "+")
}
