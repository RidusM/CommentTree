package handler

import (
	"net/http"

	_ "ctree/docs" // required for Swagger

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title           Comment Tree Service API
// @version         1.0
// @description     Service for work with tree comments
// @termsOfService  http://swagger.io/terms/
// @contact.name    RidusM
// @contact.email   stormkillpeople@gmail.com
// @license.name    MIT-0
// @license.url     https://github.com/aws/mit-0
// @host            localhost:8080
// @BasePath        /
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
