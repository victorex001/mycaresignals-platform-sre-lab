// internal/server/router.go
package server

import (
	"database/sql"
	"net/http"

	"qr-feedback-api/internal/config"
	"qr-feedback-api/internal/handlers"
	"qr-feedback-api/internal/middleware"
	"qr-feedback-api/internal/notify"
	"qr-feedback-api/internal/storage"

	"github.com/gin-gonic/gin"
)

// CORS middleware – handles preflight and normal requests using cfg.AllowedOrigins
func corsMiddleware(allowedOrigins []string) gin.HandlerFunc {
	originSet := map[string]struct{}{}
	for _, o := range allowedOrigins {
		if o != "" {
			originSet[o] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		// Only allow configured origins (or no Origin – e.g. curl)
		if origin != "" {
			if _, ok := originSet[origin]; ok {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Vary", "Origin")
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
				c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Admin-Token")
				c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			}
		}

		// Preflight: return 204 and stop here
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func NewRouter(cfg config.Config, db *sql.DB) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	// log so we know this build is running
	println("DEBUG: NewRouter starting (CORS + admin + structured feedback)")

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// 1) CORS first
	r.Use(corsMiddleware(cfg.AllowedOrigins))

	// 2) Rate limiter
	r.Use(middleware.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst))

	// 3) Shared deps
	repo := storage.NewRepo(db, cfg.SecretKey)
	mailer := notify.NewMailer(cfg)

	// 4) Root + health
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "qr-feedback-api",
			"status":  "ok",
		})
	})
	r.GET("/livez", handlers.Livez())
	r.GET("/readyz", handlers.Readyz(db))

	// 5) Admin API (JSON + CSV), protected by X-Admin-Token
	adminHandler := handlers.NewAdminHandler(repo)

	admin := r.Group("/api/v1/admin", func(c *gin.Context) {
		token := c.GetHeader("X-Admin-Token")
		if token == "" || token != cfg.AdminToken {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	})

	// Simple debug endpoint to verify admin routes are live
	admin.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// VISITOR FEEDBACK (existing)
	admin.GET("/feedback", adminHandler.ListFeedback)
	admin.GET("/feedback/monthly-report", adminHandler.MonthlyFeedbackReport)
	admin.GET("/feedback/export", adminHandler.ExportFeedbackCSV)
	admin.GET("/feedback/export-positive", adminHandler.ExportPositiveCSV)
	admin.GET("/feedback/summary", adminHandler.Summary)
	admin.GET("/feedback/:feedbackID", adminHandler.GetFeedback)
	admin.POST("/feedback/:feedbackID/reviews", adminHandler.AddFeedbackReview)

	// 🆕 YOUNG PEOPLE MONTHLY VOICE – ADMIN ANALYSIS + EXPORT
	admin.GET("/young-voice", adminHandler.ListYoungVoice)
	admin.GET("/young-voice/export", adminHandler.ExportYoungVoiceCSV)
	admin.POST("/monthly-voice-contexts", adminHandler.CreateMonthlyVoiceContext)

	// 6) Public API v1
	v1 := r.Group("/api/v1")
	{
		// Explicit OPTIONS handlers (CORS already catches it, but safe)
		v1.OPTIONS("/feedback", func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		})
		v1.OPTIONS("/young-voice", func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		})

		// Structured + safeguarding-aware submit handler (visitors)
		v1.POST("/feedback", handlers.SubmitFeedback(repo, mailer))

		// 🧒 Monthly young people feedback (public, POST)
		v1.POST("/young-voice", handlers.SubmitYoungVoice(repo, mailer))
	}

	return r
}
