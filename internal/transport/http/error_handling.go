package handler

import (
	"comtree/internal/entity"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wb-go/wbf/logger"
)

func (h *TreeHandler) handleServiceError(c *gin.Context, op string, err error) {
	ctx := c.Request.Context()
	log := h.log.Ctx(ctx).With("op", op, "error", err)

	switch {
	case errors.Is(err, entity.ErrCommentNotFound):
		log.LogAttrs(ctx, logger.WarnLevel, "comment not found sent")
		h.respondError(c, http.StatusNotFound, "comment_not_found",
			"Comment not found", err)

	case errors.Is(err, entity.ErrCommentDeleted):
		log.LogAttrs(ctx, logger.WarnLevel, "comment already deleted")
		h.respondError(c, http.StatusGone, "already_deleted",
			"Comment is already deleted", err)

	case errors.Is(err, entity.ErrParentNotFound):
		log.LogAttrs(ctx, logger.WarnLevel, "parent not found")
		h.respondError(c, http.StatusNotFound, "recipient_not_found",
			"Parent not found", err)

	case errors.Is(err, entity.ErrMaxDepthExceeded):
		log.LogAttrs(ctx, logger.WarnLevel, "max depth exceeded")
		h.respondError(c, http.StatusInternalServerError, "max_depth_exceeded",
			"Max depth exceeded", err)

	case errors.Is(err, entity.ErrInvalidData):
		log.LogAttrs(ctx, logger.WarnLevel, "invalid data")
		h.respondError(c, http.StatusBadRequest, "invalid_data", "Invalid input data", err)

	case errors.Is(err, entity.ErrConflictingData):
		log.LogAttrs(ctx, logger.WarnLevel, "conflicting data")
		h.respondError(c, http.StatusConflict, "conflict", "Data conflict occurred", err)

	default:
		log.LogAttrs(ctx, logger.ErrorLevel, "internal server error")
		h.respondError(c, http.StatusInternalServerError, "internal_error",
			"Internal server error occurred", err)
	}
}
