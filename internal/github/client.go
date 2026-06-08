package github

import (
	"context"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Searcher interface {
	Searcher(ctx context.Context, params SearchParams) (*SearchParams, error)
}

type ErrorRateLimit struct {
	RetryAfter time.Duration
}

// errorRateLimti is returned when Github response with 429 or 403
func (e *ErrorRateLimit) Error() string {
	return fmt.Sprintf("Github rate limit exceeded. Retry after %s", e.RetryAfter)
}

// ErrorUnexpectedStatus is wrapping non 200 HTTP codes. These are not rate limits
type ErrorUnexpectedStatus struct {
	StatusCode int
	Body       string
}

func (e *ErrorUnexpectedStatus) Error() string {
	return fmt.Sprintf("github API returned %d: %s", e.StatusCode, e.Body)
}

type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	logger     *zap.Logger
	maxRetries int
}

func NewClient(baseURL, token string, timeout time.Duration, logger *zap.Logger) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL:    baseURL,
		token:      token,
		logger:     logger,
		maxRetries: 3,
	}
}

func (c *Client) SearchCode(ctx context.Context, params SearchParams) (*SearchResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {

			backoff := time.Duration(100*math.Pow(2, float64(attempt))) * time.Millisecond // 100 , 200 ,400 ms
			c.logger.Info("retrying github request",
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff),
			)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		resp, err := c.doSearch(ctx, params)
		if err != nil {
			// Re-trying n/w errors
			lastErr = err
			c.logger.Warn("github request failed", zap.Error(err), zap.Int("attempt", attempt))
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("github search failed after %d attempts: %w", c.maxRetries, lastErr)
}

func (c *Client) doSearch(ctx context.Context, params SearchParams) (*SearchResponse, error) {
	reqURL := fmt.Sprintf("%s/search/code", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	q := url.Values{}
	q.Set("q", params.Query)
	q.Set("per_page", strconv.Itoa(params.PerPage))
	q.Set("page", strconv.Itoa(params.Page))
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	start := time.Now()
	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer httpResp.Body.Close()

	c.logger.Debug("github api call",
		zap.String("url", req.URL.String()),
		zap.Int("status", httpResp.StatusCode),
		zap.Duration("latency", time.Since(start)),
	)

	// Rate limit detection. GitHub returns 403 with X-RateLimit-Remaining: 0 for primary rate limits  and 429 with Retry-After for secondary rate limits.
	if httpResp.StatusCode == http.StatusTooManyRequests ||
		(httpResp.StatusCode == http.StatusForbidden &&
			httpResp.Header.Get("X-RateLimit-Remaining") == "0") {

		retryAfter := parseRetryAfter(httpResp.Header.Get("Retry-After"))
		return nil, &ErrorRateLimit{RetryAfter: retryAfter}
	}

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(httpResp.Body, 1024))
		// 4xx series errors (except 429) are not retryable — return immediately
		return nil, &ErrorUnexpectedStatus{
			StatusCode: httpResp.StatusCode,
			Body:       string(body),
		}
	}

	var result SearchResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

func parseRetryAfter(header string) time.Duration {
	if header == "" {
		return 60 * time.Second
	}
	secs, err := strconv.Atoi(header)
	if err != nil {
		return 60 * time.Second
	}
	return time.Duration(secs) * time.Second
}
