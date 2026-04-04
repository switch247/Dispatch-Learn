package usecase

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"dispatchlearn/internal/audit"
	"dispatchlearn/internal/crypto"
	"dispatchlearn/internal/domain"
	"dispatchlearn/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LMSUseCase struct {
	repo      *repository.LMSRepository
	audit     *audit.Service
	encryptor *crypto.Encryptor
	webhookUC *WebhookUseCase
}

func NewLMSUseCase(repo *repository.LMSRepository, audit *audit.Service, enc *crypto.Encryptor, webhookUC *WebhookUseCase) *LMSUseCase {
	return &LMSUseCase{repo: repo, audit: audit, encryptor: enc, webhookUC: webhookUC}
}

// Courses
func (uc *LMSUseCase) CreateCourse(tenantID, actorID string, req *domain.CreateCourseRequest) (*domain.Course, error) {
	course := &domain.Course{
		BaseModel: domain.BaseModel{
			ID:       uuid.New().String(),
			TenantID: tenantID,
		},
		Title:       req.Title,
		Description: req.Description,
		Category:    req.Category,
		IsActive:    true,
	}

	if err := uc.repo.CreateCourse(course); err != nil {
		return nil, err
	}

	uc.audit.Log(audit.LogEntry{
		TenantID:   tenantID,
		ActorID:    actorID,
		Action:     "course.created",
		EntityType: "course",
		EntityID:   course.ID,
		AfterState: course,
	})

	return course, nil
}

func (uc *LMSUseCase) GetCourse(tenantID, id string) (*domain.Course, error) {
	return uc.repo.FindCourseByID(tenantID, id)
}

func (uc *LMSUseCase) ListCourses(tenantID string, page, perPage int) ([]domain.Course, int64, error) {
	return uc.repo.ListCourses(tenantID, page, perPage)
}

// Content Items
func (uc *LMSUseCase) AddContentItem(tenantID, actorID string, item *domain.ContentItem) (*domain.ContentItem, error) {
	if item.SizeBytes > 50*1024*1024 {
		return nil, errors.New("content item exceeds 50MB limit")
	}

	item.BaseModel = domain.BaseModel{
		ID:       uuid.New().String(),
		TenantID: tenantID,
	}

	if err := uc.repo.CreateContentItem(item); err != nil {
		return nil, err
	}

	uc.audit.Log(audit.LogEntry{
		TenantID:   tenantID,
		ActorID:    actorID,
		Action:     "content.created",
		EntityType: "content_item",
		EntityID:   item.ID,
		AfterState: item,
	})

	return item, nil
}

// Assessments
func (uc *LMSUseCase) CreateAssessment(tenantID, actorID string, assessment *domain.Assessment) (*domain.Assessment, error) {
	assessment.BaseModel = domain.BaseModel{
		ID:       uuid.New().String(),
		TenantID: tenantID,
	}

	if err := uc.repo.CreateAssessment(assessment); err != nil {
		return nil, err
	}

	uc.audit.Log(audit.LogEntry{
		TenantID:   tenantID,
		ActorID:    actorID,
		Action:     "assessment.created",
		EntityType: "assessment",
		EntityID:   assessment.ID,
		AfterState: assessment,
	})

	return assessment, nil
}

func (uc *LMSUseCase) GetAssessment(tenantID, id string) (*domain.Assessment, error) {
	return uc.repo.FindAssessmentByID(tenantID, id)
}

// Attempts
func (uc *LMSUseCase) StartAttempt(tenantID, userID, assessmentID string) (*domain.AssessmentAttempt, error) {
	assessment, err := uc.repo.FindAssessmentByID(tenantID, assessmentID)
	if err != nil {
		return nil, errors.New("assessment not found")
	}

	count, err := uc.repo.CountAttempts(tenantID, assessmentID, userID)
	if err != nil {
		return nil, err
	}

	if count >= int64(assessment.MaxAttempts) {
		return nil, fmt.Errorf("maximum %d attempts reached", assessment.MaxAttempts)
	}

	attempt := &domain.AssessmentAttempt{
		BaseModel: domain.BaseModel{
			ID:       uuid.New().String(),
			TenantID: tenantID,
		},
		AssessmentID: assessmentID,
		UserID:       userID,
		AttemptNo:    int(count) + 1,
		StartedAt:    time.Now(),
	}

	if err := uc.repo.CreateAttempt(attempt); err != nil {
		return nil, err
	}

	uc.audit.Log(audit.LogEntry{
		TenantID:   tenantID,
		ActorID:    userID,
		Action:     "attempt.started",
		EntityType: "assessment_attempt",
		EntityID:   attempt.ID,
	})

	return attempt, nil
}

func (uc *LMSUseCase) SubmitAttempt(tenantID, userID, attemptID string, answers string, score int) (*domain.Grade, error) {
	attempt, err := uc.repo.FindAttemptByID(tenantID, attemptID)
	if err != nil {
		return nil, errors.New("attempt not found")
	}

	if attempt.UserID != userID {
		return nil, errors.New("unauthorized")
	}

	if attempt.CompletedAt != nil {
		return nil, errors.New("attempt already completed")
	}

	// Update attempt
	now := time.Now()
	attempt.CompletedAt = &now
	attempt.Answers = answers

	// Encrypt score
	encScore, err := uc.encryptor.Encrypt(strconv.Itoa(score))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt grade: %w", err)
	}

	letterGrade := numericToLetter(score)
	isPassing := score >= 70

	grade := &domain.Grade{
		BaseModel: domain.BaseModel{
			ID:       uuid.New().String(),
			TenantID: tenantID,
		},
		UserID:       userID,
		AssessmentID: attempt.AssessmentID,
		AttemptID:    attemptID,
		NumericScore: encScore,
		LetterGrade:  letterGrade,
		IsPassing:    isPassing,
		GradedAt:     now,
	}

	if err := uc.repo.CreateGrade(grade); err != nil {
		return nil, err
	}

	uc.audit.Log(audit.LogEntry{
		TenantID:   tenantID,
		ActorID:    userID,
		Action:     "grade.recorded",
		EntityType: "grade",
		EntityID:   grade.ID,
		AfterState: map[string]interface{}{"letter_grade": letterGrade, "is_passing": isPassing},
	})

	uc.webhookUC.DispatchEvent(tenantID, "scoring.completed", map[string]interface{}{"user_id": userID, "grade_id": grade.ID, "is_passing": isPassing})

	return grade, nil
}

// Certifications
func (uc *LMSUseCase) IssueCertification(tenantID, actorID, userID, courseID string) (*domain.Certification, error) {
	// Check if active cert already exists
	existing, err := uc.repo.FindActiveCertification(tenantID, userID, courseID)
	if err == nil && existing != nil {
		return existing, nil
	}

	cert := &domain.Certification{
		BaseModel: domain.BaseModel{
			ID:       uuid.New().String(),
			TenantID: tenantID,
		},
		UserID:    userID,
		CourseID:  courseID,
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().AddDate(0, 0, 365),
	}

	if err := uc.repo.CreateCertification(cert); err != nil {
		return nil, err
	}

	uc.audit.Log(audit.LogEntry{
		TenantID:   tenantID,
		ActorID:    actorID,
		Action:     "certification.issued",
		EntityType: "certification",
		EntityID:   cert.ID,
		AfterState: cert,
	})

	uc.webhookUC.DispatchEvent(tenantID, "learning.completed", map[string]interface{}{"user_id": userID, "course_id": courseID, "certification_id": cert.ID})

	return cert, nil
}

func (uc *LMSUseCase) GetActiveCertification(tenantID, userID, courseID string) (*domain.Certification, error) {
	return uc.repo.FindActiveCertification(tenantID, userID, courseID)
}

func (uc *LMSUseCase) ListCertifications(tenantID, userID string) ([]domain.Certification, error) {
	return uc.repo.ListCertifications(tenantID, userID)
}

// Qualified Dispatch Check
func (uc *LMSUseCase) IsAgentQualified(tenantID, userID, orderCategory string) (bool, string) {
	courses, err := uc.repo.FindCoursesByCategory(tenantID, orderCategory)
	if err != nil || len(courses) == 0 {
		return true, "" // No course requirements for this category
	}

	for _, course := range courses {
		// Check certification
		cert, err := uc.repo.FindActiveCertification(tenantID, userID, course.ID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return false, fmt.Sprintf("missing certification for course: %s", course.Title)
			}
			return false, "error checking certification"
		}
		if cert.ExpiresAt.Before(time.Now()) {
			return false, fmt.Sprintf("certification expired for course: %s", course.Title)
		}

		// Check grades - highest must be >= 70
		grade, err := uc.repo.FindHighestGrade(tenantID, course.ID, userID)
		if err != nil {
			return false, fmt.Sprintf("no grade found for course: %s", course.Title)
		}
		if !grade.IsPassing {
			return false, fmt.Sprintf("grade below threshold for course: %s", course.Title)
		}
	}

	return true, ""
}

// Reader Artifacts
func (uc *LMSUseCase) CreateReaderArtifact(tenantID, userID string, artifact *domain.ReaderArtifact) (*domain.ReaderArtifact, error) {
	artifact.BaseModel = domain.BaseModel{
		ID:       uuid.New().String(),
		TenantID: tenantID,
	}
	artifact.UserID = userID

	if err := uc.repo.CreateReaderArtifact(artifact); err != nil {
		return nil, err
	}

	// Create immutable history
	uc.repo.CreateArtifactHistory(&domain.ArtifactHistory{
		BaseModel: domain.BaseModel{
			ID:       uuid.New().String(),
			TenantID: tenantID,
		},
		ArtifactID: artifact.ID,
		Action:     "created",
		NewValue:   artifact.Content,
		ChangedAt:  time.Now(),
	})

	return artifact, nil
}

func (uc *LMSUseCase) ListReaderArtifacts(tenantID, userID, contentID string) ([]domain.ReaderArtifact, error) {
	return uc.repo.ListReaderArtifacts(tenantID, userID, contentID)
}

func numericToLetter(score int) string {
	switch {
	case score >= 93:
		return "A"
	case score >= 90:
		return "A-"
	case score >= 87:
		return "B+"
	case score >= 83:
		return "B"
	case score >= 80:
		return "B-"
	case score >= 77:
		return "C+"
	case score >= 73:
		return "C"
	case score >= 70:
		return "C-"
	case score >= 67:
		return "D+"
	case score >= 63:
		return "D"
	case score >= 60:
		return "D-"
	default:
		return "F"
	}
}
