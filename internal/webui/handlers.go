package webui

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"arxivagent/internal/model"
	"arxivagent/internal/service"
)

type Handler struct {
	services *service.Services
}

type homeData struct {
	Title       string
	SearchTopic string
	Message     string
	Error       string
	Drafts      []model.SiteDraft
}

type draftPageData struct {
	Title string
	Post  draftPagePost
}

type draftPagePost struct {
	Title        string
	ArxivID      string
	Summary      *string
	RenderedHTML template.HTML
	ReviewStatus string
	SourceURL    string
	Tags         []string
	Assets       []model.PaperAsset
}

func NewHandler(services *service.Services) *Handler {
	return &Handler{services: services}
}

func (h *Handler) Home(c *gin.Context) {
	drafts, err := h.services.Site.TodayDrafts(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.HTML(http.StatusOK, "home.tmpl", homeData{
		Title:       "今日论文稿件",
		SearchTopic: strings.TrimSpace(c.Query("topic")),
		Message:     strings.TrimSpace(c.Query("message")),
		Error:       strings.TrimSpace(c.Query("error")),
		Drafts:      drafts,
	})
}

func (h *Handler) DiscoverAction(c *gin.Context) {
	searchTopic := strings.TrimSpace(c.PostForm("search_topic"))

	result, err := h.services.Discovery.RunDaily(c.Request.Context(), service.RunDailyDiscoveryInput{
		TriggerSource: "manual",
		Force:         true,
		SearchTopic:   searchTopic,
	})
	if err != nil {
		redirectHome(c, searchTopic, "", err.Error())
		return
	}

	message := fmt.Sprintf("发现完成：抓取 %d 篇，筛选 %d 篇，推荐 %d 篇。", result.FetchedCount, result.FilteredCount, result.RecommendedCount)
	redirectHome(c, searchTopic, message, "")
}

func (h *Handler) GenerateAction(c *gin.Context) {
	searchTopic := strings.TrimSpace(c.PostForm("search_topic"))

	discoveryResult, err := h.services.Discovery.RunDaily(c.Request.Context(), service.RunDailyDiscoveryInput{
		TriggerSource: "manual",
		Force:         true,
		SearchTopic:   searchTopic,
	})
	if err != nil {
		redirectHome(c, searchTopic, "", err.Error())
		return
	}

	parseResult, err := h.services.Processing.ParseAndGenerateRecommended(c.Request.Context(), service.BatchParseGenerateInput{
		TriggerSource: "manual",
	})
	if err != nil {
		redirectHome(c, searchTopic, "", err.Error())
		return
	}

	message := fmt.Sprintf("已完成搜索并生成：推荐 %d 篇，成功生成 %d 篇稿件。", discoveryResult.RecommendedCount, parseResult.ProcessedCount)
	redirectHome(c, searchTopic, message, "")
}

func (h *Handler) DraftPage(c *gin.Context) {
	post, err := h.services.Site.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		if err == service.ErrNotFound {
			c.String(http.StatusNotFound, "draft not found")
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.HTML(http.StatusOK, "draft.tmpl", draftPageData{
		Title: post.Title,
		Post: draftPagePost{
			Title:        post.Title,
			ArxivID:      post.ArxivID,
			Summary:      post.Summary,
			RenderedHTML: template.HTML(safeRenderedHTML(post)),
			ReviewStatus: post.ReviewStatus,
			SourceURL:    post.SourceURL,
			Tags:         post.Tags,
			Assets:       post.Assets,
		},
	})
}

func (h *Handler) AssetBinary(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid asset id")
		return
	}
	data, contentType, err := h.services.Site.GetAssetBinary(c.Request.Context(), id)
	if err != nil {
		if err == service.ErrNotFound {
			c.String(http.StatusNotFound, "asset not found")
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Data(http.StatusOK, contentType, data)
}

func safeRenderedHTML(post *model.SitePost) string {
	if post.RenderedHTML != nil && *post.RenderedHTML != "" {
		return *post.RenderedHTML
	}
	return "<pre>" + template.HTMLEscapeString(post.MarkdownContent) + "</pre>"
}

func redirectHome(c *gin.Context, topic, message, errText string) {
	params := url.Values{}
	if strings.TrimSpace(topic) != "" {
		params.Set("topic", strings.TrimSpace(topic))
	}
	if strings.TrimSpace(message) != "" {
		params.Set("message", strings.TrimSpace(message))
	}
	if strings.TrimSpace(errText) != "" {
		params.Set("error", strings.TrimSpace(errText))
	}

	target := "/"
	if encoded := params.Encode(); encoded != "" {
		target += "?" + encoded
	}
	c.Redirect(http.StatusSeeOther, target)
}
