package repository

import (
	"comtree/internal/entity"
	"context"
	"errors"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	pgxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
)

type CommentRepository struct {
	db *pgxdriver.Postgres
}

func NewCommentRepository(db *pgxdriver.Postgres) *CommentRepository {
	return &CommentRepository{db: db}
}

func (r *CommentRepository) Create(ctx context.Context, qe pgxdriver.QueryExecuter, comment entity.Comment) (*entity.Comment, error) {
	const op = "repository.comment.Create"

executor := execOrDB(qe, r.db)

	insert := r.db.Insert("comments").
		Columns("id", "parent_id", "author", "content", "is_deleted", "depth").
		Values(comment.ID, comment.ParentID, comment.Author, comment.Content, comment.IsDeleted, comment.Depth).
		Suffix("RETURNING id, parent_id, author, content, is_deleted, path, depth")

	query, args, err := insert.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: build insert query: %w", op, err)
	}

	var result entity.Comment
	err = executor.QueryRow(ctx, query, args...).Scan(
		&result.ID,
		&result.ParentID,
		&result.Author,
		&result.Content,
		&result.IsDeleted,
		&result.Path,
		&result.Depth,
	)

	if err != nil {
		return nil, fmt.Errorf("%s: create comment: %w", op, err)
	}

	return &result, nil
}

func (r *CommentRepository) GetByID(ctx context.Context, qe pgxdriver.QueryExecuter, id uuid.UUID) (*entity.Comment, error) {
	const op = "repository.comment.GetByID"

	executor := execOrDB(qe, r.db)

	selectQuery := r.db.Select("id", "parent_id", "author", "content", "is_deleted", "path", "depth").
		From("comments").
		Where(squirrel.Eq{"id": id})

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: select query: %w", op, err)
	}

	var comment entity.Comment
	err = executor.QueryRow(ctx, query, args...).Scan(
		&comment.ID,
		&comment.ParentID,
		&comment.Author,
		&comment.Content,
		&comment.IsDeleted,
		&comment.Path,
		&comment.Depth,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, entity.ErrCommentNotFound
		}
		return nil, fmt.Errorf("%s: get comment by id: %w", op, err)
	}

	return &comment, nil
}

func (r *CommentRepository) GetChildren(ctx context.Context, qe pgxdriver.QueryExecuter, parentPath string, limit, offset int) ([]entity.Comment, error) {
	executor := execOrDB(qe, r.db)

	selectQuery := r.db.Select("id", "parent_id", "author", "content", "is_deleted", "path", "depth").
		From("comments").
		Where(squirrel.Like{"path": parentPath + "%"}).
		Where(squirrel.Eq{"is_deleted": false}).
		OrderBy("path ASC").
		Limit(uint64(limit)).
		Offset(uint64(offset))

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select query: %w", err)
	}

	rows, err := executor.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query children: %w", err)
	}
	defer rows.Close()

	var comments []entity.Comment
	for rows.Next() {
		var c entity.Comment
		if err := rows.Scan(&c.ID, &c.ParentID, &c.Author, &c.Content, &c.IsDeleted, &c.Path, &c.Depth); err != nil {
			return nil, fmt.Errorf("scan child: %w", err)
		}
		comments = append(comments, c)
	}
	return comments, nil
}

func (r *CommentRepository) SoftDelete(ctx context.Context, qe pgxdriver.QueryExecuter, path string) error {
	executor := execOrDB(qe, r.db)

	update := r.db.Update("comments").
		Set("is_deleted", true).
		Where(squirrel.Like{"path": path + "%"})

	query, args, err := update.ToSql()
	if err != nil {
		return fmt.Errorf("build update query: %w", err)
	}

	if _, err := executor.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	return nil
}
func (r *CommentRepository) GetRootComments(ctx context.Context, qe pgxdriver.QueryExecuter, limit, offset int) ([]entity.Comment, int64, error) {
	const op = "repository.comment.GetRootComments"

	executor := execOrDB(qe, r.db)

	selectQuery := r.db.Select("id", "parent_id", "author", "content", "is_deleted", "path", "depth").
		From("comments").
		Where(squirrel.Eq{"parent_id": nil, "is_deleted": false}).
		OrderBy("id DESC").
		Limit(uint64(limit)).
		Offset(uint64(offset))

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("%s: select query: %w", op, err)
	}

	rows, err := executor.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	comments := make([]entity.Comment, 0)
	for rows.Next() {
		var comment entity.Comment
		if err := rows.Scan(
			&comment.ID,
			&comment.ParentID,
			&comment.Author,
			&comment.Content,
			&comment.IsDeleted,
			&comment.Path,
			&comment.Depth,
		); err != nil {
			return nil, 0, fmt.Errorf("%s: scan: %w", op, err)
		}
		comments = append(comments, comment)
	}

	countQuery := r.db.Select("COUNT(*)").
		From("comments").
		Where(squirrel.Eq{"parent_id": nil, "is_deleted": false})

	query, args, err = countQuery.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("%s: count query: %w", op, err)
	}

	var total int64
	if err := executor.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("%s: count: %w", op, err)
	}

	return comments, total, nil
}

func (r *CommentRepository) Search(ctx context.Context, qe pgxdriver.QueryExecuter, searchQuery string, limit, offset int) ([]entity.Comment, int64, error) {
	const op = "repository.comment.Search"

	executor := execOrDB(qe, r.db)

	selectQuery := r.db.Select("id", "parent_id", "author", "content", "is_deleted", "path", "depth").
		From("comments").
		Where("to_tsvector('english', content || ' ' || author) @@ plainto_tsquery('english', ?)", searchQuery).
		Where(squirrel.Eq{"is_deleted": false}).
		OrderBy("id DESC").
		Limit(uint64(limit)).
		Offset(uint64(offset))

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("%s: select query: %w", op, err)
	}

	rows, err := executor.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	comments := make([]entity.Comment, 0)
	for rows.Next() {
		var comment entity.Comment
		if err := rows.Scan(
			&comment.ID,
			&comment.ParentID,
			&comment.Author,
			&comment.Content,
			&comment.IsDeleted,
			&comment.Path,
			&comment.Depth,
		); err != nil {
			return nil, 0, fmt.Errorf("%s: scan: %w", op, err)
		}
		comments = append(comments, comment)
	}

	countQuery := r.db.Select("COUNT(*)").
		From("comments").
		Where("to_tsvector('english', content || ' ' || author) @@ plainto_tsquery('english', ?)", searchQuery).
		Where(squirrel.Eq{"is_deleted": false})

	query, args, err = countQuery.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("%s: count query: %w", op, err)
	}

	var total int64
	if err := executor.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("%s: count: %w", op, err)
	}

	return comments, total, nil
}
