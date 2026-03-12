package repository

import (
	"comtree/internal/entity"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/wb-go/wbf/redis"
)

const (
	_cacheTTL          = 5 * time.Minute
	_commentPrefix     = "comment:"
	_commentTreePrefix = "tree:"
)

type CacheRepository struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewCacheRepository(rdb *redis.Client, ttl time.Duration) *CacheRepository {
	if ttl == 0 {
		ttl = _cacheTTL
	}
	return &CacheRepository{
		rdb: rdb,
		ttl: ttl,
	}
}

func (r *CacheRepository) GetComment(ctx context.Context, id uuid.UUID) (*entity.Comment, error) {
	const op = "repository.cache.GetComment"

	key := _commentPrefix + id.String()

	cached, err := r.rdb.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	var comment entity.Comment
	if err := json.Unmarshal([]byte(cached), &comment); err != nil {
		return nil, fmt.Errorf("%s: unmarshal: %w", op, err)
	}

	return &comment, nil
}

func (r *CacheRepository) SetComment(ctx context.Context, comment *entity.Comment) error {
	const op = "repository.cache.SetComment"
	key := _commentPrefix + comment.ID.String()

	data, err := json.Marshal(comment)
	if err != nil {
		return fmt.Errorf("%s: marshal: %w", op, err)
	}

	if err := r.rdb.SetWithExpiration(ctx, key, data, r.ttl); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (r *CacheRepository) DeleteComment(ctx context.Context, id uuid.UUID) error {
	const op = "repository.cache.DeleteComment"
	key := _commentPrefix + id.String()
	if err := r.rdb.Del(ctx, key); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (r *CacheRepository) GetCommentTree(ctx context.Context, parentID *uuid.UUID, page, pageSize int) (*entity.CommentListResult, error) {
	const op = "repository.cache.GetCommentTree"

	key := r.getTreeCacheKey(parentID, page, pageSize)

	cached, err := r.rdb.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	var result entity.CommentListResult
	if err := json.Unmarshal([]byte(cached), &result); err != nil {
		return nil, fmt.Errorf("%s: unmarshal: %w", op, err)
	}

	return &result, nil
}

func (r *CacheRepository) SetCommentTree(ctx context.Context, parentID *uuid.UUID, page, pageSize int, result *entity.CommentListResult) error {
	const op = "repository.cache.SetCommentTree"
	key := r.getTreeCacheKey(parentID, page, pageSize)

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("%s: marshal: %w", op, err)
	}

	if err := r.rdb.SetWithExpiration(ctx, key, data, r.ttl); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (r *CacheRepository) InvalidateTree(ctx context.Context) error {
	const op = "repository.cache.InvalidateTree"

	pattern := _commentTreePrefix + "*"
	var cursor uint64

	for {
		cmd := r.rdb.Scan(ctx, cursor, pattern, 100)
		if cmd.Err() != nil {
			return fmt.Errorf("%s: redis scan: %w", op, cmd.Err())
		}

		keys, nextCursor, err := cmd.Result()
		if err != nil {
			return fmt.Errorf("%s: scan result: %w", op, err)
		}

		for _, key := range keys {
			if err := r.rdb.Del(ctx, key); err != nil {
				continue
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return nil
}

func (r *CacheRepository) getTreeCacheKey(parentID *uuid.UUID, page, pageSize int) string {
	if parentID == nil {
		return fmt.Sprintf("%sroot:p%d:ps%d", _commentTreePrefix, page, pageSize)
	}
	return fmt.Sprintf("%s%s:p%d:ps%d", _commentTreePrefix, parentID.String(), page, pageSize)
}
