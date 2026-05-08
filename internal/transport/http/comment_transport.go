package handler

import (
	"context"
	"net/http"

	"ctree/internal/entity"
	"ctree/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wb-go/wbf/logger"
)

const _maxRequestBodySize = 1 << 20

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
		svc: svc,
		log: log,
	}

	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, _maxRequestBodySize)
	})

	router.Use(h.requestIDMiddleware())
	router.Use(h.loggingMiddleware())
	router.Use(h.baseCORSMiddleware())
	router.Use(gin.Recovery())

	h.router = router

	h.router.LoadHTMLGlob("web/*.html")
	h.router.Static("/static", "./web")

	h.setupRoutes()

	return h
}

func (h *CommentHandler) Engine() *gin.Engine {
	return h.router
}
