package handler

import (
	"net/http"
	"strconv"

	"dispatchlearn/internal/domain"

	"github.com/gin-gonic/gin"
)

func getPagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	return page, perPage
}

func respondOK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, domain.APIResponse{Data: data})
}

func respondCreated(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, domain.APIResponse{Data: data})
}

func respondList(c *gin.Context, data interface{}, page, perPage int, total int64) {
	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}
	c.JSON(http.StatusOK, domain.APIResponse{
		Data: data,
		Meta: &domain.Meta{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		},
	})
}

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, domain.APIResponse{
		Errors: []domain.APIError{{Code: code, Message: message}},
	})
}

func respondValidation(c *gin.Context, message string) {
	respondError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", message)
}

func respondConflict(c *gin.Context, message string) {
	respondError(c, http.StatusConflict, "CONFLICT", message)
}
