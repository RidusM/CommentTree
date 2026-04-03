package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func (h *CommentHandler) setupRoutes() {
	h.router.GET("/health", h.Health)
	h.router.POST("/comments", h.CreateComment)
	h.router.GET("/comments", h.GetComments)
	h.router.GET("/comments/search", h.SearchComments)
	h.router.DELETE("/comments/:id", h.DeleteComment)

	h.router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{})
	})

	h.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}