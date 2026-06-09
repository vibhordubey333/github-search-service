package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vibhordubey333/github-service/internal/github"
	"go.uber.org/zap"
)

type mockGithubSearcher struct {
	SearchCodeFunc func(ctx context.Context, params github.SearchParams) (*github.SearchResponse, error)
}

func (m *mockGithubSearcher) SearchCode(ctx context.Context, params github.SearchParams) (*github.SearchResponse, error) {
	if m.SearchCodeFunc != nil {
		return m.SearchCodeFunc(ctx, params)
	}
	return nil, nil
}

func TestSearchService_Search(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name           string
		input          SearchInput
		mockSearchCode func(ctx context.Context, params github.SearchParams) (*github.SearchResponse, error)
		wantLen        int
		wantErr        bool
		errContains    string
	}{
		{
			name: "successful single-page search",
			input: SearchInput{
				SearchTerm: "test",
			},
			mockSearchCode: func(ctx context.Context, params github.SearchParams) (*github.SearchResponse, error) {
				return &github.SearchResponse{
					TotalCount: 2,
					Items: []github.SearchItem{
						{HTMLURL: "url1", Repository: github.Repository{FullName: "repo1"}},
						{HTMLURL: "url2", Repository: github.Repository{FullName: "repo2"}},
					},
				}, nil
			},
			wantLen: 2,
			wantErr: false,
		},
		{
			name: "successful multi-page concurrent search",
			input: SearchInput{
				SearchTerm: "test",
			},
			mockSearchCode: func(ctx context.Context, params github.SearchParams) (*github.SearchResponse, error) {
				if params.Page == 1 {
					return &github.SearchResponse{
						TotalCount: 35, // 2 pages (30 per page)
						Items: make([]github.SearchItem, 30),
					}, nil
				}
				if params.Page == 2 {
					return &github.SearchResponse{
						TotalCount: 35,
						Items: make([]github.SearchItem, 5),
					}, nil
				}
				return nil, errors.New("unexpected page")
			},
			wantLen: 35,
			wantErr: false,
		},
		{
			name: "validation error empty search term",
			input: SearchInput{
				SearchTerm: "",
			},
			mockSearchCode: func(ctx context.Context, params github.SearchParams) (*github.SearchResponse, error) {
				t.Fatal("SearchCode should not be called")
				return nil, nil
			},
			wantLen:     0,
			wantErr:     true,
			errContains: "search_term is required",
		},
		{
			name: "github client error",
			input: SearchInput{
				SearchTerm: "test",
			},
			mockSearchCode: func(ctx context.Context, params github.SearchParams) (*github.SearchResponse, error) {
				return nil, errors.New("github api error")
			},
			wantLen:     0,
			wantErr:     true,
			errContains: "github api error",
		},
		{
			name: "validation error search term too long",
			input: SearchInput{
				SearchTerm: strings.Repeat("a", 257),
			},
			mockSearchCode: func(ctx context.Context, params github.SearchParams) (*github.SearchResponse, error) {
				t.Fatal("SearchCode should not be called")
				return nil, nil
			},
			wantLen:     0,
			wantErr:     true,
			errContains: "search_term exceeds maximum length",
		},
		{
			name: "validation error invalid user characters",
			input: SearchInput{
				SearchTerm: "test",
				User:       "invalid:user",
			},
			mockSearchCode: func(ctx context.Context, params github.SearchParams) (*github.SearchResponse, error) {
				t.Fatal("SearchCode should not be called")
				return nil, nil
			},
			wantLen:     0,
			wantErr:     true,
			errContains: "user field contains invalid characters",
		},
		{
			name: "github search page 2 error",
			input: SearchInput{
				SearchTerm: "test",
			},
			mockSearchCode: func(ctx context.Context, params github.SearchParams) (*github.SearchResponse, error) {
				if params.Page == 1 {
					return &github.SearchResponse{
						TotalCount: 35, // 2 pages (30 per page)
						Items:      make([]github.SearchItem, 30),
					}, nil
				}
				if params.Page == 2 {
					return nil, errors.New("api error on page 2")
				}
				return nil, errors.New("unexpected page")
			},
			wantLen:     0,
			wantErr:     true,
			errContains: "github search page 2: api error on page 2",
		},
		{
			name: "context cancellation",
			input: SearchInput{
				SearchTerm: "test",
			},
			mockSearchCode: func(ctx context.Context, params github.SearchParams) (*github.SearchResponse, error) {
				// The initial page fetch checks the context
				return nil, ctx.Err()
			},
			wantLen:     0,
			wantErr:     true,
			errContains: "context canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockGithubSearcher{
				SearchCodeFunc: tt.mockSearchCode,
			}
			svc := NewSearchService(mock, logger, 5)

			ctx := context.Background()
			if strings.Contains(tt.name, "context cancellation") {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			results, err := svc.Search(ctx, tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else {
					if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
						t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
					}
					if strings.Contains(tt.name, "validation error") && !errors.Is(err, ErrorInvalidInput) {
						t.Errorf("expected error to wrap ErrorInvalidInput, got %v", err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(results) != tt.wantLen {
					t.Errorf("expected %d results, got %d", tt.wantLen, len(results))
				}
			}
		})
	}
}
