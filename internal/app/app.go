package app

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"arxivagent/internal/config"
	"arxivagent/internal/db"
	"arxivagent/internal/httpapi"
	"arxivagent/internal/repository"
	"arxivagent/internal/service"
)

type App struct {
	Router *gin.Engine
	pool   *pgxpool.Pool
}

func New(cfg *config.Config) (*App, error) {
	pool, err := db.NewPool(context.Background(), cfg)
	if err != nil {
		return nil, err
	}

	repos := repository.NewRepositories(pool)
	services := service.NewServices(repos, cfg)
	router := httpapi.NewRouter(cfg, services)

	return &App{
		Router: router,
		pool:   pool,
	}, nil
}

func (a *App) Close() {
	if a.pool != nil {
		a.pool.Close()
	}
}
