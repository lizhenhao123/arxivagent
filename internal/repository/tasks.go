package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"arxivagent/internal/model"
)

type TaskRepository struct {
	pool *pgxpool.Pool
}

func (r *TaskRepository) List(ctx context.Context) ([]model.TaskRun, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, task_type, biz_date::text, status, trigger_source, started_at, ended_at, error_message
		FROM task_runs
		ORDER BY started_at DESC
		LIMIT 50
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.TaskRun
	for rows.Next() {
		var item model.TaskRun
		if err := rows.Scan(&item.ID, &item.TaskType, &item.BizDate, &item.Status, &item.TriggerSource, &item.StartedAt, &item.EndedAt, &item.ErrorMessage); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
