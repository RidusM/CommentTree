package handler

import (
	"time"

	"github.com/google/uuid"
)

// swagger:model CreateCommentRequest
type CreateCommentRequest struct {
	ParentID *uuid.UUID `json:"parent_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440001"`
	Author   string     `json:"author" binding:"required" example:"admin"`
	Content  string     `json:"content" binding:"required" example:"hello world"`
}

// swagger:model CommentResponse
type CommentResponse struct {
	ID        uuid.UUID  `json:"id" example:"550e8400-e29b-41d4-a716-446655440001"`
	ParentID  *uuid.UUID `json:"parent_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440001"`
	Author    string     `json:"author" example:"admin"`
	Content   string     `json:"content" example:"hello world"`
	IsDeleted bool       `json:"is_deleted" example:"true"`
	Depth     int        `json:"depth" example:"5"`
	CreatedAt time.Time  `json:"created_at" example:"2023-10-27T10:00:00Z"`
}

// swagger:model CommentTreeResponse
type CommentTreeResponse struct {
	Comment  CommentResponse       `json:"comment"`
	Children []CommentTreeResponse `json:"children,omitempty"`
}

// swagger:model CommentListResponse
type CommentListResponse struct {
	Comments   []CommentTreeResponse `json:"comments"`
	TotalCount int64                 `json:"total_count" example:"800"`
	Page       int                   `json:"page" example:"4"`
	PageSize   int                   `json:"page_size" example:"20"`
	TotalPages int                   `json:"total_pages" example:"40"`
}

// swagger:model SearchResponse
type SearchResponse struct {
	Comments   []CommentResponse `json:"comments"`
	TotalCount int64             `json:"total_count" example:"54"`
	Query      string            `json:"query" example:"admin"`
}

// swagger:model ErrorResponse
type ErrorResponse struct {
	Error   string `json:"error"             example:"notification not found"`
	Code    string `json:"code,omitempty"    example:"not_found"`
	Details string `json:"details,omitempty" example:"notification with id 123 does not exist"`
}

// swagger:model SuccessResponse
type SuccessResponse struct {
	Message string `json:"message" example:"Notification cancelled successfully"`
}
