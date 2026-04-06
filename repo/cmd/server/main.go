package main

import (
	"fmt"
	"os"
	"time"

	"dispatchlearn/config"
	"dispatchlearn/internal/audit"
	"dispatchlearn/internal/crypto"
	"dispatchlearn/internal/domain"
	"dispatchlearn/internal/handler"
	"dispatchlearn/internal/middleware"
	"dispatchlearn/internal/repository"
	"dispatchlearn/internal/usecase"
	"dispatchlearn/internal/worker"
	"dispatchlearn/logging"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	// Load and validate config
	cfg := config.Load()
	cfg.Validate() // Panics if APP_ENV=prod and TLS certs are missing
	logging.Init(logging.INFO, os.Stdout)
	logging.Info("server", "init", "Starting DispatchLearn server")

	// Connect to database with retry
	var db *gorm.DB
	var err error
	dsn := cfg.BuildDSN()

	for i := 0; i < 30; i++ {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err == nil {
			sqlDB, _ := db.DB()
			if sqlDB.Ping() == nil {
				break
			}
		}
		logging.Info("server", "db", fmt.Sprintf("Waiting for database... attempt %d/30", i+1))
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		logging.Error("server", "db", "Failed to connect to database: "+err.Error())
		os.Exit(1)
	}

	logging.Info("server", "db", "Database connected")

	// Auto-migrate
	if err := autoMigrate(db); err != nil {
		logging.Error("server", "migrate", "Migration failed: "+err.Error())
		os.Exit(1)
	}
	logging.Info("server", "migrate", "Database migration completed")

	// Seed data
	seedData(db, cfg)

	// Initialize services
	encryptor, err := crypto.NewEncryptor(cfg.Encryption.MasterKey)
	if err != nil {
		logging.Error("server", "crypto", "Failed to initialize encryptor: "+err.Error())
		os.Exit(1)
	}

	auditSvc := audit.NewService(db)

	// Repositories
	authRepo := repository.NewAuthRepository(db)
	lmsRepo := repository.NewLMSRepository(db)
	dispatchRepo := repository.NewDispatchRepository(db)
	financeRepo := repository.NewFinanceRepository(db)
	systemRepo := repository.NewSystemRepository(db)

	// Quota provider with DB-backed overrides (1-minute cache)
	quotaProvider := middleware.NewDBQuotaProvider(db)

	// Use cases
	authUC := usecase.NewAuthUseCase(authRepo, auditSvc, cfg)
	webhookQuota := middleware.NewWebhookQuotaTracker(cfg.Quota.WebhookDailyLimit, quotaProvider)
	webhookUC := usecase.NewWebhookUseCase(systemRepo, auditSvc, webhookQuota)
	lmsUC := usecase.NewLMSUseCase(lmsRepo, auditSvc, encryptor, webhookUC)
	dispatchUC := usecase.NewDispatchUseCase(dispatchRepo, lmsUC, auditSvc, encryptor, webhookUC)
	financeUC := usecase.NewFinanceUseCase(financeRepo, auditSvc, encryptor)
	reportUC := usecase.NewReportUseCase(systemRepo, financeRepo, dispatchRepo, auditSvc, "/app/exports")

	// Handlers
	authH := handler.NewAuthHandler(authUC, cfg)
	lmsH := handler.NewLMSHandler(lmsUC)
	dispatchH := handler.NewDispatchHandler(dispatchUC)
	financeH := handler.NewFinanceHandler(financeUC)
	systemH := handler.NewSystemHandler(reportUC, webhookUC)

	// Start background expiry worker (runs every 5 minutes)
	expiryWorker := worker.NewExpiryWorker(dispatchUC, 5*time.Minute)
	expiryWorker.Start()
	defer expiryWorker.Stop()

	// Setup Gin
	gin.SetMode(cfg.Server.Mode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger())

	// Rate limiter
	rateLimiter := middleware.NewRateLimiter(cfg.Quota.DefaultRPM, cfg.Quota.DefaultBurst, quotaProvider)

	// Public routes
	r.GET("/health", handler.HealthCheck)
	r.GET("/docs", handler.DocsHandler)
	r.Static("/static", "/app/static") // Serve offline Swagger UI assets

	api := r.Group("/api/v1")
	api.GET("/openapi.json", handler.OpenAPISpecHandler)
	{
		// Auth (public)
		auth := api.Group("/auth")
		{
			auth.POST("/register", authH.Register)
			auth.POST("/login", authH.Login)
			auth.POST("/refresh", authH.RefreshToken)
			auth.GET("/oauth2/login", authH.OAuth2Login)
			auth.POST("/oauth2/callback", authH.OAuth2Callback)
		}

		// Protected routes
		protected := api.Group("")
		protected.Use(middleware.AuthMiddleware(cfg))
		protected.Use(middleware.TenantIsolation())
		protected.Use(rateLimiter.Middleware())
		{
			// Users
			protected.GET("/me", authH.GetMe)
			protected.GET("/users", middleware.RequireRole("admin", "system_admin"), authH.ListUsers)
			protected.GET("/users/:id", middleware.RequireRole("admin", "system_admin"), authH.GetUser)
			protected.POST("/users/:id/roles", middleware.RequireRole("admin", "system_admin"), authH.AssignRole)
			protected.GET("/roles", authH.ListRoles)

			// Sessions
			protected.GET("/sessions", authH.ListSessions)
			protected.DELETE("/sessions/:session_id", authH.RevokeSession)
			protected.POST("/auth/logout/:session_id", authH.Logout)

			// LMS - Courses
			protected.POST("/courses", middleware.RequireRole("admin", "instructor"), lmsH.CreateCourse)
			protected.GET("/courses", lmsH.ListCourses)
			protected.GET("/courses/:id", lmsH.GetCourse)
			protected.POST("/courses/:id/content", middleware.RequireRole("admin", "instructor"), lmsH.AddContentItem)
			protected.POST("/courses/:id/assessments", middleware.RequireRole("admin", "instructor"), lmsH.CreateAssessment)

			// LMS - Assessments
			protected.GET("/assessments/:assessment_id", lmsH.GetAssessment)
			protected.POST("/assessments/:assessment_id/attempts", lmsH.StartAttempt)
			protected.POST("/attempts/:attempt_id/submit", lmsH.SubmitAttempt)

			// LMS - Certifications
			protected.POST("/certifications", middleware.RequireRole("admin", "instructor"), lmsH.IssueCertification)
			protected.GET("/certifications", lmsH.ListCertifications)

			// LMS - Reader Artifacts
			protected.POST("/reader-artifacts", lmsH.CreateReaderArtifact)
			protected.GET("/reader-artifacts", lmsH.ListReaderArtifacts)

			// Dispatch - Orders
			protected.POST("/orders", middleware.RequireRole("admin", "dispatcher"), dispatchH.CreateOrder)
			protected.GET("/orders", dispatchH.ListOrders)
			protected.GET("/orders/:id", dispatchH.GetOrder)
			protected.PATCH("/orders/:id/status", middleware.RequireRole("admin", "dispatcher"), dispatchH.TransitionOrder)
			protected.POST("/orders/:id/accept", dispatchH.AcceptOrder)
			protected.GET("/orders/:id/recommendations", middleware.RequireRole("admin", "dispatcher"), dispatchH.RecommendAgents)
			protected.POST("/dispatch/expire-stale", middleware.RequireRole("admin", "system_admin"), dispatchH.ExpireStaleOrders)

			// Dispatch - Service Zones
			protected.POST("/service-zones", middleware.RequireRole("admin"), dispatchH.CreateServiceZone)
			protected.GET("/service-zones", dispatchH.ListServiceZones)

			// Dispatch - Agent Profiles
			protected.POST("/agent-profiles", middleware.RequireRole("admin"), dispatchH.CreateAgentProfile)
			protected.GET("/agent-profiles/:user_id", dispatchH.GetAgentProfile)

			// Finance - Invoices (all routes require admin or finance role)
			protected.POST("/invoices", middleware.RequireRole("admin", "finance"), financeH.CreateInvoice)
			protected.GET("/invoices", middleware.RequireRole("admin", "finance"), financeH.ListInvoices)
			protected.GET("/invoices/:id", middleware.RequireRole("admin", "finance"), financeH.GetInvoice)
			protected.POST("/invoices/:id/issue", middleware.RequireRole("admin", "finance"), financeH.IssueInvoice)
			protected.GET("/invoices/:id/payments", middleware.RequireRole("admin", "finance"), financeH.ListPaymentsByInvoice)

			// Finance - Payments (all routes require admin or finance role)
			protected.POST("/payments", middleware.RequireRole("admin", "finance"), financeH.RecordPayment)
			protected.GET("/payments/:id", middleware.RequireRole("admin", "finance"), financeH.GetPayment)
			protected.GET("/orders/:id/payments", middleware.RequireRole("admin", "finance"), financeH.ListPaymentsByOrder)

			// Finance - Refunds
			protected.POST("/refunds", middleware.RequireRole("admin", "finance"), financeH.ProcessRefund)

			// Finance - Ledger
			protected.GET("/ledger", middleware.RequireRole("admin", "finance"), financeH.ListLedgerEntries)
			protected.GET("/orders/:id/ledger", middleware.RequireRole("admin", "finance"), financeH.ListLedgerEntriesByOrder)

			// System - Audit
			protected.GET("/audit-logs", middleware.RequireRole("admin", "system_admin"), systemH.ListAuditLogs)
			protected.POST("/audit-logs/verify", middleware.RequireRole("admin", "system_admin"), systemH.VerifyAuditChain)

			// System - Config
			protected.GET("/config-changes", middleware.RequireRole("admin", "system_admin"), systemH.ListConfigChanges)

			// System - Reports
			protected.POST("/reports", middleware.RequireRole("admin", "system_admin"), systemH.GenerateReport)
			protected.GET("/reports", middleware.RequireRole("admin", "system_admin"), systemH.ListReports)
			protected.GET("/reports/:id", middleware.RequireRole("admin", "system_admin"), systemH.GetReport)

			// System - Webhooks
			protected.POST("/webhooks", middleware.RequireRole("admin", "system_admin"), systemH.CreateWebhookSubscription)
			protected.GET("/webhooks", middleware.RequireRole("admin", "system_admin"), systemH.ListWebhookSubscriptions)
			protected.GET("/webhooks/:id", middleware.RequireRole("admin", "system_admin"), systemH.GetWebhookSubscription)
			protected.GET("/webhooks/dead-letters", middleware.RequireRole("admin", "system_admin"), systemH.ListDeadLetters)

			// System - Quotas
			protected.GET("/quotas", middleware.RequireRole("admin", "system_admin"), systemH.GetQuotaOverride)
			protected.PUT("/quotas", middleware.RequireRole("admin", "system_admin"), systemH.SetQuotaOverride)
		}
	}

	addr := ":" + cfg.Server.Port
	logging.Info("server", "start", "Listening on "+addr)

	if cfg.TLS.Enabled {
		// When TLS is enabled, certificates are mandatory — no HTTP fallback
		if cfg.TLS.CertFile == "" || cfg.TLS.KeyFile == "" {
			logging.Error("server", "tls", "FATAL: TLS is enabled but TLS_CERT_FILE and TLS_KEY_FILE are missing. Cannot start. Set ENABLE_TLS=false to disable TLS.")
			os.Exit(1)
		}
		logging.Info("server", "tls", "Starting with TLS on "+addr)
		if err := r.RunTLS(addr, cfg.TLS.CertFile, cfg.TLS.KeyFile); err != nil {
			logging.Error("server", "start", "TLS server failed: "+err.Error())
			os.Exit(1)
		}
	} else {
		// TLS disabled — plain HTTP (for development/testing only)
		if err := r.Run(addr); err != nil {
			logging.Error("server", "start", "Server failed: "+err.Error())
			os.Exit(1)
		}
	}
}

func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&domain.Tenant{},
		&domain.User{},
		&domain.Role{},
		&domain.Permission{},
		&domain.UserRole{},
		&domain.RolePermission{},
		&domain.UserSession{},
		&domain.Course{},
		&domain.ContentItem{},
		&domain.Assessment{},
		&domain.AssessmentAttempt{},
		&domain.Grade{},
		&domain.Certification{},
		&domain.ReaderArtifact{},
		&domain.ArtifactHistory{},
		&domain.Order{},
		&domain.DispatchAcceptance{},
		&domain.ServiceZone{},
		&domain.DistanceMatrix{},
		&domain.Zip4Centroid{},
		&domain.AgentProfile{},
		&domain.AgentMetrics{},
		&domain.Invoice{},
		&domain.Payment{},
		&domain.LedgerEntry{},
		&domain.LedgerLink{},
		&domain.WebhookSubscription{},
		&domain.WebhookDelivery{},
		&domain.AuditLog{},
		&domain.ConfigChange{},
		&domain.Report{},
		&domain.QuotaOverride{},
	)
}

func seedData(db *gorm.DB, cfg *config.Config) {
	// Check if seed data already exists
	var count int64
	db.Model(&domain.Tenant{}).Count(&count)
	if count > 0 {
		logging.Info("server", "seed", "Seed data already exists, skipping")
		return
	}

	logging.Info("server", "seed", "Seeding initial data")

	// Create default tenant
	tenant := domain.Tenant{
		ID:       "00000000-0000-0000-0000-000000000001",
		Name:     "Default Tenant",
		IsActive: true,
	}
	db.Create(&tenant)

	// Create permissions
	permissions := []domain.Permission{
		{BaseModel: domain.BaseModel{ID: "perm-users-read", TenantID: tenant.ID}, Name: "users:read", Resource: "users", Action: "read"},
		{BaseModel: domain.BaseModel{ID: "perm-users-write", TenantID: tenant.ID}, Name: "users:write", Resource: "users", Action: "write"},
		{BaseModel: domain.BaseModel{ID: "perm-courses-read", TenantID: tenant.ID}, Name: "courses:read", Resource: "courses", Action: "read"},
		{BaseModel: domain.BaseModel{ID: "perm-courses-write", TenantID: tenant.ID}, Name: "courses:write", Resource: "courses", Action: "write"},
		{BaseModel: domain.BaseModel{ID: "perm-orders-read", TenantID: tenant.ID}, Name: "orders:read", Resource: "orders", Action: "read"},
		{BaseModel: domain.BaseModel{ID: "perm-orders-write", TenantID: tenant.ID}, Name: "orders:write", Resource: "orders", Action: "write"},
		{BaseModel: domain.BaseModel{ID: "perm-finance-read", TenantID: tenant.ID}, Name: "finance:read", Resource: "finance", Action: "read"},
		{BaseModel: domain.BaseModel{ID: "perm-finance-write", TenantID: tenant.ID}, Name: "finance:write", Resource: "finance", Action: "write"},
		{BaseModel: domain.BaseModel{ID: "perm-audit-read", TenantID: tenant.ID}, Name: "audit:read", Resource: "audit", Action: "read"},
		{BaseModel: domain.BaseModel{ID: "perm-system-admin", TenantID: tenant.ID}, Name: "system:admin", Resource: "system", Action: "admin"},
	}
	for _, p := range permissions {
		db.Create(&p)
	}

	// Create roles
	adminRole := domain.Role{
		BaseModel:   domain.BaseModel{ID: "role-admin", TenantID: tenant.ID},
		Name:        "admin",
		Description: "Administrator with full access",
	}
	db.Create(&adminRole)

	dispatcherRole := domain.Role{
		BaseModel:   domain.BaseModel{ID: "role-dispatcher", TenantID: tenant.ID},
		Name:        "dispatcher",
		Description: "Dispatcher - manages orders and assignments",
	}
	db.Create(&dispatcherRole)

	agentRole := domain.Role{
		BaseModel:   domain.BaseModel{ID: "role-agent", TenantID: tenant.ID},
		Name:        "agent",
		Description: "Field agent",
	}
	db.Create(&agentRole)

	instructorRole := domain.Role{
		BaseModel:   domain.BaseModel{ID: "role-instructor", TenantID: tenant.ID},
		Name:        "instructor",
		Description: "Training instructor",
	}
	db.Create(&instructorRole)

	financeRole := domain.Role{
		BaseModel:   domain.BaseModel{ID: "role-finance", TenantID: tenant.ID},
		Name:        "finance",
		Description: "Finance operator",
	}
	db.Create(&financeRole)

	// Assign all permissions to admin
	for _, p := range permissions {
		db.Create(&domain.RolePermission{RoleID: adminRole.ID, PermissionID: p.ID, TenantID: tenant.ID})
	}

	// Create admin user (password: admin123)
	adminUser := domain.User{
		BaseModel:    domain.BaseModel{ID: "user-admin", TenantID: tenant.ID},
		Username:     "admin",
		PasswordHash: "$2a$10$lfeFCgx2NfWbra0Szg8lme0IeICzbhD4pXVJARzMX7WsTwzHIoYw6", // admin123
		FullName:     "System Administrator",
		IsActive:     true,
	}
	db.Create(&adminUser)
	db.Create(&domain.UserRole{UserID: adminUser.ID, RoleID: adminRole.ID, TenantID: tenant.ID})

	// Create sample agent user (password: admin123)
	agentUser := domain.User{
		BaseModel:    domain.BaseModel{ID: "user-agent1", TenantID: tenant.ID},
		Username:     "agent1",
		PasswordHash: "$2a$10$lfeFCgx2NfWbra0Szg8lme0IeICzbhD4pXVJARzMX7WsTwzHIoYw6", // admin123
		FullName:     "John Agent",
		IsActive:     true,
	}
	db.Create(&agentUser)
	db.Create(&domain.UserRole{UserID: agentUser.ID, RoleID: agentRole.ID, TenantID: tenant.ID})

	// Create service zone
	zone := domain.ServiceZone{
		BaseModel:   domain.BaseModel{ID: "zone-1", TenantID: tenant.ID},
		Name:        "Metro Area",
		ZipCodes:    "10001,10002,10003,10004,10005",
		CentroidLat: 40.7484,
		CentroidLng: -73.9967,
	}
	db.Create(&zone)

	// Create agent profile
	agentProfile := domain.AgentProfile{
		BaseModel:       domain.BaseModel{ID: "ap-1", TenantID: tenant.ID},
		UserID:          agentUser.ID,
		ZoneID:          zone.ID,
		ZipCode:         "10001",
		IsAvailable:     true,
		MaxWorkload:     8,
		ReputationScore: 75.00,
	}
	db.Create(&agentProfile)

	logging.Info("server", "seed", "Seed data created successfully")
}
