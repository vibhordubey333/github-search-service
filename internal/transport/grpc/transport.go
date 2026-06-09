/*
 Receive proto request → call service → return proto response.
*/
package grpc

import (
	"context"
	"errors"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	searchv1 "github.com/vibhordubey333/github-service/api/proto/v1"
	"github.com/vibhordubey333/github-service/internal/github"
	"github.com/vibhordubey333/github-service/internal/service"
)

type SearchHandler struct {
	searchv1.UnimplementedGithubSearchServiceServer
	svc    *service.SearchService
	logger *zap.Logger
}

func NewSearchHandler(svc *service.SearchService, logger *zap.Logger) *SearchHandler {
	return &SearchHandler{svc: svc, logger: logger}
}

func (h *SearchHandler) Search(ctx context.Context, req *searchv1.SearchRequest) (*searchv1.SearchResponse, error) {
	input := service.SearchInput{
		SearchTerm: req.GetSearchTerm(),
		User:       req.GetUser(),
	}

	results, err := h.svc.Search(ctx, input)
	if err != nil {
		return nil, mapError(err)
	}

	protoResults := make([]*searchv1.Result, 0, len(results))
	for _, r := range results {
		protoResults = append(protoResults, &searchv1.Result{
			FileUrl: r.FileURL,
			Repo:    r.Repo,
		})
	}

	return &searchv1.SearchResponse{Results: protoResults}, nil
}

func mapError(err error) error {
	var rateLimitErr *github.ErrorRateLimit
	var unexpectedErr *github.ErrorUnexpectedStatus

	switch {
	case errors.As(err, &rateLimitErr):
		return status.Errorf(codes.ResourceExhausted,
			"github rate limit exceeded, retry after %s", rateLimitErr.RetryAfter)

	case errors.As(err, &unexpectedErr):
		if unexpectedErr.StatusCode == 422 {
			// GitHub 422 = validation failed (e.g., empty query)
			return status.Errorf(codes.InvalidArgument, "invalid search query: %s", unexpectedErr.Body)
		}
		if unexpectedErr.StatusCode >= 500 {
			return status.Errorf(codes.Unavailable, "github is unavailable: %d", unexpectedErr.StatusCode)
		}
		return status.Errorf(codes.Internal, "unexpected github response: %d", unexpectedErr.StatusCode)

	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request timed out")

	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")

	default:
		// Check for validation errors from the service layer
		if isValidationError(err) {
			return status.Errorf(codes.InvalidArgument, err.Error())
		}
		return status.Errorf(codes.Internal, "internal error: %s", err.Error())
	}
}

func isValidationError(err error) bool {
	// In production, use a typed sentinel or errors.As with a ValidationError type.
	// For clarity here we use string prefix. In real code: define a ValidationError struct.
	return errors.Is(err, service.ErrorInvalidInput)
}
