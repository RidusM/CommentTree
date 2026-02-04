package entity

import (
	"time"

	"github.com/google/uuid"
)

type Comment struct {
	ID        uuid.UUID  `json:"id"`
	ParentID  *uuid.UUID `json:"parent_id"`
	Author    string     `json:"author"`
	Content   string     `json:"content"`
	IsDeleted bool       `json:"is_deleted"`
	Path      string     `json:"path"`
	Depth     int        `json:"depth"`
}

func (c *Comment) CreatedAt() time.Time {
	return ExtractTimestampFromUUIDv7(c.ID)
}

type CommentTree struct {
	Comment  Comment       `json:"comment"`
	Children []CommentTree `json:"children,omitempty"`
}

type CommentListResult struct {
	Comments   []CommentTree `json:"comments"`
	TotalCount int64         `json:"total_count"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	TotalPages int           `json:"total_pages"`
}

type SearchResult struct {
	Comments   []Comment `json:"comments"`
	TotalCount int64     `json:"total_count"`
	Query      string    `json:"query"`
}

func ExtractTimestampFromUUIDv7(id uuid.UUID) time.Time {
	timestamp := uint64(id[0])<<40 | uint64(id[1])<<32 | uint64(id[2])<<24 |
		uint64(id[3])<<16 | uint64(id[4])<<8 | uint64(id[5])

	seconds := timestamp / 1000
	nanos := (timestamp % 1000) * 1_000_000

	return time.Unix(int64(seconds), int64(nanos)).UTC()
}
