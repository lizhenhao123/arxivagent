package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"arxivagent/internal/service"
)

type Handler struct {
	services *service.Services
}

func NewHandler(services *service.Services) *Handler {
	return &Handler{services: services}
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data: map[string]string{
			"status": "ok",
		},
	})
}

func (h *Handler) ListPapers(c *gin.Context) {
	page := parseIntOrDefault(c.Query("page"), 1)
	pageSize := parseIntOrDefault(c.Query("page_size"), 20)

	result, err := h.services.Papers.List(c.Request.Context(), service.ListPapersInput{
		Page:          page,
		PageSize:      pageSize,
		Status:        c.Query("status"),
		RecommendedOn: c.Query("recommended_on"),
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data: PaginatedData{
			Items:    result.Items,
			Page:     page,
			PageSize: pageSize,
			Total:    result.Total,
		},
	})
}

func (h *Handler) GetPaper(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid paper id")
		return
	}

	paper, err := h.services.Papers.GetDetail(c.Request.Context(), id)
	if err != nil {
		respondError(c, statusFromError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: paper})
}

func (h *Handler) ParseGeneratePaper(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid paper id")
		return
	}

	result, err := h.services.Processing.ParseAndGenerate(c.Request.Context(), service.ParseGeneratePaperInput{
		PaperID: id,
	})
	if err != nil {
		respondError(c, statusFromError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: result})
}

func (h *Handler) ListDrafts(c *gin.Context) {
	page := parseIntOrDefault(c.Query("page"), 1)
	pageSize := parseIntOrDefault(c.Query("page_size"), 20)

	result, err := h.services.Drafts.List(c.Request.Context(), service.ListDraftsInput{
		Page:         page,
		PageSize:     pageSize,
		ReviewStatus: c.Query("review_status"),
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data: PaginatedData{
			Items:    result.Items,
			Page:     page,
			PageSize: pageSize,
			Total:    result.Total,
		},
	})
}

func (h *Handler) GetDraft(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid draft id")
		return
	}

	draft, err := h.services.Drafts.GetDetail(c.Request.Context(), id)
	if err != nil {
		respondError(c, statusFromError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: draft})
}

type updateDraftRequest struct {
	Title           string   `json:"title"`
	Summary         string   `json:"summary"`
	IntroText       string   `json:"intro_text"`
	MarkdownContent string   `json:"markdown_content"`
	CoverText       string   `json:"cover_text"`
	Tags            []string `json:"tags"`
	ReviewComment   string   `json:"review_comment"`
}

func (h *Handler) UpdateDraft(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid draft id")
		return
	}

	var req updateDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	draft, err := h.services.Drafts.Update(c.Request.Context(), service.UpdateDraftInput{
		ID:              id,
		Title:           req.Title,
		Summary:         req.Summary,
		IntroText:       req.IntroText,
		MarkdownContent: req.MarkdownContent,
		CoverText:       req.CoverText,
		Tags:            req.Tags,
		ReviewComment:   req.ReviewComment,
	})
	if err != nil {
		respondError(c, statusFromError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: draft})
}

func (h *Handler) ApproveDraft(c *gin.Context) {
	h.updateDraftStatus(c, service.ReviewStatusApproved)
}

func (h *Handler) RejectDraft(c *gin.Context) {
	h.updateDraftStatus(c, service.ReviewStatusRejected)
}

func (h *Handler) RenderDraft(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid draft id")
		return
	}

	draft, err := h.services.Drafts.Render(c.Request.Context(), id)
	if err != nil {
		respondError(c, statusFromError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: draft})
}

func (h *Handler) GetTodayDrafts(c *gin.Context) {
	items, err := h.services.Site.TodayDrafts(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: items})
}

func (h *Handler) GetSitePost(c *gin.Context) {
	post, err := h.services.Site.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		respondError(c, statusFromError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: post})
}

func (h *Handler) ListConfigs(c *gin.Context) {
	items, err := h.services.Configs.List(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: items})
}

func (h *Handler) GetTaskRuns(c *gin.Context) {
	items, err := h.services.Tasks.List(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: items})
}

func (h *Handler) RunDailyDiscovery(c *gin.Context) {
	var req struct {
		BizDate     string `json:"biz_date"`
		Force       bool   `json:"force"`
		SearchTopic string `json:"search_topic"`
		MaxResults  int    `json:"max_results"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && c.Request.ContentLength > 0 {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	var bizDate time.Time
	var err error
	if req.BizDate != "" {
		bizDate, err = time.Parse("2006-01-02", req.BizDate)
		if err != nil {
			respondError(c, http.StatusBadRequest, "invalid biz_date, expected YYYY-MM-DD")
			return
		}
	}

	result, err := h.services.Discovery.RunDaily(c.Request.Context(), service.RunDailyDiscoveryInput{
		BizDate:       bizDate,
		TriggerSource: "manual",
		Force:         req.Force,
		SearchTopic:   req.SearchTopic,
		MaxResults:    req.MaxResults,
	})
	if err != nil {
		respondError(c, statusFromError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: result})
}

func (h *Handler) RunDiscoverGenerate(c *gin.Context) {
	var req struct {
		BizDate     string `json:"biz_date"`
		Force       bool   `json:"force"`
		SearchTopic string `json:"search_topic"`
		MaxResults  int    `json:"max_results"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && c.Request.ContentLength > 0 {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	var bizDate time.Time
	var err error
	if req.BizDate != "" {
		bizDate, err = time.Parse("2006-01-02", req.BizDate)
		if err != nil {
			respondError(c, http.StatusBadRequest, "invalid biz_date, expected YYYY-MM-DD")
			return
		}
	}

	discoveryResult, err := h.services.Discovery.RunDaily(c.Request.Context(), service.RunDailyDiscoveryInput{
		BizDate:       bizDate,
		TriggerSource: "manual",
		Force:         req.Force,
		SearchTopic:   req.SearchTopic,
		MaxResults:    req.MaxResults,
	})
	if err != nil {
		respondError(c, statusFromError(err), err.Error())
		return
	}

	parseResult, err := h.services.Processing.ParseAndGenerateRecommended(c.Request.Context(), service.BatchParseGenerateInput{
		BizDate:       bizDate,
		TriggerSource: "manual",
	})
	if err != nil {
		respondError(c, statusFromError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data: map[string]interface{}{
			"discovery":      discoveryResult,
			"parse_generate": parseResult,
		},
	})
}

func (h *Handler) RunParseGenerateRecommended(c *gin.Context) {
	var req struct {
		BizDate string `json:"biz_date"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && c.Request.ContentLength > 0 {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	var bizDate time.Time
	var err error
	if req.BizDate != "" {
		bizDate, err = time.Parse("2006-01-02", req.BizDate)
		if err != nil {
			respondError(c, http.StatusBadRequest, "invalid biz_date, expected YYYY-MM-DD")
			return
		}
	}

	result, err := h.services.Processing.ParseAndGenerateRecommended(c.Request.Context(), service.BatchParseGenerateInput{
		BizDate:       bizDate,
		TriggerSource: "manual",
	})
	if err != nil {
		respondError(c, statusFromError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: result})
}

func (h *Handler) updateDraftStatus(c *gin.Context, status string) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid draft id")
		return
	}

	var req struct {
		ReviewComment string `json:"review_comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && c.Request.ContentLength > 0 {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	draft, err := h.services.Drafts.UpdateStatus(c.Request.Context(), service.UpdateDraftStatusInput{
		ID:            id,
		ReviewStatus:  status,
		ReviewComment: req.ReviewComment,
	})
	if err != nil {
		respondError(c, statusFromError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: draft})
}

func parseIntOrDefault(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func respondError(c *gin.Context, status int, message string) {
	c.JSON(status, APIResponse{
		Code:    status,
		Message: message,
	})
}

func statusFromError(err error) int {
	switch err {
	case service.ErrNotFound:
		return http.StatusNotFound
	case service.ErrInvalidState, service.ErrValidation:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
