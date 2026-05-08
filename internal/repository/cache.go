//nolint:musttag
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"ctree/internal/entity"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	rediswbf "github.com/wb-go/wbf/redis"
)

const (
	_defaultTTL        = 5 * time.Minute
	_commentPrefix     = "comment:"
	_commentTreePrefix = "tree:"

	_cacheScanBatch = 100
)

type CacheRepository struct {
	rdb *rediswbf.Client
}

func NewCacheRepository(rdb *rediswbf.Client) *CacheRepository {
	return &CacheRepository{rdb: rdb}
}

func (r *CacheRepository) cacheKeyComment(id uuid.UUID) string {
	return _commentPrefix + id.String()
}

func (r *CacheRepository) cacheKeyTree(parentID *uuid.UUID, page, pageSize uint64) string {
	pid := "root"
	if parentID != nil {
		pid = parentID.String()
	}
	return fmt.Sprintf("tree:%s:p%d_s%d", pid, page, pageSize)
}

func (r *CacheRepository) GetComment(ctx context.Context, id uuid.UUID) (*entity.Comment, error) {
	const op = "repository.cache.GetComment"

	cached, err := r.rdb.Get(ctx, r.cacheKeyComment(id))
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, entity.ErrDataNotFound
		}
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	if cached == "" {
		return nil, entity.ErrDataNotFound
	}

	var comment entity.Comment
	if err = json.Unmarshal([]byte(cached), &comment); err != nil {
		return nil, fmt.Errorf("%s: unmarshal: %w", op, err)
	}
	return &comment, nil
}

func (r *CacheRepository) SaveComment(ctx context.Context, comment *entity.Comment) error {
	const op = "repository.cache.SaveComment"

	data, err := json.Marshal(comment)
	if err != nil {
		return fmt.Errorf("%s: marshal: %w", op, err)
	}

	if err = r.rdb.SetWithExpiration(ctx, r.cacheKeyComment(comment.ID), data, _defaultTTL); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (r *CacheRepository) InvalidateComment(ctx context.Context, id uuid.UUID) error {
	const op = "repository.cache.InvalidateComment"

	if err := r.rdb.Del(ctx, r.cacheKeyComment(id)); err != nil {
		if errors.Is(err, redis.Nil) {
			return nil
		}
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (r *CacheRepository) GetCommentTree(
	ctx context.Context,
	parentID *uuid.UUID,
	page, pageSize uint64,
) (*entity.CommentListResult, error) {
	const op = "repository.cache.GetCommentTree"

	cached, err := r.rdb.Get(ctx, r.cacheKeyTree(parentID, page, pageSize))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	var result entity.CommentListResult
	if err = json.Unmarshal([]byte(cached), &result); err != nil {
		return nil, fmt.Errorf("%s: unmarshal: %w", op, err)
	}
	return &result, nil
}

func (r *CacheRepository) SaveCommentTree(
	ctx context.Context,
	parentID *uuid.UUID,
	page, pageSize uint64,
	result *entity.CommentListResult,
) error {
	const op = "repository.cache.SaveCommentTree"

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("%s: marshal: %w", op, err)
	}

	if err = r.rdb.SetWithExpiration(ctx, r.cacheKeyTree(parentID, page, pageSize), data, _defaultTTL); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (r *CacheRepository) InvalidateTree(ctx context.Context) error {
	const op = "repository.cache.InvalidateTree"

	pattern := _commentTreePrefix + "*"
	var cursor uint64

	for {
		cmd := r.rdb.Scan(ctx, cursor, pattern, _cacheScanBatch)
		if cmd.Err() != nil {
			return fmt.Errorf("%s: %w", op, cmd.Err())
		}

		keys, nextCursor, err := cmd.Result()
		if err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		for _, key := range keys {
			if err = r.rdb.Del(ctx, key); err != nil {
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
