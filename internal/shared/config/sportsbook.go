package config

type Sportsbook struct {
	Port                  string
	GRPCPort              string
	DatabaseURL           string
	OddsFeedServiceAddr   string
	WalletServiceAddr     string
	SettleIntervalSeconds int
}

func LoadSportsbook() Sportsbook {
	return Sportsbook{
		Port:                  getEnv("PORT", "8083"),
		GRPCPort:              getEnv("GRPC_PORT", "50053"),
		DatabaseURL:           getEnv("DATABASE_URL", "postgres://wallet:wallet@localhost:5433/sportsbook?sslmode=disable"),
		OddsFeedServiceAddr:   getEnv("ODDSFEED_SERVICE_ADDR", "localhost:50052"),
		WalletServiceAddr:     getEnv("WALLET_SERVICE_ADDR", "localhost:50051"),
		SettleIntervalSeconds: getEnvInt("SETTLE_INTERVAL_SECONDS", 60),
	}
}
