package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/org/nexus/internal/models"
)

// Client is an HTTP client for the Nexus API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a new Nexus API client.
func New(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// Log sends a snapshot to POST /api/v1/log.
func (c *Client) Log(ctx context.Context, req models.LogRequest) (*models.LogResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/log", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("unexpected status %d: %v", resp.StatusCode, errResp)
	}

	var logResp models.LogResponse
	if err := json.NewDecoder(resp.Body).Decode(&logResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &logResp, nil
}

// GetSnapshot retrieves a snapshot by hash.
func (c *Client) GetSnapshot(ctx context.Context, hash string) (*models.Snapshot, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/snapshots/"+hash, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("snapshot not found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var snapshot models.Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &snapshot, nil
}

// ListSnapshots queries snapshots with optional filters.
func (c *Client) ListSnapshots(ctx context.Context, query models.SnapshotQuery) (*models.SnapshotListResponse, error) {
	u, err := url.Parse(c.baseURL + "/api/v1/snapshots")
	if err != nil {
		return nil, fmt.Errorf("parsing URL: %w", err)
	}

	q := u.Query()
	if query.Path != "" {
		q.Set("path", query.Path)
	}
	if query.StatusCode > 0 {
		q.Set("statusCode", strconv.Itoa(query.StatusCode))
	}
	if query.Method != "" {
		q.Set("method", query.Method)
	}
	if query.Limit > 0 {
		q.Set("limit", strconv.FormatInt(query.Limit, 10))
	}
	if query.Offset > 0 {
		q.Set("offset", strconv.FormatInt(query.Offset, 10))
	}
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var listResp models.SnapshotListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &listResp, nil
}
