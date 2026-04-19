package handler

import (
	"net/http"

	"dispatchlearn/internal/domain"
	"dispatchlearn/internal/middleware"
	"dispatchlearn/internal/usecase"

	"github.com/gin-gonic/gin"
)

type LMSHandler struct {
	uc *usecase.LMSUseCase
}

func NewLMSHandler(uc *usecase.LMSUseCase) *LMSHandler {
	return &LMSHandler{uc: uc}
}

// Courses
func (h *LMSHandler) CreateCourse(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	actorID := middleware.GetUserID(c)

	var req domain.CreateCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	course, err := h.uc.CreateCourse(tenantID, actorID, &req)
	if err != nil {
		respondError(c, http.StatusBadRequest, "CREATE_FAILED", err.Error())
		return
	}

	respondCreated(c, course)
}

func (h *LMSHandler) GetCourse(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	id := c.Param("id")

	course, err := h.uc.GetCourse(tenantID, id)
	if err != nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "course not found")
		return
	}

	respondOK(c, course)
}

func (h *LMSHandler) ListCourses(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	page, perPage := getPagination(c)

	courses, total, err := h.uc.ListCourses(tenantID, page, perPage)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	respondList(c, courses, page, perPage, total)
}

// Content Items
func (h *LMSHandler) AddContentItem(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	actorID := middleware.GetUserID(c)
	courseID := c.Param("id")

	var item domain.ContentItem
	if err := c.ShouldBindJSON(&item); err != nil {
		respondValidation(c, err.Error())
		return
	}
	item.CourseID = courseID

	result, err := h.uc.AddContentItem(tenantID, actorID, &item)
	if err != nil {
		respondError(c, http.StatusBadRequest, "CREATE_FAILED", err.Error())
		return
	}

	respondCreated(c, result)
}

// Assessments
func (h *LMSHandler) CreateAssessment(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	actorID := middleware.GetUserID(c)
	courseID := c.Param("id")

	var assessment domain.Assessment
	if err := c.ShouldBindJSON(&assessment); err != nil {
		respondValidation(c, err.Error())
		return
	}
	assessment.CourseID = courseID

	result, err := h.uc.CreateAssessment(tenantID, actorID, &assessment)
	if err != nil {
		respondError(c, http.StatusBadRequest, "CREATE_FAILED", err.Error())
		return
	}

	respondCreated(c, result)
}

func (h *LMSHandler) GetAssessment(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	id := c.Param("assessment_id")

	assessment, err := h.uc.GetAssessment(tenantID, id)
	if err != nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "assessment not found")
		return
	}

	respondOK(c, assessment)
}

// Attempts
func (h *LMSHandler) StartAttempt(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	userID := middleware.GetUserID(c)
	assessmentID := c.Param("assessment_id")

	attempt, err := h.uc.StartAttempt(tenantID, userID, assessmentID)
	if err != nil {
		respondError(c, http.StatusBadRequest, "ATTEMPT_FAILED", err.Error())
		return
	}

	respondCreated(c, attempt)
}

func (h *LMSHandler) SubmitAttempt(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	userID := middleware.GetUserID(c)
	attemptID := c.Param("attempt_id")

	var req struct {
		Answers string `json:"answers" binding:"required"`
		Score   int    `json:"score" binding:"required,min=0,max=100"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	grade, err := h.uc.SubmitAttempt(tenantID, userID, attemptID, req.Answers, req.Score)
	if err != nil {
		respondError(c, http.StatusBadRequest, "SUBMIT_FAILED", err.Error())
		return
	}

	respondCreated(c, maskGrade(grade, canViewSensitiveGrades(c)))
}

// Certifications
func (h *LMSHandler) IssueCertification(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	actorID := middleware.GetUserID(c)

	var req struct {
		UserID   string `json:"user_id" binding:"required"`
		CourseID string `json:"course_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	cert, err := h.uc.IssueCertification(tenantID, actorID, req.UserID, req.CourseID)
	if err != nil {
		respondError(c, http.StatusBadRequest, "ISSUE_FAILED", err.Error())
		return
	}

	respondCreated(c, cert)
}

func (h *LMSHandler) ListCertifications(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	currentUserID := middleware.GetUserID(c)
	targetUserID := c.DefaultQuery("user_id", currentUserID)

	// Cert query scoping: non-admin users can only view their own certs
	if targetUserID != currentUserID {
		roles := middleware.GetRoles(c)
		allowed := false
		for _, r := range roles {
			if r == "admin" || r == "system_admin" {
				allowed = true
				break
			}
		}
		if !allowed {
			respondError(c, http.StatusForbidden, "FORBIDDEN", "cannot view other user's certifications")
			return
		}
	}

	certs, err := h.uc.ListCertifications(tenantID, targetUserID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	respondOK(c, certs)
}

// Reader Artifacts
func (h *LMSHandler) CreateReaderArtifact(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	userID := middleware.GetUserID(c)

	var artifact domain.ReaderArtifact
	if err := c.ShouldBindJSON(&artifact); err != nil {
		respondValidation(c, err.Error())
		return
	}
	if artifact.ContentID == "" {
		respondValidation(c, "content_id is required")
		return
	}

	result, err := h.uc.CreateReaderArtifact(tenantID, userID, &artifact)
	if err != nil {
		respondError(c, http.StatusBadRequest, "CREATE_FAILED", err.Error())
		return
	}

	respondCreated(c, result)
}

func (h *LMSHandler) ListReaderArtifacts(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	userID := middleware.GetUserID(c)
	contentID := c.Query("content_id")

	artifacts, err := h.uc.ListReaderArtifacts(tenantID, userID, contentID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	respondOK(c, artifacts)
}
