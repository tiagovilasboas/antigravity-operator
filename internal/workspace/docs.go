package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const DefaultDocsBaseURL = "https://docs.googleapis.com/v1"

var docURLRegex = regexp.MustCompile(`/document/(?:u/\d+/)?d/([a-zA-Z0-9-_]+)`)

// Document represents an ingested or created Google Doc.
type Document struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	BodyText   string `json:"body_text"`
	RevisionID string `json:"revision_id,omitempty"`
	URL        string `json:"url,omitempty"`
}

// DocsClient interacts with the Google Docs API v1.
type DocsClient struct {
	Client      *http.Client
	BaseURL     string
	TokenSource func(ctx context.Context) (string, error)
}

// NewDocsClient instantiates a DocsClient with defaults.
func NewDocsClient(client *http.Client, baseURL string, tokenSource func(ctx context.Context) (string, error)) *DocsClient {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if baseURL == "" {
		baseURL = DefaultDocsBaseURL
	}
	return &DocsClient{
		Client:      client,
		BaseURL:     baseURL,
		TokenSource: tokenSource,
	}
}

// ExtractDocumentID parses a document ID from a full Google Docs URL or returns the raw ID.
func ExtractDocumentID(input string) string {
	input = strings.TrimSpace(input)
	if m := docURLRegex.FindStringSubmatch(input); len(m) > 1 {
		return m[1]
	}
	return input
}

// GetDocument retrieves the document metadata and extracts all plain-text paragraphs.
func (c *DocsClient) GetDocument(ctx context.Context, docIDOrURL string) (*Document, error) {
	docID := ExtractDocumentID(docIDOrURL)
	if docID == "" {
		return nil, errors.New("document id or url is required")
	}

	token, err := c.TokenSource(ctx)
	if err != nil {
		return nil, fmt.Errorf("get auth token: %w", err)
	}

	endpoint := fmt.Sprintf("%s/documents/%s", c.BaseURL, url.PathEscape(docID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docs get request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docs api error (%d): %s", resp.StatusCode, string(body))
	}

	var raw struct {
		DocumentID string `json:"documentId"`
		Title      string `json:"title"`
		RevisionID string `json:"revisionId"`
		Body       struct {
			Content []struct {
				Paragraph *struct {
					Elements []struct {
						TextRun *struct {
							Content string `json:"content"`
						} `json:"textRun"`
					} `json:"elements"`
				} `json:"paragraph"`
			} `json:"content"`
		} `json:"body"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse docs json: %w", err)
	}

	var sb strings.Builder
	for _, c := range raw.Body.Content {
		if c.Paragraph != nil {
			for _, elem := range c.Paragraph.Elements {
				if elem.TextRun != nil {
					sb.WriteString(elem.TextRun.Content)
				}
			}
		}
	}

	return &Document{
		ID:         raw.DocumentID,
		Title:      raw.Title,
		RevisionID: raw.RevisionID,
		BodyText:   sb.String(),
		URL:        fmt.Sprintf("https://docs.google.com/document/d/%s/edit", raw.DocumentID),
	}, nil
}

// CreateDocument creates a new Google Doc and optionally appends initial text content.
func (c *DocsClient) CreateDocument(ctx context.Context, title string, content string) (*Document, error) {
	if title == "" {
		return nil, errors.New("document title cannot be empty")
	}

	token, err := c.TokenSource(ctx)
	if err != nil {
		return nil, fmt.Errorf("get auth token: %w", err)
	}

	// 1. Create document
	endpoint := fmt.Sprintf("%s/documents", c.BaseURL)
	createPayload := map[string]string{
		"title": title,
	}
	createData, _ := json.Marshal(createPayload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(createData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docs create request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("docs create api error (%d): %s", resp.StatusCode, string(body))
	}

	var created struct {
		DocumentID string `json:"documentId"`
		Title      string `json:"title"`
		RevisionID string `json:"revisionId"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		return nil, fmt.Errorf("parse docs create response: %w", err)
	}

	doc := &Document{
		ID:         created.DocumentID,
		Title:      created.Title,
		RevisionID: created.RevisionID,
		URL:        fmt.Sprintf("https://docs.google.com/document/d/%s/edit", created.DocumentID),
		BodyText:   content,
	}

	// 2. Append content via batchUpdate if provided
	if strings.TrimSpace(content) != "" {
		batchEndpoint := fmt.Sprintf("%s/documents/%s:batchUpdate", c.BaseURL, url.PathEscape(created.DocumentID))
		batchPayload := map[string]interface{}{
			"requests": []map[string]interface{}{
				{
					"insertText": map[string]interface{}{
						"location": map[string]int{"index": 1},
						"text":     content,
					},
				},
			},
		}
		batchData, _ := json.Marshal(batchPayload)

		bReq, err := http.NewRequestWithContext(ctx, http.MethodPost, batchEndpoint, bytes.NewReader(batchData))
		if err != nil {
			return doc, fmt.Errorf("prepare content request: %w", err)
		}
		bReq.Header.Set("Authorization", "Bearer "+token)
		bReq.Header.Set("Content-Type", "application/json")
		bReq.Header.Set("Accept", "application/json")

		bResp, bErr := c.Client.Do(bReq)
		if bErr != nil {
			return doc, fmt.Errorf("append content request: %w", bErr)
		}
		defer bResp.Body.Close()
		if bResp.StatusCode != http.StatusOK {
			bBody, _ := io.ReadAll(bResp.Body)
			return doc, fmt.Errorf("append content api error (%d): %s", bResp.StatusCode, string(bBody))
		}
	}

	return doc, nil
}
