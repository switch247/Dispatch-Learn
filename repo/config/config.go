package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	JWT        JWTConfig
	Encryption EncryptionConfig
	Quota      QuotaConfig
	Backup     BackupConfig
	TLS        TLSConfig
	OAuth2     OAuth2Config
}

// OAuth2Config holds settings for on-prem OIDC/OAuth2 identity provider
// integration, gated by the Enabled flag (env: USE_OAUTH2).
type OAuth2Config struct {
	Enabled      bool
	MockMode     bool   // USE_OAUTH2_MOCK - if true, use mock provider for testing; if false, require real IssuerURL
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type ServerConfig struct {
	Port string
	Mode string // debug, release, test
	Env  string // dev, staging, prod
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	DSN      string
}

type JWTConfig struct {
	Secret            string
	AccessExpiry      time.Duration
	RefreshExpiry     time.Duration
	MaxActiveSessions int
}

type EncryptionConfig struct {
	MasterKey        string
	KeyRotationDays  int
	Algorithm        string
}

type QuotaConfig struct {
	DefaultRPM          int
	DefaultBurst        int
	WebhookDailyLimit   int
}

type TLSConfig struct {
	Enabled  bool
	CertFile string
	KeyFile  string
}

type BackupConfig struct {
	Dir             string
	RetentionDays   int
	BinlogRetention time.Duration
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
			Mode: getEnv("GIN_MODE", "release"),
			Env:  getEnv("APP_ENV", "dev"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "mysql"),
			Port:     getEnv("DB_PORT", "3306"),
			User:     getEnv("DB_USER", "dispatchlearn"),
			Password: getEnv("DB_PASSWORD", "dispatchlearn_secret"),
			Name:     getEnv("DB_NAME", "dispatchlearn"),
		},
		JWT: JWTConfig{
			Secret:            getEnv("JWT_SECRET", "dispatchlearn-jwt-secret-change-in-production"),
			AccessExpiry:      30 * time.Minute,
			RefreshExpiry:     7 * 24 * time.Hour,
			MaxActiveSessions: getEnvInt("JWT_MAX_SESSIONS", 10),
		},
		Encryption: EncryptionConfig{
			MasterKey:       getEnv("ENCRYPTION_MASTER_KEY", "0123456789abcdef0123456789abcdef"),
			KeyRotationDays: 180,
			Algorithm:       "AES-256-GCM",
		},
		Quota: QuotaConfig{
			DefaultRPM:        getEnvInt("QUOTA_RPM", 600),
			DefaultBurst:      getEnvInt("QUOTA_BURST", 120),
			WebhookDailyLimit: getEnvInt("QUOTA_WEBHOOK_DAILY", 10000),
		},
		TLS: TLSConfig{
			Enabled:  getEnvBool("ENABLE_TLS", false),
			CertFile: getEnv("TLS_CERT_FILE", ""),
			KeyFile:  getEnv("TLS_KEY_FILE", ""),
		},
		Backup: BackupConfig{
			Dir:             getEnv("BACKUP_DIR", "/app/backups"),
			RetentionDays:   getEnvInt("BACKUP_RETENTION_DAYS", 30),
			BinlogRetention: 15 * time.Minute,
		},
		OAuth2: OAuth2Config{
			Enabled:      getEnvBool("USE_OAUTH2", false),
			MockMode:     getEnvBool("USE_OAUTH2_MOCK", false),
			IssuerURL:    getEnv("OAUTH2_ISSUER_URL", ""),
			ClientID:     getEnv("OAUTH2_CLIENT_ID", ""),
			ClientSecret: getEnv("OAUTH2_CLIENT_SECRET", ""),
			RedirectURL:  getEnv("OAUTH2_REDIRECT_URL", ""),
		},
	}
}

// Validate checks critical configuration constraints.
// Panics if APP_ENV=prod and TLS certificates are missing.
func (c *Config) Validate() {
	if c.Server.Env == "prod" && c.TLS.Enabled {
		if c.TLS.CertFile == "" || c.TLS.KeyFile == "" {
			panic("FATAL: APP_ENV=prod requires TLS_CERT_FILE and TLS_KEY_FILE when ENABLE_TLS=true")
		}
	}
}

func (c *Config) BuildDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.Database.User, c.Database.Password, c.Database.Host, c.Database.Port, c.Database.Name)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}
