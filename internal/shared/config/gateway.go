package config

import (
	"os"
	"strconv"
)

type Gateway struct {
	Port                string
	JWKSURL             string
	WalletServiceAddr   string
	AdminUserIDs        string
	CORSAllowedOrigins  string
	SupportedCurrencies string
	SupportedChains     string
	RateLimitRPS        int
	RateLimitBurst      int
	RateLimitBackend    string
	RedisAddr           string
}

func LoadGateway() Gateway {
	return Gateway{
		Port:                getEnv("PORT", "8080"),
		JWKSURL:             getEnv("JWKS_URL", "http://localhost:3000/api/auth/jwks"),
		WalletServiceAddr:   getEnv("WALLET_SERVICE_ADDR", "localhost:50051"),
		AdminUserIDs:        getEnv("ADMIN_USER_IDS", ""),
		CORSAllowedOrigins:  getEnv("CORS_ALLOWED_ORIGINS", ""),
		SupportedCurrencies: getEnv("SUPPORTED_CURRENCIES", "USDT"),
		SupportedChains:     getEnv("SUPPORTED_CHAINS", "anvil"),
		RateLimitRPS:        getEnvInt("RATE_LIMIT_RPS", 100),
		RateLimitBurst:      getEnvInt("RATE_LIMIT_BURST", 100),
		RateLimitBackend:    getEnv("RATE_LIMIT_BACKEND", "memory"),
		RedisAddr:           getEnv("REDIS_ADDR", "redis:6379"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
