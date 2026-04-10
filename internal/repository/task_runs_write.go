package repository

import (
	"context"
	"time"
)

func (r *TaskRepository) Create(ctx context.Context, taskType string, bizDate time.Time, triggerSource string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO task_runs (task_type, biz_date, status, trigger_source, started_at)
		VALUES ($1, $2, 'RUNNING', $3, NOW())
		RETURNING id
	`, taskType, bizDate.Format("2006-01-02"), triggerSource).Scan(&id)
	return id, err
}

func (r *TaskRepository) Finish(ctx context.Context, id int64, status string, resultSummary []byte, errorMessage *string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE task_runs
		SET status = $2,
			ended_at = NOW(),
			duration_ms = GREATEST(0, FLOOR(EXTRACT(EPOCH FROM (NOW() - started_at)) * 1000)),
			result_summary = COALESCE($3::jsonb, '{}'::jsonb),
			error_message = $4
		WHERE id = $1
	`, id, status, string(resultSummary), errorMessage)
	return err
}
