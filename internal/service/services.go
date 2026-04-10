package service

import (
	"arxivagent/internal/config"
	"arxivagent/internal/repository"
)

type Services struct {
	Discovery  *DiscoveryService
	Processing *ProcessingService
	Papers     *PaperService
	Drafts     *DraftService
	Site       *SiteService
	Configs    *ConfigService
	Tasks      *TaskService
}

func NewServices(repos *repository.Repositories, cfg *config.Config) *Services {
	return &Services{
		Discovery:  NewDiscoveryService(repos.Discovery, repos.Tasks, cfg),
		Processing: NewProcessingService(repos.Processing, repos.Tasks, cfg),
		Papers:     &PaperService{repo: repos.Papers},
		Drafts:     &DraftService{repo: repos.Drafts, siteCfg: cfg.Site},
		Site:       &SiteService{repo: repos.Site},
		Configs:    &ConfigService{repo: repos.Configs},
		Tasks:      &TaskService{repo: repos.Tasks},
	}
}
