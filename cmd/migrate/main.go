package main

import (
	"context"
	"log"
	"os"

	"arxivagent/internal/config"
	"arxivagent/internal/db"
)

func main() {
	cfgPath := config.DefaultConfigPath()
	if envPath := os.Getenv("ARXIV_AGENT_CONFIG"); envPath != "" {
		cfgPath = envPath
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	pool, err := db.NewPool(context.Background(), cfg)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	if err := db.RunEmbeddedMigrations(context.Background(), pool); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	log.Println("migrations completed")
}
