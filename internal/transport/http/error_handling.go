package handler

import (
	"errors"
	"net/http"

	"ctree/internal/entity"

	"github.com/gin-gonic/gin"
)

func (h *CommentHandler) handleServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, entity.ErrInvalidData):
		h.respondError(c, http.StatusBadRequest, "invalid_data",
			"Invalid input data", err)
	case errors.Is(err, entity.ErrConflictingData):
		h.respondError(c, http.StatusConflict, "conflict",
			"Data conflict occurred", err)
	case errors.Is(err, entity.ErrCommentNotFound):
		h.respondError(c, http.StatusNotFound, "comment_not_found",
			"Comment not found", err)
	case errors.Is(err, entity.ErrParentNotFound):
		h.respondError(c, http.StatusNotFound, "recipient_not_found",
			"Parent not found", err)
	case errors.Is(err, entity.ErrMaxDepthExceeded):
		h.respondError(c, http.StatusInternalServerError, "max_depth_exceeded",
			"Max depth exceeded", err)
	default:
		h.respondError(c, http.StatusInternalServerError, "internal_error",
			"Internal server error occurred", err)
	}
}
