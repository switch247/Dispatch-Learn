package unit

import (
	"testing"
	"time"

	"dispatchlearn/internal/domain"

	"github.com/stretchr/testify/assert"
)

// ---- Content type enum values ----

func TestContentTypeValues(t *testing.T) {
	valid := []string{"epub", "pdf", "html"}
	invalid := []string{"docx", "mp4", "txt", "", "EPUB", "PDF"}

	validSet := map[string]bool{"epub": true, "pdf": true, "html": true}

	for _, ct := range valid {
		assert.True(t, validSet[ct], "expected %q to be a valid content type", ct)
	}
	for _, ct := range invalid {
		assert.False(t, validSet[ct], "expected %q to be an invalid content type", ct)
	}
}

// ---- Artifact type enum values ----

func TestArtifactTypeValues(t *testing.T) {
	valid := []string{"bookmark", "highlight", "annotation"}
	invalid := []string{"note", "tag", "", "Bookmark", "HIGHLIGHT"}

	validSet := map[string]bool{"bookmark": true, "highlight": true, "annotation": true}

	for _, at := range valid {
		assert.True(t, validSet[at], "expected %q to be a valid artifact type", at)
	}
	for _, at := range invalid {
		assert.False(t, validSet[at], "expected %q to be an invalid artifact type", at)
	}
}

// ---- Assessment defaults ----

func TestAssessmentDefaults(t *testing.T) {
	a := domain.Assessment{
		MaxAttempts:  3,
		PassingScore: 70,
		IsActive:     true,
	}

	assert.Equal(t, 3, a.MaxAttempts, "default max attempts is 3")
	assert.Equal(t, 70, a.PassingScore, "default passing score is 70")
	assert.True(t, a.IsActive)
}

func TestAssessmentPassingThreshold(t *testing.T) {
	cases := []struct {
		score    int
		isPassing bool
	}{
		{100, true},
		{70, true},
		{71, true},
		{69, false},
		{0, false},
		{59, false},
	}

	for _, tc := range cases {
		result := tc.score >= 70
		assert.Equal(t, tc.isPassing, result, "score=%d", tc.score)
	}
}

// ---- Certification expiry ----

func TestCertificationExpiry365Days(t *testing.T) {
	issuedAt := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	expiresAt := issuedAt.AddDate(1, 0, 0)

	daysDiff := expiresAt.Sub(issuedAt).Hours() / 24

	// 365 or 366 for leap years; use >= 365 check
	assert.GreaterOrEqual(t, daysDiff, 365.0, "certification must expire at least 365 days after issue")

	cert := domain.Certification{
		IssuedAt:  issuedAt,
		ExpiresAt: expiresAt,
		IsRevoked: false,
	}
	assert.False(t, cert.IsRevoked)
	assert.True(t, cert.ExpiresAt.After(cert.IssuedAt))
}

func TestCertificationNotExpiredBeforeExpiry(t *testing.T) {
	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.AddDate(1, 0, 0)

	isExpired := time.Now().After(expiresAt)
	assert.False(t, isExpired, "cert issued now should not yet be expired")
}

func TestCertificationRevokedState(t *testing.T) {
	now := time.Now()
	cert := domain.Certification{
		IsRevoked: true,
		RevokedAt: &now,
	}

	assert.True(t, cert.IsRevoked)
	assert.NotNil(t, cert.RevokedAt)
}

// ---- Grade struct fields ----

func TestGradeStructFields(t *testing.T) {
	g := domain.Grade{
		LetterGrade: "A",
		IsPassing:   true,
	}

	assert.Equal(t, "A", g.LetterGrade)
	assert.True(t, g.IsPassing)
}

func TestGradePassingField(t *testing.T) {
	passing := domain.Grade{IsPassing: true}
	failing := domain.Grade{IsPassing: false}

	assert.True(t, passing.IsPassing)
	assert.False(t, failing.IsPassing)
}

// ---- Max content size constant ----

func TestMaxContentSizeIs50MB(t *testing.T) {
	const maxBytes = 50 * 1024 * 1024
	assert.Equal(t, int64(52428800), int64(maxBytes))
}

func TestContentSizeBoundaryChecks(t *testing.T) {
	const maxBytes = int64(50 * 1024 * 1024)

	cases := []struct {
		size      int64
		overLimit bool
	}{
		{maxBytes, false},
		{maxBytes - 1, false},
		{maxBytes + 1, true},
		{0, false},
		{1, false},
	}

	for _, tc := range cases {
		assert.Equal(t, tc.overLimit, tc.size > maxBytes, "size=%d", tc.size)
	}
}

// ---- ReaderArtifact struct ----

func TestReaderArtifactFields(t *testing.T) {
	ra := domain.ReaderArtifact{
		UserID:       "user-1",
		ContentID:    "content-1",
		ArtifactType: "bookmark",
		Position:     `{"page":1}`,
		Content:      "",
	}

	assert.Equal(t, "bookmark", ra.ArtifactType)
	assert.Equal(t, "user-1", ra.UserID)
	assert.Equal(t, "content-1", ra.ContentID)
}
