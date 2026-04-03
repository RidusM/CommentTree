package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title           Comment Tree Service API
// @version         1.0
// @description     API для работы с древовидными комментариями: создание, чтение, удаление, поиск с поддержкой неограниченной вложенности
// @termsOfService  http://swagger.io/terms/
// @contact.name    Roman Ivanov
// @contact.email   stormkillpeople@gmail.com
// @license.name    MIT-0
// @license.url     https://github.com/aws/mit-0
// @host            localhost:8080
// @BasePath        /
// @schemes         http https
// @securityDefinitions.apikey ApiKeyAuth
// @in              header
// @name            Authorization
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