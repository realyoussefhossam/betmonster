package config

import (
	"os"
)

type Gateway struct {
	Port              string
	JWKSURL           string
	WalletServiceAddr string
	AdminUserIDs      string
}

func LoadGateway() Gateway {
	return Gateway{
		Port:              getEnv("PORT", "8080"),
		JWKSURL:           getEnv("JWKS_URL", "http://localhost:3000/api/auth/jwks"),
		WalletServiceAddr: getEnv("WALLET_SERVICE_ADDR", "localhost:50051"),
		AdminUserIDs:      getEnv("ADMIN_USER_IDS", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
