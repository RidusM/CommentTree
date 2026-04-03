package handler

import (
	"comtree/internal/entity"
	"comtree/internal/service"
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wb-go/wbf/logger"
)

type CommentService interface {
	CreateComment(ctx context.Context, req service.CreateCommentRequest) (*entity.Comment, error)
	GetComments(ctx context.Context, req service.GetCommentsRequest) (*entity.CommentListResult, error)
	DeleteComment(ctx context.Context, id uuid.UUID) error
	SearchComments(ctx context.Context, req service.SearchRequest) (*entity.SearchResult, error)
}

type CommentHandler struct {
	svc    CommentService
	log    logger.Logger
	router *gin.Engine
}

func NewCommentHandler(svc CommentService, log logger.Logger) *CommentHandler {
	h := &CommentHandler{
		svc:    svc,
		log:    log,
		router: gin.New(),
	}

	h.router.Use(h.requestIDMiddleware())
	h.router.Use(h.loggingMiddleware())
	h.router.Use(gin.Recovery())
	h.router.Use(gin.ErrorLogger())

	h.router.LoadHTMLGlob("web/*.html")
	h.router.Static("/static", "./web")

	h.setupRoutes()
	return h
}


func (h *CommentHandler) Engine() *gin.Engine { return h.router }