package webui

import (
	"html/template"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"arxivagent/internal/model"
	"arxivagent/internal/service"
)

type Handler struct {
	services *service.Services
}

type homeData struct {
	Title  string
	Drafts interface{}
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
		Title:  "今日论文稿件",
		Drafts: drafts,
	})
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
