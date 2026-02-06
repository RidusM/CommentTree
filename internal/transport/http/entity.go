package httpt

import (
	"time"

	"github.com/google/uuid"
)

type CreateCommentRequest struct {
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
	Author   string     `json:"author" binding:"required"`
	Conent   string     `json:"content" binding:"required"`
}

type CommentResponse struct {
	ID        uuid.UUID  `json:"id"`
	ParentID  *uuid.UUID `json:"parent_id,omitempty"`
	Author    string     `json:"author"`
	Content   string     `json:"content"`
	IsDeleted bool       `json:"is_deleted"`
	Depth     int        `json:"depth"`
	CreatedAt time.Time  `json:"created_at"`
}

type CommentTreeResponse struct {
	Comment  CommentResponse       `json:"comment"`
	Children []CommentTreeResponse `json:"children,omitempty"`
}

type CommentListResponse struct {
	Comments   []CommentTreeResponse `json:"comments"`
	TotalCount int64                 `json:"total_count"`
	Page       int                   `json:"page"`
	PageSize   int                   `json:"page_size"`
	TotalPages int                   `json:"total_pages"`
}

type SearchResponse struct {
	Comments   []CommentResponse `json:"comments"`
	TotalCount int64             `json:"total_count"`
	Query      string            `json:"query"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"messaage"`
	Deatils string `json:"details,omitempty"`
}

type SuccessRepsonse struct {
	Message string `json:"message"`
}
