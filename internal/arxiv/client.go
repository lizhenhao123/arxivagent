package arxiv

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"arxivagent/internal/config"
)

type Client struct {
	baseURL    string
	userAgent  string
	httpClient *http.Client
}

type Paper struct {
	ArxivID         string
	VersionNo       int
	Title           string
	Authors         []Author
	Abstract        string
	PrimaryCategory string
	Categories      []string
	PublishedAt     time.Time
	UpdatedAt       time.Time
	PDFURL          string
	SourceURL       string
	RawID           string
}

type Author struct {
	Name string `json:"name"`
}

type QueryOptions struct {
	SearchQuery string
	Start       int
	MaxResults  int
	SortBy      string
	SortOrder   string
}

type atomFeed struct {
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	ID              string         `xml:"id"`
	Updated         string         `xml:"updated"`
	Published       string         `xml:"published"`
	Title           string         `xml:"title"`
	Summary         string         `xml:"summary"`
	Authors         []atomAuthor   `xml:"author"`
	Links           []atomLink     `xml:"link"`
	Categories      []atomCategory `xml:"category"`
	PrimaryCategory atomCategory   `xml:"primary_category"`
}

type atomAuthor struct {
	Name string `xml:"name"`
}

type atomLink struct {
	Title string `xml:"title,attr"`
	Href  string `xml:"href,attr"`
}

type atomCategory struct {
	Term string `xml:"term,attr"`
}

var versionPattern = regexp.MustCompile(`v(\d+)$`)

func NewClient(cfg config.ArxivConfig) *Client {
	return &Client{
		baseURL:   cfg.BaseURL,
		userAgent: cfg.UserAgent,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Search(ctx context.Context, opts QueryOptions) ([]Paper, error) {
	values := url.Values{}
	values.Set("search_query", opts.SearchQuery)
	values.Set("start", fmt.Sprintf("%d", opts.Start))
	values.Set("max_results", fmt.Sprintf("%d", opts.MaxResults))
	if opts.SortBy != "" {
		values.Set("sortBy", opts.SortBy)
	}
	if opts.SortOrder != "" {
		values.Set("sortOrder", opts.SortOrder)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("arxiv api status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var feed atomFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, err
	}

	papers := make([]Paper, 0, len(feed.Entries))
	for _, entry := range feed.Entries {
		paper, err := convertEntry(entry)
		if err != nil {
			return nil, err
		}
		papers = append(papers, paper)
	}
	return papers, nil
}

func convertEntry(entry atomEntry) (Paper, error) {
	publishedAt, err := time.Parse(time.RFC3339, entry.Published)
	if err != nil {
		return Paper{}, err
	}
	updatedAt, err := time.Parse(time.RFC3339, entry.Updated)
	if err != nil {
		return Paper{}, err
	}

	rawID := entry.ID
	baseID, versionNo := splitArxivID(rawID)

	authors := make([]Author, 0, len(entry.Authors))
	for _, author := range entry.Authors {
		authors = append(authors, Author{Name: strings.TrimSpace(author.Name)})
	}

	categories := make([]string, 0, len(entry.Categories))
	for _, category := range entry.Categories {
		if term := strings.TrimSpace(category.Term); term != "" {
			categories = append(categories, term)
		}
	}

	pdfURL := ""
	sourceURL := strings.TrimSpace(entry.ID)
	for _, link := range entry.Links {
		if link.Title == "pdf" || strings.Contains(link.Href, "/pdf/") {
			pdfURL = strings.TrimSpace(link.Href)
			break
		}
	}
	if pdfURL == "" && baseID != "" {
		pdfURL = "https://arxiv.org/pdf/" + baseID + ".pdf"
	}

	return Paper{
		ArxivID:         baseID,
		VersionNo:       versionNo,
		Title:           normalizeWhitespace(entry.Title),
		Authors:         authors,
		Abstract:        normalizeWhitespace(entry.Summary),
		PrimaryCategory: strings.TrimSpace(entry.PrimaryCategory.Term),
		Categories:      categories,
		PublishedAt:     publishedAt,
		UpdatedAt:       updatedAt,
		PDFURL:          pdfURL,
		SourceURL:       sourceURL,
		RawID:           rawID,
	}, nil
}

func splitArxivID(raw string) (string, int) {
	trimmed := strings.TrimSpace(raw)
	if idx := strings.LastIndex(trimmed, "/"); idx >= 0 {
		trimmed = trimmed[idx+1:]
	}
	matches := versionPattern.FindStringSubmatch(trimmed)
	if len(matches) == 2 {
		version := 1
		fmt.Sscanf(matches[1], "%d", &version)
		return strings.TrimSuffix(trimmed, matches[0]), version
	}
	return trimmed, 1
}

func normalizeWhitespace(v string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(v)), " ")
}
