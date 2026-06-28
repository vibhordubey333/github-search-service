package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/vibhordubey333/github-service/internal/github"
)

type SearchResult struct {
	FileURL string
	Repo    string
}

// SearchInput is the service-layer input type.
type SearchInput struct {
	SearchTerm string
	User       string
}

type GithubSearcher interface {
	SearchCode(ctx context.Context, params github.SearchParams) (*github.SearchResponse, error)
}

type SearchService struct {
	github         GithubSearcher
	logger         *zap.Logger
	maxConcurrency int
}

func NewSearchService(gh GithubSearcher, logger *zap.Logger, maxConcurrency int) *SearchService {
	return &SearchService{
		github:         gh,
		logger:         logger,
		maxConcurrency: maxConcurrency,
	}
}

/*
Search, aggregate return results.

Concurrency design:
1. Full context propagation — client cancellation stops GitHub calls immediately.
2. Structure is ready for concurrent multi-page fetching .
3. errgroup for clean goroutine + error management.

 errgroup over raw goroutines + WaitGroup?
- errgroup.WithContext cancels all goroutines on first error
- No need to manually coordinate error channels
- Cleaner code, fewer race conditions
*/

func (s *SearchService) Search(ctx context.Context, input SearchInput) ([]SearchResult, error) {
	if err := validateInput(input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	query := buildQuery(input)
	s.logger.Info("executing search",
		zap.String("query", query),
		zap.String("user", input.User),
	)

	// Fetch page 1 synchronously to get total count
	params := github.SearchParams{
		Query:   query,
		PerPage: 30,
		Page:    1,
	}

	resp, err := s.github.SearchCode(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("github search page 1: %w", err)
	}

	var allResults []SearchResult
	for _, item := range resp.Items {
		allResults = append(allResults, SearchResult{
			FileURL: item.HTMLURL,
			Repo:    item.Repository.FullName,
		})
	}

	// Calculate total pages (cap at 90 results / 3 pages to stay well within GitHub's 10 req/min code_search limit)
	totalCount := resp.TotalCount
	if totalCount > 90 {
		totalCount = 90
	}
	totalPages := (totalCount + 29) / 30

	if totalPages > 1 {
		// Semaphore to bound concurrent GitHub API calls.
		sem := make(chan struct{}, s.maxConcurrency)
		g, gCtx := errgroup.WithContext(ctx)
		var mu sync.Mutex

		for page := 2; page <= totalPages; page++ {
			page := page

			g.Go(func() error {
				// Acquire semaphore
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }() // Release on return
				case <-gCtx.Done():
					return gCtx.Err()
				}

				pageParams := github.SearchParams{
					Query:   query,
					PerPage: 30,
					Page:    page,
				}

				pageResp, err := s.github.SearchCode(gCtx, pageParams)
				if err != nil {
					return fmt.Errorf("github search page %d: %w", page, err)
				}

				results := make([]SearchResult, 0, len(pageResp.Items))
				for _, item := range pageResp.Items {
					results = append(results, SearchResult{
						FileURL: item.HTMLURL,
						Repo:    item.Repository.FullName,
					})
				}

				mu.Lock()
				allResults = append(allResults, results...)
				mu.Unlock()

				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return nil, err
		}
	}

	return allResults, nil
}

func buildQuery(input SearchInput) string {
	parts := []string{input.SearchTerm}
	if input.User != "" {
		parts = append(parts, "user:"+input.User)
	}
	return strings.Join(parts, " ")
}

func validateInput(input SearchInput) error {
	if strings.TrimSpace(input.SearchTerm) == "" {
		return fmt.Errorf("%w: search_term is required", ErrorInvalidInput)
	}
	if len(input.SearchTerm) > 256 {
		return fmt.Errorf("%w: search_term exceeds maximum length", ErrorInvalidInput)
	}

	forbiddenChars := []string{":", "\"", "(", ")"}
	for _, ch := range forbiddenChars {
		if strings.Contains(input.User, ch) {
			return fmt.Errorf("%w: user field contains invalid characters", ErrorInvalidInput)
		}
	}
	return nil
}
