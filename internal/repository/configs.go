package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"

	"arxivagent/internal/model"
)

type ConfigRepository struct {
	pool *pgxpool.Pool
}

func (r *ConfigRepository) List(ctx context.Context) ([]model.SystemConfig, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT config_key, config_value, description, updated_at
		FROM system_configs
		ORDER BY config_key ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.SystemConfig
	for rows.Next() {
		var item model.SystemConfig
		if err := rows.Scan(&item.Key, &item.Value, &item.Description, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *ConfigRepository) GetByKey(ctx context.Context, key string) (*model.SystemConfig, error) {
	var item model.SystemConfig
	err := r.pool.QueryRow(ctx, `
		SELECT config_key, config_value, description, updated_at
		FROM system_configs
		WHERE config_key = $1
	`, key).Scan(&item.Key, &item.Value, &item.Description, &item.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if item.Key == "" {
		return nil, errors.New("empty config key")
	}
	return &item, nil
}
