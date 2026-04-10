package service

import (
	"context"

	"arxivagent/internal/model"
	"arxivagent/internal/repository"
)

type TaskService struct {
	repo *repository.TaskRepository
}

func (s *TaskService) List(ctx context.Context) ([]model.TaskRun, error) {
	return s.repo.List(ctx)
}
