package repository

import (
	"context"
	"errors"
	"fmt"

	"ctree/internal/entity"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	pgxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
)

const (
	_commentColumns  = "id, parent_id, author, content, is_deleted, path, depth, created_at, updated_at"
	_columnIsDeleted = "is_deleted"
)

type CommentRepository struct {
	db *pgxdriver.Postgres
}

func NewCommentRepository(db *pgxdriver.Postgres) *CommentRepository {
	return &CommentRepository{db: db}
}

func (r *CommentRepository) Create(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	c entity.Comment,
) (*entity.Comment, error) {
	const op = "repository.comment.Create"

	sql, args, err := r.db.Insert("comments").
		Columns("id", "parent_id", "author", "content", "is_deleted", "path", "depth").
		Values(c.ID, c.ParentID, c.Author, c.Content, c.IsDeleted, c.Path, c.Depth).
		Suffix("RETURNING " + _commentColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	var comment entity.Comment
	err = execOrDB(qe, r.db).QueryRow(ctx, sql, args...).Scan(
		&comment.ID,
		&comment.ParentID,
		&comment.Author,
		&comment.Content,
		&comment.IsDeleted,
		&comment.Path,
		&comment.Depth,
		&comment.CreatedAt,
		&comment.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("%s: %w", op, entity.ErrConflictingData)
		}
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &comment, nil
}

func (r *CommentRepository) GetByID(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	id uuid.UUID,
) (*entity.Comment, error) {
	const op = "repository.comment.GetByID"

	sql, args, err := r.db.Select(_commentColumns).
		From("comments").
		Where(squirrel.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	var c entity.Comment
	err = execOrDB(qe, r.db).QueryRow(ctx, sql, args...).Scan(
		&c.ID,
		&c.ParentID,
		&c.Author,
		&c.Content,
		&c.IsDeleted,
		&c.Path,
		&c.Depth,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%s: %w", op, entity.ErrDataNotFound)
		}
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &c, nil
}

func (r *CommentRepository) GetChildren(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	parentPath string,
	limit, offset uint64,
) ([]entity.Comment, error) {
	const op = "repository.comment.GetChildren"

	query := r.db.Select(_commentColumns).
		From("comments").
		Where(squirrel.Like{"path": parentPath + "/%"}).
		Where(squirrel.Eq{_columnIsDeleted: false}).
		OrderBy("path").
		Limit(limit).
		Offset(offset)

	return r.scanComments(ctx, qe, query, op)
}

func (r *CommentRepository) GetRootComments(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	limit, offset uint64,
) ([]entity.Comment, uint64, error) {
	const op = "repository.comment.GetRootComments"

	baseQuery := r.db.Select(_commentColumns).
		From("comments").
		Where(squirrel.Eq{"parent_id": nil, _columnIsDeleted: false}).
		OrderBy("created_at DESC").
		Limit(limit).
		Offset(offset)

	comments, err := r.scanComments(ctx, qe, baseQuery, op)
	if err != nil {
		return nil, 0, err
	}

	total, err := r.count(ctx, qe, "comments", squirrel.Eq{"parent_id": nil, _columnIsDeleted: false}, op)
	if err != nil {
		return nil, 0, err
	}

	return comments, total, nil
}

func (r *CommentRepository) Search(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	searchQuery string,
	limit, offset uint64,
) ([]entity.Comment, uint64, error) {
	const op = "repository.comment.Search"

	searchCondition := "to_tsvector('english', content || ' ' || author) @@ plainto_tsquery('english', ?)"

	query := r.db.Select(_commentColumns).
		From("comments").
		Where(searchCondition, searchQuery).
		Where(squirrel.Eq{_columnIsDeleted: false}).
		OrderBy("created_at DESC").
		Limit(limit).
		Offset(offset)

	comments, err := r.scanComments(ctx, qe, query, op)
	if err != nil {
		return nil, 0, err
	}

	total, err := r.count(ctx, qe, "comments", squirrel.And{
		squirrel.Expr(searchCondition, searchQuery),
		squirrel.Eq{_columnIsDeleted: false},
	}, op)
	if err != nil {
		return nil, 0, err
	}

	return comments, total, nil
}

func (r *CommentRepository) SoftDelete(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	path string,
) error {
	const op = "repository.comment.SoftDelete"

	sql, args, err := r.db.Update("comments").
		Set(_columnIsDeleted, true).
		Where(squirrel.Like{"path": path + "%"}).
		ToSql()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if _, err = execOrDB(qe, r.db).Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (r *CommentRepository) scanComments(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	builder squirrel.SelectBuilder,
	op string,
) ([]entity.Comment, error) {
	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	rows, err := execOrDB(qe, r.db).Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	var comments []entity.Comment
	for rows.Next() {
		var c entity.Comment
		if err = rows.Scan(
			&c.ID,
			&c.ParentID,
			&c.Author,
			&c.Content,
			&c.IsDeleted,
			&c.Path,
			&c.Depth,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		comments = append(comments, c)
	}
	return comments, nil
}

func (r *CommentRepository) count(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	table string,
	where any,
	op string,
) (uint64, error) {
	sql, args, err := r.db.Select("COUNT(*)").From(table).Where(where).ToSql()
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	var total uint64
	if err = execOrDB(qe, r.db).QueryRow(ctx, sql, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	return total, nil
}
