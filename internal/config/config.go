package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Server    ServerConfig    `json:"server"`
	Arxiv     ArxivConfig     `json:"arxiv"`
	Postgres  PostgresConfig  `json:"postgres"`
	Redis     RedisConfig     `json:"redis"`
	Storage   StorageConfig   `json:"storage"`
	Site      SiteConfig      `json:"site"`
	Worker    WorkerConfig    `json:"worker"`
	LLM       LLMConfig       `json:"llm"`
	Scheduler SchedulerConfig `json:"scheduler"`
}

type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type ArxivConfig struct {
	BaseURL            string `json:"base_url"`
	UserAgent          string `json:"user_agent"`
	MaxResults         int    `json:"max_results"`
	RequestIntervalSec int    `json:"request_interval_seconds"`
}

type PostgresConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
	SSLMode  string `json:"sslmode"`
}

type RedisConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password"`
	DB       int    `json:"db"`
}

type StorageConfig struct {
	PDFDir      string `json:"pdf_dir"`
	ImageDir    string `json:"image_dir"`
	MarkdownDir string `json:"markdown_dir"`
}

type SiteConfig struct {
	BaseURL         string `json:"base_url"`
	DraftPathPrefix string `json:"draft_path_prefix"`
}

type WorkerConfig struct {
	PythonBin    string `json:"python_bin"`
	ParserScript string `json:"parser_script"`
}

type LLMConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

type SchedulerConfig struct {
	Timezone     string `json:"timezone"`
	DailyRunTime string `json:"daily_run_time"`
}

func DefaultConfigPath() string {
	return filepath.Join("configs", "runtime.local.json")
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", path, err)
	}

	cfg.applyDefaults()
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Server.Host == "" {
		c.Server.Host = "127.0.0.1"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Arxiv.BaseURL == "" {
		c.Arxiv.BaseURL = "https://export.arxiv.org/api/query"
	}
	if c.Arxiv.UserAgent == "" {
		c.Arxiv.UserAgent = "arxivagent/0.1 (+local)"
	}
	if c.Arxiv.MaxResults == 0 {
		c.Arxiv.MaxResults = 50
	}
	if c.Arxiv.RequestIntervalSec == 0 {
		c.Arxiv.RequestIntervalSec = 3
	}
	if c.Postgres.SSLMode == "" {
		c.Postgres.SSLMode = "disable"
	}
	if c.Site.DraftPathPrefix == "" {
		c.Site.DraftPathPrefix = "/drafts"
	}
	if c.Worker.PythonBin == "" {
		c.Worker.PythonBin = "python"
	}
	if c.Worker.ParserScript == "" {
		c.Worker.ParserScript = filepath.Join("worker", "parse_pdf.py")
	}
}

func (c ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func (c PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User,
		c.Password,
		c.Host,
		c.Port,
		c.Database,
		c.SSLMode,
	)
}
