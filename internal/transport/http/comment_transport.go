package handler

import (
	"comtree/internal/entity"
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wb-go/wbf/logger"
)

type NotifyService interface {
	CreateComment(ctx context.Context, req CreateCommentRequest) (*entity.Comment, error)
	GetComments(ctx context.Context, req CreateCommentRequest) (*entity.CommentListResult, error)
	DeleteComment(ctx context.Context, id uuid.UUID) error
	SearchComments(ctx context.Context, req ) (*entity.SearchResult, error)
}

type NotifyHandler struct {
	svc    *service.NotifyService
	log    logger.Logger
	router *gin.Engine
}

func NewNotifyHandler(
	svc *service.NotifyService,
	log logger.Logger,
) *NotifyHandler {
	h := &NotifyHandler{
		svc: svc,
		log: log,
	}

	router := gin.New()

	router.Use(h.requestIDMiddleware())
	router.Use(h.loggingMiddleware())
	router.Use(gin.Recovery())

	h.router = router

	h.router.LoadHTMLGlob("web/*.html")
	h.router.Static("/static", "./web")

	h.setupRoutes()

	return h
}

func (h *NotifyHandler) Engine() *gin.Engine {
	return h.router
}
