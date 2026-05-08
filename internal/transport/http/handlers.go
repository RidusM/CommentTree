//nolint:revive, staticcheck
package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"ctree/internal/entity"
	"ctree/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// @Summary Create a comment
// @Description Creates a new comment with an optional parent. Returns the created object with the calculated path and depth.
// @Tags Comments
// @Accept json
// @Produce json
// @Param request body CreateCommentRequest true "Comment data"
// @Success 201 {object} CommentResponse "Comment created"
// @Failure 400 {object} ErrorResponse "Input validation error"
// @Failure 404 {object} ErrorResponse "Parent comment not found"
// @Failure 422 {object} ErrorResponse "Maximum nesting depth exceeded"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /comments [post]
func (h *CommentHandler) CreateComment(c *gin.Context) {
	ctx := c.Request.Context()

	var req CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid_json", "Invalid request payload", err)
		return
	}

	if req.ParentID != nil && *req.ParentID == uuid.Nil {
		req.ParentID = nil
	}

	serviceReq := service.CreateCommentRequest{
		ParentID: req.ParentID,
		Author:   req.Author,
		Content:  req.Content,
	}

	comment, err := h.svc.CreateComment(ctx, serviceReq)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.Header("Location", fmt.Sprintf("/comments/%s", comment.ID))
	h.respondJSON(c, http.StatusCreated, toCommentResponse(*comment))
}

// @Summary Get the comment tree
// @Description Returns the comment tree: either the roots (if parent_id is not specified) or a subtree from the specified parent. Supports pagination
// @Tags Comments
// @Produce json
// @Param parent_id query string false "Parent comment ID (to get a subtree)" Format(uuid)
// @Param page query int false "Page number" default(1) minimum(1)
// @Param page_size query int false "Page size" default(20) minimum(1) maximum(100)
// @Success 200 {object} CommentListResponse "Comment list with tree"
// @Failure 400 {object} ErrorResponse "Invalid UUID or pagination parameters format"
// @Failure 404 {object} ErrorResponse "Comment not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /comments [get]
func (h *CommentHandler) GetComments(c *gin.Context) {
	ctx := c.Request.Context()

	var req GetCommentsRequest
	req.Page, _ = strconv.ParseUint(c.DefaultQuery("page", "1"), 10, 64)
	req.PageSize, _ = strconv.ParseUint(c.DefaultQuery("page_size", "20"), 10, 64)

	if pid := c.Query("parent_id"); pid != "" {
		id, err := uuid.Parse(pid)
		if err != nil {
			h.respondError(c, http.StatusBadRequest, "invalid_uuid", "Invalid parent_id", err)
			return
		}
		req.ParentID = &id
	}

	serviceReq := service.GetCommentsRequest{
		ParentID: req.ParentID,
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	comment, err := h.svc.GetComments(ctx, serviceReq)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	response := CommentListResponse{
		Comments:   make([]CommentTreeResponse, len(comment.Comments)),
		TotalCount: comment.TotalCount,
		Page:       comment.Page,
		PageSize:   comment.PageSize,
		TotalPages: comment.TotalPages,
	}
	for i, ct := range comment.Comments {
		response.Comments[i] = toCommentTreeResponse(ct)
	}

	h.respondJSON(c, http.StatusOK, response)
}

// @Summary Search comments
// @Description Full-text search by author and comment content. Returns a flat list of matches with pagination.
// @Tags Comments
// @Produce json
// @Param q query string true "Search query" minlength(1)
// @Param page query int false "Page number" default(1) minimum(1)
// @Param page_size query int false "Page size" default(20) minimum(1) maximum(100)
// @Success 200 {object} SearchResponse "Search results"
// @Failure 400 {object} ErrorResponse "Empty or invalid search query"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /comments/search [get]
func (h *CommentHandler) SearchComments(c *gin.Context) {
	ctx := c.Request.Context()

	var req SearchRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid_query", "Invalid query parameters", err)
		return
	}

	serviceReq := service.SearchRequest{
		Query:    req.Query,
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	comments, err := h.svc.SearchComments(ctx, serviceReq)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	response := SearchResponse{
		Comments:   make([]CommentResponse, len(comments.Comments)),
		TotalCount: comments.TotalCount,
		Query:      comments.Query,
	}
	for i, cm := range comments.Comments {
		response.Comments[i] = toCommentResponse(cm)
	}

	h.respondJSON(c, http.StatusOK, response)
}

// @Summary Delete comment
// @Description Soft delete comment and all its descendants (cascade). The comment is marked as is_deleted=true
// @Tags Comments
// @Produce json
// @Param id path string true "Comment ID" Format(uuid)
// @Success 200 {object} SuccessResponse "Comment deleted"
// @Failure 400 {object} ErrorResponse "Invalid UUID format"
// @Failure 404 {object} ErrorResponse "Comment not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /comments/{id} [delete]
func (h *CommentHandler) DeleteComment(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid_uuid", "Invalid comment ID", err)
		return
	}

	if err = h.svc.DeleteComment(ctx, id); err != nil {
		if errors.Is(err, entity.ErrCommentNotFound) {
			h.respondError(c, http.StatusNotFound, "not_found", "Comment not found", err)
			return
		}
		h.handleServiceError(c, err)
		return
	}

	h.respondJSON(c, http.StatusOK, SuccessResponse{Message: "Comment deleted"})
}

// @Summary Health check endpoint
// @Description Return service status and current timestamp. No authentication required.
// @Tags System
// @Produce json
// @Success 200 {object} HealthResponse "Service is healthy"
// @Router /health [get]
func (h *CommentHandler) Health(c *gin.Context) {
	response := HealthResponse{
		Status: "ok",
		Time:   time.Now(),
	}
	h.respondJSON(c, http.StatusOK, response)
}

func (h *CommentHandler) respondJSON(c *gin.Context, status int, data any) {
	c.JSON(status, data)
}

func (h *CommentHandler) respondError(c *gin.Context, status int, code, message string, err error) {
	response := ErrorResponse{
		Error: message,
		Code:  code,
	}
	if err != nil {
		response.Details = err.Error()
	}
	h.respondJSON(c, status, response)
}

func toCommentResponse(c entity.Comment) CommentResponse {
	return CommentResponse{
		ID:        c.ID,
		ParentID:  c.ParentID,
		Author:    c.Author,
		Content:   c.Content,
		IsDeleted: c.IsDeleted,
		Depth:     c.Depth,
		CreatedAt: c.CreatedAt,
	}
}

func toCommentTreeResponse(ct entity.CommentTree) CommentTreeResponse {
	resp := CommentTreeResponse{
		Comment:  toCommentResponse(ct.Comment),
		Children: make([]CommentTreeResponse, len(ct.Children)),
	}
	for i, child := range ct.Children {
		resp.Children[i] = toCommentTreeResponse(child)
	}
	return resp
}
