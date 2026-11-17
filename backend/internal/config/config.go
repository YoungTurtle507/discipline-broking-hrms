package config

import (
	"os"
	"strconv"
)

type Config struct {
	BackendPort string
	DBHost      string
	DBPort      string
	DBUser      string
	DBPassword  string
	DBName      string
	// DBSSLMode is passed straight through to the Postgres DSN. Local dev
	// Postgres has no TLS listener at all, hence "disable" by default;
	// hosted Postgres (e.g. Neon) refuses non-TLS connections and needs
	// this set to "require" via env.
	DBSSLMode string
	RedisHost string
	RedisPort string
	// RedisDB selects Redis's logical DB index (0-15 by default) — lets two
	// separate projects share one native Redis instance/port with fully
	// separate keyspaces instead of needing a second port.
	RedisDB        int
	FaceServiceURL string
	// NtfyTopic is the ntfy.sh topic OFFICE IN / OUT alerts are pushed to;
	// empty turns the alerts off.
	NtfyTopic string
	// StoragePath is where uploaded files (e.g. leave documents) are written,
	// as a path on the backend's own filesystem — resolved relative to the
	// backend process's working directory unless given as absolute.
	StoragePath string

	SessionTTLHours    int
	CookieSecure       bool
	BootstrapAdminUser string
	BootstrapAdminPass string
}

func Load() *Config {
	return &Config{
		// Render (and most other PaaS hosts) assign the listen port at
		// runtime via PORT and route traffic to exactly that port — BACKEND_PORT
		// stays the local-dev override since PORT isn't set there.
		BackendPort:    getEnv("PORT", getEnv("BACKEND_PORT", "8080")),
		DBHost:         getEnv("DB_HOST", "postgres"),
		DBPort:         getEnv("DB_PORT", "5432"),
		DBUser:         getEnv("POSTGRES_USER", "hr_admin"),
		DBPassword:     getEnv("POSTGRES_PASSWORD", ""),
		DBName:         getEnv("POSTGRES_DB", "hr_system"),
		DBSSLMode:      getEnv("DB_SSLMODE", "disable"),
		RedisHost:      getEnv("REDIS_HOST", "redis"),
		RedisPort:      getEnv("REDIS_PORT", "6379"),
		RedisDB:        getEnvInt("REDIS_DB", 0),
		FaceServiceURL: getEnv("FACE_SERVICE_URL", "http://localhost:8001"),
		StoragePath:    getEnv("STORAGE_PATH", "./storage"),
		NtfyTopic:      getEnv("NTFY_TOPIC", ""),

		SessionTTLHours:    getEnvInt("SESSION_TTL_HOURS", 24),
		CookieSecure:       getEnvBool("COOKIE_SECURE", false),
		BootstrapAdminUser: getEnv("BOOTSTRAP_ADMIN_USERNAME", "admin"),
		BootstrapAdminPass: getEnv("BOOTSTRAP_ADMIN_PASSWORD", "changeme_admin_local"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
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
