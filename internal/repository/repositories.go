package repository

import "github.com/jackc/pgx/v5/pgxpool"

type Repositories struct {
	Papers     *PaperRepository
	Discovery  *DiscoveryRepository
	Processing *ProcessingRepository
	Drafts     *DraftRepository
	Site       *SiteRepository
	Configs    *ConfigRepository
	Tasks      *TaskRepository
}

func NewRepositories(pool *pgxpool.Pool) *Repositories {
	return &Repositories{
		Papers:     &PaperRepository{pool: pool},
		Discovery:  NewDiscoveryRepository(pool),
		Processing: NewProcessingRepository(pool),
		Drafts:     &DraftRepository{pool: pool},
		Site:       &SiteRepository{pool: pool},
		Configs:    &ConfigRepository{pool: pool},
		Tasks:      &TaskRepository{pool: pool},
	}
}
