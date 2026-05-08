package entity

import (
	"time"

	"github.com/google/uuid"
)

type Comment struct {
	ID        uuid.UUID
	ParentID  *uuid.UUID
	Author    string
	Content   string
	IsDeleted bool
	Path      string
	Depth     uint64
	CreatedAt *time.Time
	UpdatedAt *time.Time
}

type CommentTree struct {
	Comment  Comment
	Children []CommentTree
}

type CommentListResult struct {
	Comments   []CommentTree
	TotalCount uint64
	Page       uint64
	PageSize   uint64
	TotalPages uint64
}

type SearchResult struct {
	Comments   []Comment
	TotalCount uint64
	Query      string
}
