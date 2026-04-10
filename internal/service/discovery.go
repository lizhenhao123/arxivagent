package service

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"arxivagent/internal/arxiv"
	"arxivagent/internal/config"
	"arxivagent/internal/repository"
)

const DailyDiscoveryTaskType = "daily_discovery"

type DiscoveryService struct {
	repo   *repository.DiscoveryRepository
	tasks  *repository.TaskRepository
	client *arxiv.Client
	cfg    *config.Config
}

type RunDailyDiscoveryInput struct {
	BizDate       time.Time
	TriggerSource string
	Force         bool
}

type RunDailyDiscoveryResult struct {
	TaskRunID              int64    `json:"task_run_id"`
	BizDate                string   `json:"biz_date"`
	FetchedCount           int      `json:"fetched_count"`
	FilteredCount          int      `json:"filtered_count"`
	ScoredCount            int      `json:"scored_count"`
	RecommendedCount       int      `json:"recommended_count"`
	RecommendedPaperIDs    []int64  `json:"recommended_paper_ids"`
	RecommendedPaperTitles []string `json:"recommended_paper_titles"`
}

type discoveryFilters struct {
	CategoryWhitelist []string `json:"category_whitelist"`
	MaxResults        int      `json:"max_results"`
	TimeWindowHours   int      `json:"time_window_hours"`
}

type discoveryKeywords struct {
	TopicKeywords      []string `json:"topic_keywords"`
	FoundationKeywords []string `json:"foundation_keywords"`
	NoveltyKeywords    []string `json:"novelty_keywords"`
	PracticalityKeys   []string `json:"practicality_keywords"`
	EvidenceKeywords   []string `json:"evidence_keywords"`
}

type ruleScore struct {
	TopicScore           int
	FoundationModelScore int
	NoveltyScore         int
	PracticalityScore    int
	EvidenceScore        int
	TotalScore           int
	Recommendation       string
	Reasons              []string
	Risks                []string
	Detail               map[string]interface{}
}

func NewDiscoveryService(repo *repository.DiscoveryRepository, tasks *repository.TaskRepository, cfg *config.Config) *DiscoveryService {
	return &DiscoveryService{
		repo:   repo,
		tasks:  tasks,
		client: arxiv.NewClient(cfg.Arxiv),
		cfg:    cfg,
	}
}

func (s *DiscoveryService) RunDaily(ctx context.Context, input RunDailyDiscoveryInput) (*RunDailyDiscoveryResult, error) {
	bizDate := input.BizDate
	if bizDate.IsZero() {
		bizDate = time.Now()
	}
	bizDate = bizDate.In(time.Local)

	if !input.Force {
		exists, err := s.repo.ExistsTaskRun(ctx, DailyDiscoveryTaskType, bizDate)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrInvalidState
		}
	}

	taskRunID, err := s.tasks.Create(ctx, DailyDiscoveryTaskType, bizDate, defaultString(input.TriggerSource, "manual"))
	if err != nil {
		return nil, err
	}

	result, runErr := s.runDaily(ctx, taskRunID, bizDate)
	if runErr != nil {
		errorMessage := runErr.Error()
		summary, _ := json.Marshal(map[string]interface{}{
			"task_run_id": taskRunID,
			"biz_date":    bizDate.Format("2006-01-02"),
			"status":      "FAILED",
		})
		_ = s.tasks.Finish(ctx, taskRunID, "FAILED", summary, &errorMessage)
		return nil, runErr
	}

	summary, _ := json.Marshal(result)
	if err := s.tasks.Finish(ctx, taskRunID, "SUCCESS", summary, nil); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *DiscoveryService) runDaily(ctx context.Context, taskRunID int64, bizDate time.Time) (*RunDailyDiscoveryResult, error) {
	configMap, err := s.repo.GetConfigMap(ctx, "paper_selection", "discovery_filters", "discovery_keywords")
	if err != nil {
		return nil, err
	}

	filters := discoveryFilters{
		CategoryWhitelist: []string{"cs.CV", "cs.AI", "eess.IV"},
		MaxResults:        s.cfg.Arxiv.MaxResults,
		TimeWindowHours:   24,
	}
	if raw := configMap["discovery_filters"]; len(raw) > 0 {
		_ = json.Unmarshal(raw, &filters)
	}
	if filters.MaxResults <= 0 {
		filters.MaxResults = s.cfg.Arxiv.MaxResults
	}
	if filters.TimeWindowHours <= 0 {
		filters.TimeWindowHours = 24
	}

	keywords := discoveryKeywords{
		TopicKeywords:      []string{"remote sensing", "earth observation", "satellite", "aerial", "geospatial", "sar", "multispectral", "hyperspectral", "change detection"},
		FoundationKeywords: []string{"foundation model", "pretraining", "pre-training", "generalist", "vision-language", "multimodal", "vlm", "large-scale"},
		NoveltyKeywords:    []string{"novel", "new", "framework", "benchmark", "dataset", "unified"},
		PracticalityKeys:   []string{"classification", "detection", "segmentation", "change detection", "localization", "retrieval"},
		EvidenceKeywords:   []string{"experiment", "benchmark", "ablation", "baseline", "state-of-the-art", "code"},
	}
	if raw := configMap["discovery_keywords"]; len(raw) > 0 {
		_ = json.Unmarshal(raw, &keywords)
	}

	recommendationCount := 1
	var paperSelection struct {
		DailyRecommendationCount int `json:"daily_recommendation_count"`
		TopNPool                 int `json:"top_n_pool"`
	}
	if raw := configMap["paper_selection"]; len(raw) > 0 {
		_ = json.Unmarshal(raw, &paperSelection)
	}
	if paperSelection.DailyRecommendationCount > 0 {
		recommendationCount = paperSelection.DailyRecommendationCount
	}

	searchQuery := buildSearchQuery(keywords.TopicKeywords, keywords.FoundationKeywords)
	papers, err := s.client.Search(ctx, arxiv.QueryOptions{
		SearchQuery: searchQuery,
		Start:       0,
		MaxResults:  filters.MaxResults,
		SortBy:      "lastUpdatedDate",
		SortOrder:   "descending",
	})
	if err != nil {
		return nil, err
	}

	cutoff := bizDate.Add(-time.Duration(filters.TimeWindowHours) * time.Hour)
	type candidate struct {
		paper   arxiv.Paper
		paperID int64
		scoreID int64
		score   ruleScore
	}
	candidates := make([]candidate, 0, len(papers))
	filteredCount := 0
	scoredCount := 0

	for _, paper := range papers {
		if paper.UpdatedAt.Before(cutoff) {
			continue
		}
		if !matchesWhitelist(paper.Categories, filters.CategoryWhitelist) {
			continue
		}

		score := scorePaper(paper, keywords)
		if score.TopicScore == 0 {
			continue
		}
		filteredCount++

		paperID, err := s.repo.UpsertPaperAndVersion(ctx, repository.UpsertPaperInput{
			ArxivID:         paper.ArxivID,
			VersionNo:       paper.VersionNo,
			Title:           paper.Title,
			Authors:         paper.Authors,
			Abstract:        paper.Abstract,
			PrimaryCategory: paper.PrimaryCategory,
			Categories:      paper.Categories,
			PublishedAt:     paper.PublishedAt,
			UpdatedAt:       paper.UpdatedAt,
			PDFURL:          paper.PDFURL,
			SourceURL:       paper.SourceURL,
			SourcePayload:   paper,
		})
		if err != nil {
			return nil, err
		}

		if err := s.repo.UpdatePaperStatus(ctx, paperID, "FILTERED", nil); err != nil {
			return nil, err
		}

		scoreID, err := s.repo.InsertScore(ctx, repository.InsertScoreInput{
			PaperID:              paperID,
			ScoreDate:            bizDate,
			TopicScore:           score.TopicScore,
			FoundationModelScore: score.FoundationModelScore,
			NoveltyScore:         score.NoveltyScore,
			PracticalityScore:    score.PracticalityScore,
			EvidenceScore:        score.EvidenceScore,
			TotalScore:           score.TotalScore,
			Recommendation:       score.Recommendation,
			ScoreReasons:         score.Reasons,
			RiskNotes:            score.Risks,
			ScoreDetail:          score.Detail,
			RuleVersion:          "rule-v1",
			PromptVersion:        "rule-v1",
		})
		if err != nil {
			return nil, err
		}

		scoredCount++
		candidates = append(candidates, candidate{
			paper:   paper,
			paperID: paperID,
			scoreID: scoreID,
			score:   score,
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score.TotalScore == candidates[j].score.TotalScore {
			return candidates[i].paper.UpdatedAt.After(candidates[j].paper.UpdatedAt)
		}
		return candidates[i].score.TotalScore > candidates[j].score.TotalScore
	})

	recommended := make([]repository.RecommendedPaper, 0, recommendationCount)
	recommendedIDs := make([]int64, 0, recommendationCount)
	recommendedTitles := make([]string, 0, recommendationCount)

	for idx, item := range candidates {
		if idx >= recommendationCount {
			break
		}
		recommended = append(recommended, repository.RecommendedPaper{
			PaperID:   item.paperID,
			ScoreID:   item.scoreID,
			RankInDay: idx + 1,
		})
		recommendedIDs = append(recommendedIDs, item.paperID)
		recommendedTitles = append(recommendedTitles, item.paper.Title)
	}

	if err := s.repo.ReplaceRecommendations(ctx, bizDate, recommended); err != nil {
		return nil, err
	}

	for _, item := range recommended {
		if _, err := s.repo.CreateDraftPlaceholder(ctx, item.PaperID, recommendedTitles[item.RankInDay-1]); err != nil {
			return nil, err
		}
	}

	return &RunDailyDiscoveryResult{
		TaskRunID:              taskRunID,
		BizDate:                bizDate.Format("2006-01-02"),
		FetchedCount:           len(papers),
		FilteredCount:          filteredCount,
		ScoredCount:            scoredCount,
		RecommendedCount:       len(recommended),
		RecommendedPaperIDs:    recommendedIDs,
		RecommendedPaperTitles: recommendedTitles,
	}, nil
}

func buildSearchQuery(topicKeywords, foundationKeywords []string) string {
	all := append([]string{}, topicKeywords...)
	all = append(all, foundationKeywords...)
	parts := make([]string, 0, len(all))
	for _, keyword := range all {
		trimmed := strings.TrimSpace(keyword)
		if trimmed == "" {
			continue
		}
		parts = append(parts, `all:"`+trimmed+`"`)
	}
	if len(parts) == 0 {
		return `all:"remote sensing"`
	}
	return strings.Join(parts, " OR ")
}

func matchesWhitelist(categories, whitelist []string) bool {
	if len(whitelist) == 0 {
		return true
	}
	for _, category := range categories {
		for _, allowed := range whitelist {
			if strings.EqualFold(category, allowed) {
				return true
			}
		}
	}
	return false
}

func scorePaper(paper arxiv.Paper, keywords discoveryKeywords) ruleScore {
	text := strings.ToLower(paper.Title + " " + paper.Abstract)
	categoryText := strings.ToLower(strings.Join(paper.Categories, " "))

	topicHits := hitKeywords(text, keywords.TopicKeywords)
	topicScore := scaledScore(len(topicHits), 3, 40)

	foundationHits := hitKeywords(text, keywords.FoundationKeywords)
	foundationScore := scaledScore(len(foundationHits), 3, 25)

	noveltyHits := hitKeywords(text, keywords.NoveltyKeywords)
	noveltyScore := scaledScore(len(noveltyHits), 3, 15)

	practicalityHits := hitKeywords(text, keywords.PracticalityKeys)
	practicalityScore := scaledScore(len(practicalityHits), 2, 10)

	evidenceHits := hitKeywords(text, keywords.EvidenceKeywords)
	evidenceScore := scaledScore(len(evidenceHits), 3, 10)

	if strings.Contains(categoryText, "cs.cv") || strings.Contains(categoryText, "eess.iv") {
		topicScore = minInt(40, topicScore+8)
	}

	total := topicScore + foundationScore + noveltyScore + practicalityScore + evidenceScore
	recommendation := "low"
	switch {
	case total >= 80:
		recommendation = "high"
	case total >= 60:
		recommendation = "medium"
	}

	reasons := make([]string, 0, 3)
	if len(topicHits) > 0 {
		reasons = append(reasons, "主题关键词命中："+strings.Join(topicHits, "、"))
	}
	if len(foundationHits) > 0 {
		reasons = append(reasons, "基础模型相关关键词命中："+strings.Join(foundationHits, "、"))
	}
	if len(practicalityHits) > 0 {
		reasons = append(reasons, "覆盖任务场景："+strings.Join(practicalityHits, "、"))
	}

	risks := make([]string, 0, 2)
	if len(foundationHits) == 0 {
		risks = append(risks, "基础模型属性证据偏弱")
	}
	if len(evidenceHits) == 0 {
		risks = append(risks, "摘要中实验或证据描述较弱")
	}

	return ruleScore{
		TopicScore:           topicScore,
		FoundationModelScore: foundationScore,
		NoveltyScore:         noveltyScore,
		PracticalityScore:    practicalityScore,
		EvidenceScore:        evidenceScore,
		TotalScore:           total,
		Recommendation:       recommendation,
		Reasons:              reasons,
		Risks:                risks,
		Detail: map[string]interface{}{
			"topic_hits":        topicHits,
			"foundation_hits":   foundationHits,
			"novelty_hits":      noveltyHits,
			"practicality_hits": practicalityHits,
			"evidence_hits":     evidenceHits,
		},
	}
}

func hitKeywords(text string, keywords []string) []string {
	hits := make([]string, 0, len(keywords))
	for _, keyword := range keywords {
		trimmed := strings.TrimSpace(strings.ToLower(keyword))
		if trimmed == "" {
			continue
		}
		if strings.Contains(text, trimmed) {
			hits = append(hits, keyword)
		}
	}
	return hits
}

func scaledScore(hitCount, fullHitCount, maxScore int) int {
	if hitCount <= 0 {
		return 0
	}
	if hitCount >= fullHitCount {
		return maxScore
	}
	score := hitCount * maxScore / fullHitCount
	if score <= 0 {
		return 1
	}
	return score
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func defaultString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
