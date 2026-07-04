package config

type Wallet struct {
	Port               string
	DatabaseURL        string
	RedisAddr          string
	NATSURL            string
	XCashBaseURL       string
	XCashAppID         string
	XCashHMACKey       string
	XCashWebhookSecret string
}

func LoadWallet() Wallet {
	return Wallet{
		Port:               getEnv("PORT", "8081"),
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://wallet:wallet@localhost:5433/wallet?sslmode=disable"),
		RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
		NATSURL:            getEnv("NATS_URL", "nats://localhost:4222"),
		XCashBaseURL:       getEnv("XCASH_BASE_URL", "http://localhost:6688"),
		XCashAppID:         getEnv("XCASH_APPID", ""),
		XCashHMACKey:       getEnv("XCASH_HMAC_KEY", ""),
		XCashWebhookSecret: getEnv("XCASH_WEBHOOK_SECRET", ""),
	}
}
