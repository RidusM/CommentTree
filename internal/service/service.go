package service

import (
	"comtree/internal/entity"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	pgxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
	"github.com/wb-go/wbf/dbpg/pgx-driver/transaction"
	"github.com/wb-go/wbf/logger"
)

const (
	_defaultMaxDepth     = 10
	_defaultPageSize     = 20
	_maxPageSize         = 100
	_slowOperationThresh = 200 * time.Millisecond
)

var (
	ErrCommentNotFound    = errors.New("comment not found")
	ErrParentNotFound     = errors.New("parent comment not found")
	ErrMaxDepthExceeded   = errors.New("maximum nesting depth exceeded")
	ErrInvalidPageSize    = errors.New("invalid page size")
	ErrInvalidSearchQuery = errors.New("invalid search query")
)

type (
	CommentRepository interface {
		Create(ctx context.Context, qe pgxdriver.QueryExecuter, comment entity.Comment) (*entity.Comment, error)
		GetByID(ctx context.Context, qe pgxdriver.QueryExecuter, id uuid.UUID) (*entity.Comment, error)
		GetChildren(ctx context.Context, qe pgxdriver.QueryExecuter, parentPath string, limit, offset int) ([]entity.Comment, error)
		GetRootComments(ctx context.Context, qe pgxdriver.QueryExecuter, limit, offset int) ([]entity.Comment, int64, error)
		SoftDelete(ctx context.Context, qe pgxdriver.QueryExecuter, path string) error
		Search(ctx context.Context, qe pgxdriver.QueryExecuter, searchQuery string, limit, offset int) ([]entity.Comment, int64, error)
	}

	CacheRepository interface {
		GetComment(ctx context.Context, id uuid.UUID) (*entity.Comment, error)
		SetComment(ctx context.Context, comment *entity.Comment) error
		DeleteComment(ctx context.Context, id uuid.UUID) error
		GetCommentTree(ctx context.Context, parentID *uuid.UUID, page, pageSize int) (*entity.CommentListResult, error)
		SetCommentTree(ctx context.Context, parentID *uuid.UUID, page, pageSize int, result *entity.CommentListResult) error
		InvalidateTree(ctx context.Context) error
	}

	CommentService struct {
		repo  CommentRepository
		cache CacheRepository
		tm    transaction.Manager
		log   logger.Logger

		maxDepth        int
		defaultPageSize int
		maxPageSize     int
	}

	CreateCommentRequest struct {
		ParentID *uuid.UUID `json:"parent_id,omitempty"`
		Author   string     `json:"author"`
		Content  string     `json:"content"`
	}

	GetCommentsRequest struct {
		ParentID *uuid.UUID
		Page     int
		PageSize int
	}

	SearchRequest struct {
		Query    string
		Page     int
		PageSize int
	}
)

func NewCommentService(
	repo CommentRepository,
	cache CacheRepository,
	tm transaction.Manager,
	log logger.Logger,
	opts ...Option,
) *CommentService {
	s := &CommentService{
		repo:            repo,
		cache:           cache,
		tm:              tm,
		log:             log,
		maxDepth:        _defaultMaxDepth,
		defaultPageSize: _defaultPageSize,
		maxPageSize:     _maxPageSize,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *CommentService) CreateComment(ctx context.Context, req CreateCommentRequest) (*entity.Comment, error) {
	const op = "service.CommentService.CreateComment"

	log := s.log.Ctx(ctx)
	startTime := time.Now()

	defer s.logSlowOperation(ctx, op, startTime, map[string]interface{}{
		"has_parent": req.ParentID != nil,
		"author":     req.Author,
	})

	log.LogAttrs(ctx, logger.InfoLevel, "create comment started",
		logger.String("op", op),
		logger.String("author", req.Author),
	)

	if err := s.validateCreateRequest(req); err != nil {
		log.LogAttrs(ctx, logger.ErrorLevel, "validation failed",
			logger.String("op", op),
			logger.Any("error", err),
		)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	var result *entity.Comment
	err := s.tm.ExecuteInTransaction(ctx, "create_comment", func(tx pgxdriver.QueryExecuter) error {
		var depth int

		if req.ParentID != nil {
			parent, err := s.repo.GetByID(ctx, tx, *req.ParentID)
			if err != nil {
				if errors.Is(err, entity.ErrDataNotFound) {
					return ErrParentNotFound
				}
				return fmt.Errorf("get parent: %w", err)
			}

			if parent.IsDeleted {
				return fmt.Errorf("parent is deleted: %w", ErrParentNotFound)
			}

			depth = parent.Depth + 1
			if depth > s.maxDepth {
				return ErrMaxDepthExceeded
			}
		} else {
			depth = 0
		}

		comment := entity.Comment{
			ParentID:  req.ParentID,
			Author:    req.Author,
			Content:   req.Content,
			IsDeleted: false,
			Depth:     depth,
		}

		created, err := s.repo.Create(ctx, tx, comment)
		if err != nil {
			return transaction.HandleError("create_comment", "create", err)
		}

		result = created
		return nil
	})

	if err != nil {
		log.LogAttrs(ctx, logger.ErrorLevel, "creation failed",
			logger.String("op", op),
			logger.Any("error", err),
		)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	_ = s.cache.InvalidateTree(ctx)

	log.LogAttrs(ctx, logger.InfoLevel, "comment created",
		logger.String("op", op),
		logger.String("id", result.ID.String()),
		logger.Duration("duration", time.Since(startTime)),
	)

	return result, nil
}

func (s *CommentService) GetComments(ctx context.Context, req GetCommentsRequest) (*entity.CommentListResult, error) {
	const op = "service.CommentService.GetComments"

	log := s.log.Ctx(ctx)
	startTime := time.Now()

	defer s.logSlowOperation(ctx, op, startTime, map[string]interface{}{
		"has_parent": req.ParentID != nil,
		"page":       req.Page,
	})

	if req.PageSize <= 0 {
		req.PageSize = s.defaultPageSize
	}
	if req.PageSize > s.maxPageSize {
		req.PageSize = s.maxPageSize
	}
	if req.Page < 1 {
		req.Page = 1
	}

	cached, err := s.cache.GetCommentTree(ctx, req.ParentID, req.Page, req.PageSize)
	if err == nil && cached != nil {
		log.LogAttrs(ctx, logger.InfoLevel, "served from cache",
			logger.String("op", op),
		)
		return cached, nil
	}

	var result *entity.CommentListResult
	err = s.tm.ExecuteInTransaction(ctx, "get_comments", func(tx pgxdriver.QueryExecuter) error {
		offset := (req.Page - 1) * req.PageSize

		if req.ParentID == nil {
			comments, total, err := s.repo.GetRootComments(ctx, tx, req.PageSize+1, offset)
			if err != nil {
				return fmt.Errorf("get root comments: %w", err)
			}

			trees := make([]entity.CommentTree, 0, len(comments))
			for _, comment := range comments {
				if len(trees) >= req.PageSize {
					break
				}

				tree, err := s.buildTree(ctx, tx, comment)
				if err != nil {
					return fmt.Errorf("build tree: %w", err)
				}
				trees = append(trees, tree)
			}

			totalPages := int((total + int64(req.PageSize) - 1) / int64(req.PageSize))

			result = &entity.CommentListResult{
				Comments:   trees,
				TotalCount: total,
				Page:       req.Page,
				PageSize:   req.PageSize,
				TotalPages: totalPages,
			}
		} else {
			parent, err := s.repo.GetByID(ctx, tx, *req.ParentID)
			if err != nil {
				if errors.Is(err, entity.ErrDataNotFound) {
					return ErrCommentNotFound
				}
				return fmt.Errorf("get parent: %w", err)
			}

			tree, err := s.buildTree(ctx, tx, *parent)
			if err != nil {
				return fmt.Errorf("build tree: %w", err)
			}

			result = &entity.CommentListResult{
				Comments:   []entity.CommentTree{tree},
				TotalCount: 1,
				Page:       req.Page,
				PageSize:   req.PageSize,
				TotalPages: 1,
			}
		}

		return nil
	})

	if err != nil {
		log.LogAttrs(ctx, logger.ErrorLevel, "get comments failed",
			logger.String("op", op),
			logger.Any("error", err),
		)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	_ = s.cache.SetCommentTree(ctx, req.ParentID, req.Page, req.PageSize, result)

	log.LogAttrs(ctx, logger.InfoLevel, "comments retrieved",
		logger.String("op", op),
		logger.Int("count", len(result.Comments)),
		logger.Duration("duration", time.Since(startTime)),
	)

	return result, nil
}

func (s *CommentService) DeleteComment(ctx context.Context, id uuid.UUID) error {
	const op = "service.CommentService.DeleteComment"

	log := s.log.Ctx(ctx)
	startTime := time.Now()

	defer s.logSlowOperation(ctx, op, startTime, map[string]interface{}{
		"id": id.String(),
	})

	log.LogAttrs(ctx, logger.InfoLevel, "delete comment started",
		logger.String("op", op),
		logger.String("id", id.String()),
	)

	err := s.tm.ExecuteInTransaction(ctx, "delete_comment", func(tx pgxdriver.QueryExecuter) error {
		comment, err := s.repo.GetByID(ctx, tx, id)
		if err != nil {
			if errors.Is(err, entity.ErrDataNotFound) {
				return ErrCommentNotFound
			}
			return fmt.Errorf("get comment: %w", err)
		}

		if comment.IsDeleted {
			return nil
		}

		if err := s.repo.SoftDelete(ctx, tx, comment.Path); err != nil {
			return transaction.HandleError("delete_comment", "soft_delete", err)
		}

		return nil
	})

	if err != nil {
		log.LogAttrs(ctx, logger.ErrorLevel, "delete failed",
			logger.String("op", op),
			logger.Any("error", err),
		)
		return fmt.Errorf("%s: %w", op, err)
	}

	_ = s.cache.DeleteComment(ctx, id)
	_ = s.cache.InvalidateTree(ctx)

	log.LogAttrs(ctx, logger.InfoLevel, "comment deleted",
		logger.String("op", op),
		logger.String("id", id.String()),
		logger.Duration("duration", time.Since(startTime)),
	)

	return nil
}

func (s *CommentService) SearchComments(ctx context.Context, req SearchRequest) (*entity.SearchResult, error) {
	const op = "service.CommentService.SearchComments"

	log := s.log.Ctx(ctx)
	startTime := time.Now()

	defer s.logSlowOperation(ctx, op, startTime, map[string]interface{}{
		"query": req.Query,
	})

	if strings.TrimSpace(req.Query) == "" {
		return nil, fmt.Errorf("%s: %w", op, ErrInvalidSearchQuery)
	}

	if req.PageSize <= 0 {
		req.PageSize = s.defaultPageSize
	}
	if req.PageSize > s.maxPageSize {
		req.PageSize = s.maxPageSize
	}
	if req.Page < 1 {
		req.Page = 1
	}

	var result *entity.SearchResult
	err := s.tm.ExecuteInTransaction(ctx, "search_comments", func(tx pgxdriver.QueryExecuter) error {
		offset := (req.Page - 1) * req.PageSize

		comments, total, err := s.repo.Search(ctx, tx, req.Query, req.PageSize, offset)
		if err != nil {
			return fmt.Errorf("search: %w", err)
		}

		result = &entity.SearchResult{
			Comments:   comments,
			TotalCount: total,
			Query:      req.Query,
		}

		return nil
	})

	if err != nil {
		log.LogAttrs(ctx, logger.ErrorLevel, "search failed",
			logger.String("op", op),
			logger.Any("error", err),
		)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	log.LogAttrs(ctx, logger.InfoLevel, "search completed",
		logger.String("op", op),
		logger.Int("results", len(result.Comments)),
		logger.Duration("duration", time.Since(startTime)),
	)

	return result, nil
}

func (s *CommentService) buildTree(ctx context.Context, tx pgxdriver.QueryExecuter, comment entity.Comment) (entity.CommentTree, error) {
	tree := entity.CommentTree{
		Comment:  comment,
		Children: make([]entity.CommentTree, 0),
	}

	children, err := s.repo.GetChildren(ctx, tx, comment.Path+"/", 1000, 0)
	if err != nil {
		return tree, fmt.Errorf("get children: %w", err)
	}

	directChildren := make([]entity.Comment, 0)
	for _, child := range children {
		if child.Depth == comment.Depth+1 {
			directChildren = append(directChildren, child)
		}
	}

	for _, child := range directChildren {
		subtree, err := s.buildTree(ctx, tx, child)
		if err != nil {
			return tree, err
		}
		tree.Children = append(tree.Children, subtree)
	}

	return tree, nil
}

func (s *CommentService) validateCreateRequest(req CreateCommentRequest) error {
	if strings.TrimSpace(req.Author) == "" {
		return fmt.Errorf("author is required: %w", entity.ErrInvalidData)
	}

	if len(req.Author) > 100 {
		return fmt.Errorf("author too long (max 100 chars): %w", entity.ErrInvalidData)
	}

	if strings.TrimSpace(req.Content) == "" {
		return fmt.Errorf("content is required: %w", entity.ErrInvalidData)
	}

	if len(req.Content) > 5000 {
		return fmt.Errorf("content too long (max 5000 chars): %w", entity.ErrInvalidData)
	}

	return nil
}

func (s *CommentService) logSlowOperation(ctx context.Context, op string, startTime time.Time, fields map[string]interface{}) {
	duration := time.Since(startTime)
	if duration > _slowOperationThresh {
		attrs := []logger.Attr{
			logger.String("op", op),
			logger.Duration("duration", duration),
		}
		for k, v := range fields {
			attrs = append(attrs, logger.Any(k, v))
		}
		s.log.Ctx(ctx).LogAttrs(ctx, logger.WarnLevel, "slow operation detected", attrs...)
	}
}
