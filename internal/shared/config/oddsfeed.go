package config

type OddsFeed struct {
	Port                  string
	GRPCPort              string
	DatabaseURL           string
	RedisAddr             string
	NATSURL               string
	Providers             string
	AzuroGraphURL         string
	AzuroWSURL            string
	AzuroEnvironment      string
	SyncIntervalSeconds   int
	WSReconnectMaxSeconds int
}

func LoadOddsFeed() OddsFeed {
	return OddsFeed{
		Port:                  getEnv("PORT", "8082"),
		GRPCPort:              getEnv("GRPC_PORT", "50052"),
		DatabaseURL:           getEnv("DATABASE_URL", "postgres://wallet:wallet@localhost:5433/oddsfeed?sslmode=disable"),
		RedisAddr:             getEnv("REDIS_ADDR", "localhost:6379"),
		NATSURL:               getEnv("NATS_URL", "nats://localhost:4222"),
		Providers:             getEnv("PROVIDERS", "mock"),
		AzuroGraphURL:         getEnv("AZURO_GRAPH_URL", ""),
		AzuroWSURL:            getEnv("AZURO_WS_URL", ""),
		AzuroEnvironment:      getEnv("AZURO_ENVIRONMENT", "PolygonUSDT"),
		SyncIntervalSeconds:   getEnvInt("SYNC_INTERVAL_SECONDS", 60),
		WSReconnectMaxSeconds: getEnvInt("WS_RECONNECT_MAX_SECONDS", 300),
	}
}
