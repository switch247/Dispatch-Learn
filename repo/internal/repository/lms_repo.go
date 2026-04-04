package repository

import (
	"time"

	"dispatchlearn/internal/domain"

	"gorm.io/gorm"
)

type LMSRepository struct {
	db *gorm.DB
}

func NewLMSRepository(db *gorm.DB) *LMSRepository {
	return &LMSRepository{db: db}
}

// Courses
func (r *LMSRepository) CreateCourse(course *domain.Course) error {
	return r.db.Create(course).Error
}

func (r *LMSRepository) FindCourseByID(tenantID, id string) (*domain.Course, error) {
	var course domain.Course
	err := r.db.Preload("ContentItems").Preload("Assessments").
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&course).Error
	if err != nil {
		return nil, err
	}
	return &course, nil
}

func (r *LMSRepository) ListCourses(tenantID string, page, perPage int) ([]domain.Course, int64, error) {
	var courses []domain.Course
	var total int64

	r.db.Model(&domain.Course{}).Where("tenant_id = ?", tenantID).Count(&total)

	err := r.db.Where("tenant_id = ?", tenantID).
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&courses).Error
	return courses, total, err
}

func (r *LMSRepository) UpdateCourse(course *domain.Course) error {
	return r.db.Save(course).Error
}

func (r *LMSRepository) FindCoursesByCategory(tenantID, category string) ([]domain.Course, error) {
	var courses []domain.Course
	err := r.db.Where("tenant_id = ? AND category = ? AND is_active = ?",
		tenantID, category, true).Find(&courses).Error
	return courses, err
}

// ContentItems
func (r *LMSRepository) CreateContentItem(item *domain.ContentItem) error {
	return r.db.Create(item).Error
}

func (r *LMSRepository) FindContentItemByID(tenantID, id string) (*domain.ContentItem, error) {
	var item domain.ContentItem
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// Assessments
func (r *LMSRepository) CreateAssessment(assessment *domain.Assessment) error {
	return r.db.Create(assessment).Error
}

func (r *LMSRepository) FindAssessmentByID(tenantID, id string) (*domain.Assessment, error) {
	var assessment domain.Assessment
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&assessment).Error
	if err != nil {
		return nil, err
	}
	return &assessment, nil
}

// Attempts
func (r *LMSRepository) CreateAttempt(attempt *domain.AssessmentAttempt) error {
	return r.db.Create(attempt).Error
}

func (r *LMSRepository) CountAttempts(tenantID, assessmentID, userID string) (int64, error) {
	var count int64
	err := r.db.Model(&domain.AssessmentAttempt{}).
		Where("tenant_id = ? AND assessment_id = ? AND user_id = ?",
			tenantID, assessmentID, userID).
		Count(&count).Error
	return count, err
}

func (r *LMSRepository) FindAttemptByID(tenantID, id string) (*domain.AssessmentAttempt, error) {
	var attempt domain.AssessmentAttempt
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&attempt).Error
	if err != nil {
		return nil, err
	}
	return &attempt, nil
}

// Grades
func (r *LMSRepository) CreateGrade(grade *domain.Grade) error {
	return r.db.Create(grade).Error
}

func (r *LMSRepository) FindHighestGrade(tenantID, assessmentID, userID string) (*domain.Grade, error) {
	var grades []domain.Grade
	err := r.db.Where("tenant_id = ? AND assessment_id = ? AND user_id = ?",
		tenantID, assessmentID, userID).
		Find(&grades).Error
	if err != nil {
		return nil, err
	}
	if len(grades) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	// Return highest (grades are encrypted so we check is_passing first)
	var best *domain.Grade
	for i := range grades {
		if best == nil || grades[i].IsPassing {
			best = &grades[i]
		}
	}
	return best, nil
}

func (r *LMSRepository) FindGradesByUser(tenantID, userID string) ([]domain.Grade, error) {
	var grades []domain.Grade
	err := r.db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).Find(&grades).Error
	return grades, err
}

// Certifications
func (r *LMSRepository) CreateCertification(cert *domain.Certification) error {
	return r.db.Create(cert).Error
}

func (r *LMSRepository) FindActiveCertification(tenantID, userID, courseID string) (*domain.Certification, error) {
	var cert domain.Certification
	err := r.db.Where("tenant_id = ? AND user_id = ? AND course_id = ? AND expires_at > ? AND is_revoked = ?",
		tenantID, userID, courseID, time.Now(), false).
		First(&cert).Error
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

func (r *LMSRepository) ListCertifications(tenantID, userID string) ([]domain.Certification, error) {
	var certs []domain.Certification
	err := r.db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Order("issued_at DESC").Find(&certs).Error
	return certs, err
}

// Reader Artifacts
func (r *LMSRepository) CreateReaderArtifact(artifact *domain.ReaderArtifact) error {
	return r.db.Create(artifact).Error
}

func (r *LMSRepository) ListReaderArtifacts(tenantID, userID, contentID string) ([]domain.ReaderArtifact, error) {
	var artifacts []domain.ReaderArtifact
	q := r.db.Where("tenant_id = ? AND user_id = ?", tenantID, userID)
	if contentID != "" {
		q = q.Where("content_id = ?", contentID)
	}
	err := q.Find(&artifacts).Error
	return artifacts, err
}

func (r *LMSRepository) UpdateReaderArtifact(artifact *domain.ReaderArtifact) error {
	return r.db.Save(artifact).Error
}

func (r *LMSRepository) CreateArtifactHistory(history *domain.ArtifactHistory) error {
	return r.db.Create(history).Error
}
