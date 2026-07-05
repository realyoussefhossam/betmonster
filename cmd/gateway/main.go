package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/realyoussefhossam/betmonster/internal/auth"
	"github.com/realyoussefhossam/betmonster/internal/gateway"
	"github.com/realyoussefhossam/betmonster/internal/shared/config"
	"github.com/realyoussefhossam/betmonster/internal/shared/logging"
)

func main() {
	cfg := config.LoadGateway()
	logger := logging.New()

	jwksClient, err := auth.NewJWKSClient(context.Background(), cfg.JWKSURL)
	if err != nil {
		logger.Error("failed to initialize jwks", slog.String("error", err.Error()))
		os.Exit(1)
	}

	walletClient, err := gateway.NewWalletClient(cfg.WalletServiceAddr)
	if err != nil {
		logger.Error("failed to connect wallet service", slog.String("error", err.Error()))
		os.Exit(1)
	}

	limiter := gateway.NewRateLimiter(cfg.RateLimitBackend, cfg.RedisAddr, cfg.RateLimitRPS, cfg.RateLimitBurst)
	defer limiter.Close()
	if cfg.RateLimitBackend == "redis" {
		if err := limiter.Ping(context.Background()); err != nil {
			logger.Error("failed to connect to redis", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	server := gateway.NewServer(logger, walletClient, jwksClient, limiter, cfg.AdminUserIDs, cfg.CORSAllowedOrigins, cfg.SupportedCurrencies, cfg.SupportedChains)
	logger.Info("gateway starting", slog.String("port", cfg.Port))
	if err := http.ListenAndServe(":"+cfg.Port, server.Router()); err != nil {
		logger.Error("gateway stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
