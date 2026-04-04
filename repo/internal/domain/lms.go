package domain

import "time"

type Course struct {
	BaseModel
	Title       string        `gorm:"type:varchar(255);not null" json:"title"`
	Description string        `gorm:"type:text" json:"description"`
	Category    string        `gorm:"type:varchar(100);index" json:"category"`
	IsActive    bool          `gorm:"default:true" json:"is_active"`
	ContentItems []ContentItem `gorm:"foreignKey:CourseID" json:"content_items,omitempty"`
	Assessments  []Assessment  `gorm:"foreignKey:CourseID" json:"assessments,omitempty"`
}

func (Course) TableName() string { return "courses" }

type ContentItem struct {
	BaseModel
	CourseID    string `gorm:"type:char(36);not null;index" json:"course_id"`
	Title       string `gorm:"type:varchar(255);not null" json:"title"`
	ContentType string `gorm:"type:enum('epub','pdf','html');not null" json:"content_type"`
	FilePath    string `gorm:"type:varchar(500)" json:"file_path"`
	Checksum    string `gorm:"type:varchar(64)" json:"checksum"`
	SizeBytes   int64  `gorm:"type:bigint" json:"size_bytes"`
	SortOrder   int    `gorm:"default:0" json:"sort_order"`
}

func (ContentItem) TableName() string { return "content_items" }

type Assessment struct {
	BaseModel
	CourseID     string `gorm:"type:char(36);not null;index" json:"course_id"`
	Title        string `gorm:"type:varchar(255);not null" json:"title"`
	Description  string `gorm:"type:text" json:"description"`
	MaxAttempts  int    `gorm:"default:3" json:"max_attempts"`
	PassingScore int    `gorm:"default:70" json:"passing_score"`
	IsActive     bool   `gorm:"default:true" json:"is_active"`
}

func (Assessment) TableName() string { return "assessments" }

type AssessmentAttempt struct {
	BaseModel
	AssessmentID string     `gorm:"type:char(36);not null;index" json:"assessment_id"`
	UserID       string     `gorm:"type:char(36);not null;index" json:"user_id"`
	AttemptNo    int        `gorm:"not null" json:"attempt_no"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	Answers      string     `gorm:"type:text" json:"answers,omitempty"`
}

func (AssessmentAttempt) TableName() string { return "assessment_attempts" }

type Grade struct {
	BaseModel
	UserID           string `gorm:"type:char(36);not null;uniqueIndex:idx_grade_unique,priority:2" json:"user_id"`
	AssessmentID     string `gorm:"type:char(36);not null;index" json:"assessment_id"`
	AttemptID        string `gorm:"type:char(36);not null;uniqueIndex:idx_grade_unique,priority:3" json:"attempt_id"`
	NumericScore     string `gorm:"type:varchar(255)" json:"numeric_score"` // encrypted
	LetterGrade      string `gorm:"type:varchar(5)" json:"letter_grade"`
	IsPassing        bool   `json:"is_passing"`
	GradedAt         time.Time `json:"graded_at"`
}

func (Grade) TableName() string { return "grades" }

type Certification struct {
	BaseModel
	UserID     string    `gorm:"type:char(36);not null;index" json:"user_id"`
	CourseID   string    `gorm:"type:char(36);not null;index" json:"course_id"`
	IssuedAt   time.Time `json:"issued_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	IsRevoked  bool      `gorm:"default:false" json:"is_revoked"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

func (Certification) TableName() string { return "certifications" }

type ReaderArtifact struct {
	BaseModel
	UserID       string `gorm:"type:char(36);not null;index" json:"user_id"`
	ContentID    string `gorm:"type:char(36);not null;index" json:"content_id"`
	ArtifactType string `gorm:"type:enum('bookmark','highlight','annotation');not null" json:"artifact_type"`
	Position     string `gorm:"type:varchar(500)" json:"position"`
	Content      string `gorm:"type:text" json:"content"`
}

func (ReaderArtifact) TableName() string { return "reader_artifacts" }

type ArtifactHistory struct {
	BaseModel
	ArtifactID string    `gorm:"type:char(36);not null;index" json:"artifact_id"`
	Action     string    `gorm:"type:enum('created','updated','deleted');not null" json:"action"`
	OldValue   string    `gorm:"type:text" json:"old_value,omitempty"`
	NewValue   string    `gorm:"type:text" json:"new_value,omitempty"`
	ChangedAt  time.Time `json:"changed_at"`
}

func (ArtifactHistory) TableName() string { return "artifact_history" }

// LMS request types
type CreateCourseRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Category    string `json:"category" binding:"required"`
}

type SubmitAssessmentRequest struct {
	Answers string `json:"answers" binding:"required"`
}

type GradeResponse struct {
	ID           string `json:"id"`
	NumericScore int    `json:"numeric_score"`
	LetterGrade  string `json:"letter_grade"`
	IsPassing    bool   `json:"is_passing"`
}
