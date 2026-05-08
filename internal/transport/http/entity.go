//nolint:revive, staticcheck
package handler

import (
	"time"

	"github.com/google/uuid"
)

// swagger:model CreateCommentRequest
type CreateCommentRequest struct {
	ParentID *uuid.UUID `json:"parent_id,omitempty" binding:"omitempty,uuid"          example:"550e8400-e29b-41d4-a716-446655440001"`
	Author   string     `json:"author"              binding:"required,min=1,max=100"  example:"admin"`
	Content  string     `json:"content"             binding:"required,min=1,max=5000" example:"hello world"`
}

// swagger:model CommentResponse
type CommentResponse struct {
	ID        uuid.UUID  `json:"id"                  example:"550e8400-e29b-41d4-a716-446655440001"`
	ParentID  *uuid.UUID `json:"parent_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440001"`
	Author    string     `json:"author"              example:"admin"`
	Content   string     `json:"content"             example:"hello world"`
	IsDeleted bool       `json:"is_deleted"          example:"false"`
	Depth     uint64     `json:"depth"               example:"2"`
	CreatedAt *time.Time `json:"created_at"          example:"2023-10-27T10:00:00Z"`
}

// swagger:model GetCommentsRequest
type GetCommentsRequest struct {
	ParentID *uuid.UUID `json:"parent_id,omitempty" binding:"omitempty,uuid"         example:"550e8400-e29b-41d4-a716-446655440001"`
	Page     uint64     `json:"page"                binding:"required,min=1"         example:"1"`
	PageSize uint64     `json:"page_size"           binding:"required,min=1,max=100" example:"20"`
}

// swagger:model SearchRequest
type SearchRequest struct {
	Query    string `form:"q"         json:"query"     binding:"required,min=1"          example:"admin"`
	Page     uint64 `form:"page"      json:"page"      binding:"omitempty,min=1"         example:"1"`
	PageSize uint64 `form:"page_size" json:"page_size" binding:"omitempty,min=1,max=100" example:"20"`
}

// swagger:model CommentTreeResponse
type CommentTreeResponse struct {
	Comment  CommentResponse       `json:"comment"`
	Children []CommentTreeResponse `json:"children,omitempty"`
}

// swagger:model CommentListResponse
type CommentListResponse struct {
	Comments   []CommentTreeResponse `json:"comments"`
	TotalCount uint64                `json:"total_count" example:"800"`
	Page       uint64                `json:"page"        example:"1"`
	PageSize   uint64                `json:"page_size"   example:"20"`
	TotalPages uint64                `json:"total_pages" example:"40"`
}

// swagger:model SearchResponse
type SearchResponse struct {
	Comments   []CommentResponse `json:"comments"`
	TotalCount uint64            `json:"total_count" example:"54"`
	Query      string            `json:"query"       example:"admin"`
}

// swagger:model ErrorResponse
type ErrorResponse struct {
	Error   string `json:"error"             example:"comment not found"`
	Code    string `json:"code,omitempty"    example:"NOT_FOUND"`
	Details string `json:"details,omitempty" example:"comment with id 123 does not exist"`
}

// swagger:model SuccessResponse
type SuccessResponse struct {
	Message string `json:"message" example:"Operation completed successfully"`
}

// swagger:model HealthResponse
type HealthResponse struct {
	Status string    `json:"status" example:"ok"`
	Time   time.Time `json:"time"   example:"2026-05-08T06:04:15Z"`
}
