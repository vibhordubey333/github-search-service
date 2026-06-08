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

	// Semaphore to bound concurrent GitHub API calls.
	// Buffered channel acts as a counting semaphore.
	// Acquiring: send to channel (blocks if full).
	// Releasing: receive from channel.
	sem := make(chan struct{}, s.maxConcurrency)

	// In v1: fetch page 1 only.
	// In v3 (pagination): loop over pages, each in its own goroutine,
	// all bounded by the same semaphore.
	pages := []int{1}

	g, gCtx := errgroup.WithContext(ctx)

	var mu sync.Mutex
	var allResults []SearchResult

	for _, page := range pages {
		page := page // capture loop variable — critical for goroutine correctness

		g.Go(func() error {
			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }() // Release on return
			case <-gCtx.Done():
				return gCtx.Err()
			}

			params := github.SearchParams{
				Query:   query,
				PerPage: 30,
				Page:    page,
			}

			resp, err := s.github.SearchCode(gCtx, params)
			if err != nil {
				return fmt.Errorf("github search page %d: %w", page, err)
			}

			results := make([]SearchResult, 0, len(resp.Items))
			for _, item := range resp.Items {
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
		return fmt.Errorf("search_term is required")
	}
	if len(input.SearchTerm) > 256 {
		return fmt.Errorf("search_term exceeds maximum length")
	}

	forbiddenChars := []string{":", "\"", "(", ")"}
	for _, ch := range forbiddenChars {
		if strings.Contains(input.User, ch) {
			return fmt.Errorf("user field contains invalid characters")
		}
	}
	return nil
}
