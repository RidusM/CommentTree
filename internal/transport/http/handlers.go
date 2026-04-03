package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"comtree/internal/entity"
	"comtree/internal/service"
)

func (h *CommentHandler) CreateComment(c *gin.Context) {
	var req service.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid_json", "Invalid request payload", err)
		return
	}

	// Нормализация nil UUID
	if req.ParentID != nil && *req.ParentID == uuid.Nil {
		req.ParentID = nil
	}

	comment, err := h.svc.CreateComment(c.Request.Context(), req)
	if err != nil {
		h.handleServiceError(c, "handler.CreateComment", err)
		return
	}

	c.Header("Location", "/comments/"+comment.ID.String())
	h.respondJSON(c, http.StatusCreated, toCommentResponse(*comment))
}

func (h *CommentHandler) GetComments(c *gin.Context) {
	var req service.GetCommentsRequest
	req.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	req.PageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if pid := c.Query("parent_id"); pid != "" {
		id, err := uuid.Parse(pid)
		if err != nil {
			h.respondError(c, http.StatusBadRequest, "invalid_uuid", "Invalid parent_id", err)
			return
		}
		req.ParentID = &id
	}

	result, err := h.svc.GetComments(c.Request.Context(), req)
	if err != nil {
		h.handleServiceError(c, "handler.GetComments", err)
		return
	}

	resp := CommentListResponse{
		Comments:   make([]CommentTreeResponse, len(result.Comments)),
		TotalCount: result.TotalCount,
		Page:       result.Page,
		PageSize:   result.PageSize,
		TotalPages: result.TotalPages,
	}
	for i, ct := range result.Comments {
		resp.Comments[i] = toCommentTreeResponse(ct)
	}

	h.respondJSON(c, http.StatusOK, resp)
}

func (h *CommentHandler) DeleteComment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid_uuid", "Invalid comment ID", err)
		return
	}

	if err := h.svc.DeleteComment(c.Request.Context(), id); err != nil {
		if errors.Is(err, service.ErrCommentNotFound) {
			h.respondError(c, http.StatusNotFound, "not_found", "Comment not found", err)
			return
		}
		h.handleServiceError(c, "handler.DeleteComment", err)
		return
	}

	h.respondJSON(c, http.StatusOK, SuccessResponse{Message: "Comment deleted"})
}

func (h *CommentHandler) SearchComments(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		h.respondError(c, http.StatusBadRequest, "empty_query", "Search query is required", nil)
		return
	}

	req := service.SearchRequest{
		Query:    q,
		Page:     1,
		PageSize: 20,
	}
	req.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	req.PageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := h.svc.SearchComments(c.Request.Context(), req)
	if err != nil {
		h.handleServiceError(c, "handler.SearchComments", err)
		return
	}

	resp := SearchResponse{
		Comments:   make([]CommentResponse, len(result.Comments)),
		TotalCount: result.TotalCount,
		Query:      result.Query,
	}
	for i, cm := range result.Comments {
		resp.Comments[i] = toCommentResponse(cm)
	}

	h.respondJSON(c, http.StatusOK, resp)
}

func (h *CommentHandler) Health(c *gin.Context) {
	h.respondJSON(c, http.StatusOK, map[string]string{"status": "ok", "time": time.Now().Format(time.RFC3339)})
}

// --- Мапперы ---
func toCommentResponse(c entity.Comment) CommentResponse {
	return CommentResponse{
		ID:        c.ID,
		ParentID:  c.ParentID,
		Author:    c.Author,
		Content:   c.Content,
		IsDeleted: c.IsDeleted,
		Depth:     c.Depth,
		CreatedAt: c.CreatedAt(),
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