package unit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"dispatchlearn/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRateLimiter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("allows requests within limit", func(t *testing.T) {
		rl := middleware.NewRateLimiter(600, 10, nil)

		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set("tenant_id", "test-tenant")
			c.Next()
		})
		r.Use(rl.Middleware())
		r.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})

		for i := 0; i < 10; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}
	})

	t.Run("blocks requests exceeding burst", func(t *testing.T) {
		rl := middleware.NewRateLimiter(60, 3, nil) // 3 burst

		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set("tenant_id", "test-tenant-2")
			c.Next()
		})
		r.Use(rl.Middleware())
		r.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})

		// Use up burst
		for i := 0; i < 3; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}

		// Next should be rate limited
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
	})

	t.Run("no tenant passes through", func(t *testing.T) {
		rl := middleware.NewRateLimiter(1, 1, nil)

		r := gin.New()
		r.Use(rl.Middleware())
		r.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
