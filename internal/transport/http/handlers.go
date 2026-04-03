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

// Create comment
// @Summary Создать комментарий
// @Description Создаёт новый комментарий с указанием родительского (опционально). Возвращает созданный объект с вычисленным путём и глубиной
// @Tags Comments
// @Accept json
// @Produce json
// @Param request body CreateCommentRequest true "Данные комментария"
// @Success 201 {object} CommentResponse "Комментарий создан"
// @Failure 400 {object} ErrorResponse "Ошибка валидации входных данных"
// @Failure 404 {object} ErrorResponse "Родительский комментарий не найден"
// @Failure 422 {object} ErrorResponse "Превышена максимальная глубина вложенности"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /comments [post]
func (h *CommentHandler) CreateComment(c *gin.Context) {
	var req service.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid_json", "Invalid request payload", err)
		return
	}

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

// Get comments tree
// @Summary Получить дерево комментариев
// @Description Возвращает дерево комментариев: либо корни (если parent_id не указан), либо поддерево от указанного родителя. Поддерживает пагинацию
// @Tags Comments
// @Produce json
// @Param parent_id query string false "ID родительского комментария (для получения поддерева)" Format(uuid)
// @Param page query int false "Номер страницы" default(1) minimum(1)
// @Param page_size query int false "Размер страницы" default(20) minimum(1) maximum(100)
// @Success 200 {object} CommentListResponse "Список комментариев с деревом"
// @Failure 400 {object} ErrorResponse "Неверный формат UUID или параметров пагинации"
// @Failure 404 {object} ErrorResponse "Комментарий не найден"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /comments [get]
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

// Search comments
// @Summary Поиск по комментариям
// @Description Полнотекстовый поиск по автору и содержимому комментариев. Возвращает плоский список совпадений с пагинацией
// @Tags Comments
// @Produce json
// @Param q query string true "Поисковый запрос" minlength(1)
// @Param page query int false "Номер страницы" default(1) minimum(1)
// @Param page_size query int false "Размер страницы" default(20) minimum(1) maximum(100)
// @Success 200 {object} SearchResponse "Результаты поиска"
// @Failure 400 {object} ErrorResponse "Пустой или невалидный поисковый запрос"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /comments/search [get]
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

// Delete comment
// @Summary Удалить комментарий
// @Description Мягкое удаление комментария и всех его вложенных потомков (каскадно). Комментарий помечается как is_deleted=true
// @Tags Comments
// @Produce json
// @Param id path string true "ID комментария" Format(uuid)
// @Success 200 {object} SuccessResponse "Комментарий удалён"
// @Failure 400 {object} ErrorResponse "Неверный формат UUID"
// @Failure 404 {object} ErrorResponse "Комментарий не найден"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /comments/{id} [delete]
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

func (h *CommentHandler) Health(c *gin.Context) {
	response := map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
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