package entity

import (
	"time"

	"github.com/google/uuid"
)

const (
	millisPerSecond  = 1000
	nanosPerMilli    = 1_000_000
	maxUnixTimestamp = 1<<63 - 1
)

type Comment struct {
	ID        uuid.UUID  `json:"id"                   validate:"required,uuid_strict"`
	ParentID  *uuid.UUID `json:"parent_id"                   validate:"required,uuid_strict"`
	Author    string     `json:"author" validate:"required"`
	Content   string     `json:"content" validate:"required"`
	IsDeleted bool       `json:"is_deleted" validate:"required"`
	Path      string     `json:"path" validate:"required"`
	Depth     int        `json:"depth" validate:"required,min=0,max=10"`
}

func (c *Comment) CreatedAt() time.Time {
	return ExtractTimestampFromUUIDv7(c.ID)
}

type CommentTree struct {
	Comment  Comment       `json:"comment" validate:"required"`
	Children []CommentTree `json:"children,omitempty" validate:"required"`
}

type CommentListResult struct {
	Comments   []CommentTree `json:"comments" validate:"required"`
	TotalCount int64         `json:"total_count" validate:"required"`
	Page       int           `json:"page" validate:"required,min=0,max=100"`
	PageSize   int           `json:"page_size" validate:"required,min = 0, max= 100"`
	TotalPages int           `json:"total_pages" validate:"required"`
}

type SearchResult struct {
	Comments   []Comment `json:"comments" validate:"required"`
	TotalCount int64     `json:"total_count" validate:"required"`
	Query      string    `json:"query" validate:"required"`
}

func ExtractTimestampFromUUIDv7(id uuid.UUID) time.Time {
	timestamp := uint64(id[0])<<40 | uint64(id[1])<<32 | uint64(id[2])<<24 |
		uint64(id[3])<<16 | uint64(id[4])<<8 | uint64(id[5])

	seconds := timestamp / millisPerSecond
	nanos := (timestamp % millisPerSecond) * nanosPerMilli

	if seconds > maxUnixTimestamp {
		return time.Time{}
	}

	// nolint:gosec
	return time.Unix(int64(seconds), int64(nanos)).UTC()
}
