package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"ctree/internal/entity"

	"github.com/google/uuid"
	pgxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
	"github.com/wb-go/wbf/dbpg/pgx-driver/transaction"
	"github.com/wb-go/wbf/logger"
)

const (
	_defaultMaxDepth = 10
	_defaultPageSize = 20
	_maxPageSize     = 100
	_buildTreeLimit  = 10000

	_slowOperationThreshold = 200 * time.Millisecond
)

type (
	CommentRepository interface {
		Create(ctx context.Context, qe pgxdriver.QueryExecuter, comment entity.Comment) (*entity.Comment, error)
		GetByID(ctx context.Context, qe pgxdriver.QueryExecuter, id uuid.UUID) (*entity.Comment, error)
		GetChildren(
			ctx context.Context,
			qe pgxdriver.QueryExecuter,
			parentPath string,
			limit, offset uint64,
		) ([]entity.Comment, error)
		GetRootComments(
			ctx context.Context,
			qe pgxdriver.QueryExecuter,
			limit, offset uint64,
		) ([]entity.Comment, uint64, error)
		SoftDelete(ctx context.Context, qe pgxdriver.QueryExecuter, path string) error
		Search(
			ctx context.Context,
			qe pgxdriver.QueryExecuter,
			searchQuery string,
			limit, offset uint64,
		) ([]entity.Comment, uint64, error)
	}

	CacheRepository interface {
		GetComment(ctx context.Context, id uuid.UUID) (*entity.Comment, error)
		SaveComment(ctx context.Context, comment *entity.Comment) error
		InvalidateComment(ctx context.Context, id uuid.UUID) error
		GetCommentTree(
			ctx context.Context,
			parentID *uuid.UUID,
			page, pageSize uint64,
		) (*entity.CommentListResult, error)
		SaveCommentTree(
			ctx context.Context,
			parentID *uuid.UUID,
			page, pageSize uint64,
			result *entity.CommentListResult,
		) error
		InvalidateTree(ctx context.Context) error
	}

	CreateCommentRequest struct {
		ParentID *uuid.UUID
		Author   string
		Content  string
	}

	GetCommentsRequest struct {
		ParentID *uuid.UUID
		Page     uint64
		PageSize uint64
	}

	SearchRequest struct {
		Query    string
		Page     uint64
		PageSize uint64
	}

	CommentService struct {
		repo  CommentRepository
		cache CacheRepository
		tm    transaction.Manager
		log   logger.Logger

		maxDepth    uint64
		pageSize    uint64
		maxPageSize uint64
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
		repo:        repo,
		cache:       cache,
		tm:          tm,
		log:         log,
		maxDepth:    _defaultMaxDepth,
		pageSize:    _defaultPageSize,
		maxPageSize: _maxPageSize,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *CommentService) CreateComment(ctx context.Context, req CreateCommentRequest) (*entity.Comment, error) {
	const op = "service.CreateComment"

	log := s.log.With("op", op)
	startTime := time.Now()
	defer s.logSlowOperation(ctx, op, startTime,
		logger.Bool("has_parent", req.ParentID != nil),
		logger.String("author", req.Author),
	)

	log.LogAttrs(ctx, logger.InfoLevel, "create comment started",
		logger.String("author", req.Author),
	)

	if strings.TrimSpace(req.Content) == "" {
		return nil, fmt.Errorf("%s: %w", op, entity.ErrInvalidData)
	}
	if strings.TrimSpace(req.Author) == "" {
		return nil, fmt.Errorf("%s: %w", op, entity.ErrInvalidData)
	}

	var result *entity.Comment
	err := s.tm.ExecuteInTransaction(ctx, "create_comment", func(tx pgxdriver.QueryExecuter) error {
		depth, parentPath, err := s.validateParentAndGetPath(ctx, tx, req.ParentID)
		if err != nil {
			return err
		}

		id, err := uuid.NewV7()
		if err != nil {
			log.LogAttrs(ctx, logger.ErrorLevel, "generate id failed", logger.Any("error", err))
			return fmt.Errorf("%s: generate id: %w", op, err)
		}

		var path string
		if parentPath == "" {
			path = "/" + id.String()
		} else {
			path = parentPath + "/" + id.String()
		}

		comment := entity.Comment{
			ID:        id,
			ParentID:  req.ParentID,
			Author:    req.Author,
			Content:   req.Content,
			IsDeleted: false,
			Path:      path,
			Depth:     depth,
		}

		created, err := s.repo.Create(ctx, tx, comment)
		if err != nil {
			return transaction.HandleError(err)
		}

		result = created
		return nil
	})
	if err != nil {
		log.LogAttrs(ctx, logger.ErrorLevel, "creation failed", logger.Any("error", err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	if err = s.cache.InvalidateTree(ctx); err != nil {
		log.LogAttrs(ctx, logger.WarnLevel, "cache tree invalidation failed", logger.Any("error", err))
	}

	log.LogAttrs(ctx, logger.InfoLevel, "comment created",
		logger.String("id", result.ID.String()),
		logger.Duration("duration", time.Since(startTime)),
	)
	return result, nil
}

func (s *CommentService) GetComments(ctx context.Context, req GetCommentsRequest) (*entity.CommentListResult, error) {
	const op = "service.GetComments"

	log := s.log.With("op", op)
	startTime := time.Now()
	defer s.logSlowOperation(ctx, op, startTime,
		logger.Bool("has_parent", req.ParentID != nil),
	)

	log.LogAttrs(ctx, logger.InfoLevel, "get comments started",
		logger.Uint64("page", req.Page),
		logger.Uint64("page_size", req.PageSize),
	)

	page, pageSize := s.normalizePagination(req.Page, req.PageSize)

	if cached, err := s.cache.GetCommentTree(ctx, req.ParentID, page, pageSize); err == nil && cached != nil {
		log.LogAttrs(ctx, logger.DebugLevel, "comments served from cache",
			logger.Int("count", len(cached.Comments)),
			logger.Duration("duration", time.Since(startTime)),
		)
		return cached, nil
	}

	var result *entity.CommentListResult
	err := s.tm.ExecuteInTransaction(ctx, "get_comments", func(tx pgxdriver.QueryExecuter) error {
		var err error
		if req.ParentID == nil {
			result, err = s.getRootCommentsWithTree(ctx, tx, page, pageSize)
			if err != nil {
				return transaction.HandleError(err)
			}
		} else {
			result, err = s.getSubtreeForParent(ctx, tx, *req.ParentID, page, pageSize)
			if err != nil {
				return transaction.HandleError(err)
			}
		}
		return err
	})
	if err != nil {
		log.LogAttrs(ctx, logger.ErrorLevel, "get comments failed", logger.Any("error", err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	if cacheErr := s.cache.SaveCommentTree(ctx, req.ParentID, page, pageSize, result); cacheErr != nil {
		log.LogAttrs(ctx, logger.WarnLevel, "cache tree save failed", logger.Any("error", cacheErr))
	}

	log.LogAttrs(ctx, logger.InfoLevel, "comments retrieved",
		logger.Int("count", len(result.Comments)),
		logger.Duration("duration", time.Since(startTime)),
	)
	return result, nil
}

func (s *CommentService) DeleteComment(ctx context.Context, id uuid.UUID) error {
	const op = "service.DeleteComment"

	log := s.log.With("op", op)
	startTime := time.Now()
	defer s.logSlowOperation(ctx, op, startTime, logger.String("id", id.String()))

	log.LogAttrs(ctx, logger.InfoLevel, "delete comment started",
		logger.String("id", id.String()),
	)

	err := s.tm.ExecuteInTransaction(ctx, "delete_comment", func(tx pgxdriver.QueryExecuter) error {
		comment, err := s.repo.GetByID(ctx, tx, id)
		if err != nil {
			if errors.Is(err, entity.ErrCommentNotFound) {
				return entity.ErrCommentNotFound
			}
			return transaction.HandleError(err)
		}

		if comment.IsDeleted {
			return nil
		}

		if err = s.repo.SoftDelete(ctx, tx, comment.Path); err != nil {
			return transaction.HandleError(err)
		}

		return nil
	})
	if err != nil {
		log.LogAttrs(ctx, logger.ErrorLevel, "delete failed", logger.Any("error", err))
		return fmt.Errorf("%s: %w", op, err)
	}

	if err = s.cache.InvalidateComment(ctx, id); err != nil {
		log.LogAttrs(ctx, logger.WarnLevel, "cache comment invalidation failed",
			logger.String("id", id.String()),
			logger.Any("error", err),
		)
	}
	if err = s.cache.InvalidateTree(ctx); err != nil {
		log.LogAttrs(ctx, logger.WarnLevel, "cache tree invalidation failed", logger.Any("error", err))
	}

	log.LogAttrs(ctx, logger.InfoLevel, "comment deleted",
		logger.String("id", id.String()),
		logger.Duration("duration", time.Since(startTime)),
	)
	return nil
}

func (s *CommentService) SearchComments(ctx context.Context, req SearchRequest) (*entity.SearchResult, error) {
	const op = "service.SearchComments"

	log := s.log.With("op", op)
	startTime := time.Now()
	defer s.logSlowOperation(ctx, op, startTime, logger.String("query", req.Query))

	log.LogAttrs(ctx, logger.InfoLevel, "search comments started",
		logger.String("query", req.Query),
	)

	if strings.TrimSpace(req.Query) == "" {
		return nil, fmt.Errorf("%s: %w", op, entity.ErrInvalidData)
	}

	page, pageSize := s.normalizePagination(req.Page, req.PageSize)

	var result *entity.SearchResult
	err := s.tm.ExecuteInTransaction(ctx, "search_comments", func(tx pgxdriver.QueryExecuter) error {
		offset := (page - 1) * pageSize

		comments, total, err := s.repo.Search(ctx, tx, req.Query, pageSize, offset)
		if err != nil {
			return transaction.HandleError(err)
		}

		result = &entity.SearchResult{
			Comments:   comments,
			TotalCount: total,
			Query:      req.Query,
		}
		return nil
	})
	if err != nil {
		log.LogAttrs(ctx, logger.ErrorLevel, "search failed", logger.Any("error", err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	log.LogAttrs(ctx, logger.InfoLevel, "search completed",
		logger.Int("results", len(result.Comments)),
		logger.Duration("duration", time.Since(startTime)),
	)
	return result, nil
}

func (s *CommentService) getRootCommentsWithTree(
	ctx context.Context,
	tx pgxdriver.QueryExecuter,
	page, pageSize uint64,
) (*entity.CommentListResult, error) {
	offset := (page - 1) * pageSize

	roots, total, err := s.repo.GetRootComments(ctx, tx, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("get root comment details: %w", err)
	}

	trees, err := s.getRootCommentsTree(ctx, tx, roots)
	if err != nil {
		return nil, fmt.Errorf("get root comments tree: %w", err)
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 && total > 0 {
		totalPages = 1
	}

	return &entity.CommentListResult{
		Comments:   trees,
		TotalCount: total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *CommentService) getSubtreeForParent(
	ctx context.Context,
	tx pgxdriver.QueryExecuter,
	parentID uuid.UUID,
	page, pageSize uint64,
) (*entity.CommentListResult, error) {
	parent, err := s.repo.GetByID(ctx, tx, parentID)
	if err != nil {
		if errors.Is(err, entity.ErrCommentNotFound) {
			return nil, entity.ErrCommentNotFound
		}
		return nil, fmt.Errorf("get parent details: %w", err)
	}

	tree, err := s.buildTree(ctx, tx, *parent)
	if err != nil {
		return nil, fmt.Errorf("build tree: %w", err)
	}

	return &entity.CommentListResult{
		Comments:   []entity.CommentTree{tree},
		TotalCount: 1,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: 1,
	}, nil
}

func (s *CommentService) validateParentAndGetPath(
	ctx context.Context,
	tx pgxdriver.QueryExecuter,
	parentID *uuid.UUID,
) (uint64, string, error) {
	if parentID == nil {
		return 0, "", nil
	}

	parent, err := s.repo.GetByID(ctx, tx, *parentID)
	if err != nil {
		if errors.Is(err, entity.ErrCommentNotFound) {
			return 0, "", entity.ErrParentNotFound
		}
		return 0, "", fmt.Errorf("get parent comment: %w", err)
	}

	if parent.IsDeleted {
		return 0, "", entity.ErrParentNotFound
	}

	depth := parent.Depth + 1
	if depth > s.maxDepth {
		return 0, "", entity.ErrMaxDepthExceeded
	}

	return depth, parent.Path, nil
}

func (s *CommentService) getRootCommentsTree(
	ctx context.Context,
	tx pgxdriver.QueryExecuter,
	roots []entity.Comment,
) ([]entity.CommentTree, error) {
	trees := make([]entity.CommentTree, 0, len(roots))
	for _, root := range roots {
		tree, err := s.buildTree(ctx, tx, root)
		if err != nil {
			return nil, fmt.Errorf("build tree: %w", err)
		}
		trees = append(trees, tree)
	}
	return trees, nil
}

func (s *CommentService) buildTree(
	ctx context.Context,
	tx pgxdriver.QueryExecuter,
	root entity.Comment,
) (entity.CommentTree, error) {
	children, err := s.repo.GetChildren(ctx, tx, root.Path, _buildTreeLimit, 0)
	if err != nil {
		return entity.CommentTree{}, fmt.Errorf("get children: %w", err)
	}

	childMap := make(map[uuid.UUID][]entity.Comment)
	for _, child := range children {
		if child.IsDeleted {
			continue
		}
		if child.ParentID != nil {
			childMap[*child.ParentID] = append(childMap[*child.ParentID], child)
		}
	}

	var build func(entity.Comment) entity.CommentTree
	build = func(node entity.Comment) entity.CommentTree {
		tree := entity.CommentTree{
			Comment:  node,
			Children: make([]entity.CommentTree, 0),
		}

		for _, directChild := range childMap[node.ID] {
			tree.Children = append(tree.Children, build(directChild))
		}

		return tree
	}

	return build(root), nil
}

func (s *CommentService) normalizePagination(page, pageSize uint64) (uint64, uint64) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = s.pageSize
	}
	if pageSize > s.maxPageSize {
		pageSize = s.maxPageSize
	}
	return page, pageSize
}

func (s *CommentService) logSlowOperation(
	ctx context.Context,
	op string,
	startTime time.Time,
	attrs ...logger.Attr,
) {
	duration := time.Since(startTime)
	if duration > _slowOperationThreshold {
		allAttrs := append([]logger.Attr{
			logger.String("op", op),
			logger.Duration("duration", duration),
		}, attrs...)
		s.log.LogAttrs(ctx, logger.WarnLevel, "slow operation detected", allAttrs...)
	}
}
