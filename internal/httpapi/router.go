package httpapi

import (
	"github.com/gin-gonic/gin"

	"arxivagent/internal/config"
	"arxivagent/internal/service"
	"arxivagent/internal/webui"
)

func NewRouter(cfg *config.Config, services *service.Services) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.LoadHTMLGlob("web/templates/*.tmpl")

	handler := NewHandler(services)
	pageHandler := webui.NewHandler(services)

	router.GET("/healthz", handler.Health)
	router.GET("/", pageHandler.Home)
	router.POST("/actions/discover", pageHandler.DiscoverAction)
	router.POST("/actions/generate", pageHandler.GenerateAction)
	router.GET("/drafts/:slug", pageHandler.DraftPage)
	router.GET("/assets/:id", pageHandler.AssetBinary)

	api := router.Group("/api")
	{
		api.GET("/papers", handler.ListPapers)
		api.GET("/papers/:id", handler.GetPaper)
		api.POST("/papers/:id/parse-generate", handler.ParseGeneratePaper)

		api.GET("/drafts", handler.ListDrafts)
		api.GET("/drafts/:id", handler.GetDraft)
		api.PUT("/drafts/:id", handler.UpdateDraft)
		api.POST("/drafts/:id/approve", handler.ApproveDraft)
		api.POST("/drafts/:id/reject", handler.RejectDraft)
		api.POST("/drafts/:id/render", handler.RenderDraft)

		api.GET("/site/drafts/today", handler.GetTodayDrafts)
		api.GET("/site/posts/:slug", handler.GetSitePost)

		api.GET("/configs", handler.ListConfigs)
		api.GET("/task-runs", handler.GetTaskRuns)
		api.POST("/task-runs/daily-discovery/run", handler.RunDailyDiscovery)
		api.POST("/task-runs/discover-generate/run", handler.RunDiscoverGenerate)
		api.POST("/task-runs/recommended-papers/parse-generate/run", handler.RunParseGenerateRecommended)
	}

	return router
}
