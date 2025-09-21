package multifetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
)

// HTTPFetcher handles HTTP/Gateway-based fetching
type HTTPFetcher struct {
	client *http.Client
}

// NewHTTPFetcher creates a new HTTP fetcher
func NewHTTPFetcher() *HTTPFetcher {
	return &HTTPFetcher{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     30 * time.Second,
				DisableCompression:  false,
				MaxConnsPerHost:     5,
				ResponseHeaderTimeout: 10 * time.Second,
			},
		},
	}
}

// Fetch retrieves content via HTTP gateway
func (hf *HTTPFetcher) Fetch(ctx context.Context, baseURL string, c cid.Cid, partialCAR bool) ([]byte, error) {
	// Construct the gateway URL
	var url string
	if partialCAR {
		// Use CAR format for partial content
		url = fmt.Sprintf("%s/ipfs/%s?format=car", strings.TrimSuffix(baseURL, "/"), c.String())
	} else {
		// Use raw format for single blocks
		url = fmt.Sprintf("%s/ipfs/%s?format=raw", strings.TrimSuffix(baseURL, "/"), c.String())
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set appropriate headers
	if partialCAR {
		req.Header.Set("Accept", "application/vnd.ipld.car")
	} else {
		req.Header.Set("Accept", "application/vnd.ipld.raw")
	}
	req.Header.Set("User-Agent", "boxo-multifetcher/1.0")

	// Execute request
	resp, err := hf.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Read response body
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

// FetchWithRange retrieves a specific byte range via HTTP
func (hf *HTTPFetcher) FetchWithRange(ctx context.Context, baseURL string, c cid.Cid, offset, length int64) ([]byte, error) {
	url := fmt.Sprintf("%s/ipfs/%s", strings.TrimSuffix(baseURL, "/"), c.String())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set range header
	if length > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+length-1))
	} else {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	req.Header.Set("User-Agent", "boxo-multifetcher/1.0")

	resp, err := hf.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Accept both 200 (full content) and 206 (partial content)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

// Close cleans up the HTTP fetcher
func (hf *HTTPFetcher) Close() error {
	hf.client.CloseIdleConnections()
	return nil
}