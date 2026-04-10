package pdfworker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"arxivagent/internal/config"
)

type Client struct {
	pythonBin    string
	parserScript string
}

type ParseRequest struct {
	PDFPath         string
	OutputImagesDir string
	PaperTitle      string
	ArxivID         string
}

type ParseResponse struct {
	PaperTitle     string                   `json:"paper_title"`
	ArxivID        string                   `json:"arxiv_id"`
	PDFMetadata    map[string]interface{}   `json:"pdf_metadata"`
	PDFPageCount   int                      `json:"pdf_page_count"`
	SectionOutline []map[string]interface{} `json:"section_outline"`
	ParsedSections map[string]string        `json:"parsed_sections"`
	Summary        map[string]interface{}   `json:"summary"`
	Figures        []Figure                 `json:"figures"`
	RawTextExcerpt string                   `json:"raw_text_excerpt"`
}

type Figure struct {
	FigureIndex int    `json:"figure_index"`
	PageNo      int    `json:"page_no"`
	FileName    string `json:"file_name"`
	LocalPath   string `json:"local_path"`
	MimeType    string `json:"mime_type"`
	SizeBytes   int    `json:"size_bytes"`
	Caption     string `json:"caption"`
}

func NewClient(cfg config.WorkerConfig) *Client {
	return &Client{
		pythonBin:    cfg.PythonBin,
		parserScript: cfg.ParserScript,
	}
}

func (c *Client) Parse(ctx context.Context, req ParseRequest) (*ParseResponse, error) {
	cmd := exec.CommandContext(
		ctx,
		c.pythonBin,
		c.parserScript,
		"--pdf", req.PDFPath,
		"--output-images-dir", req.OutputImagesDir,
		"--paper-title", req.PaperTitle,
		"--arxiv-id", req.ArxivID,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("run parser: %w: %s", err, string(output))
	}

	var resp ParseResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		return nil, fmt.Errorf("decode parser output: %w", err)
	}
	return &resp, nil
}
