package main

import (
	"database/sql"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"

	"github.com/go-redis/redis/v8"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/realyoussefhossam/betmonster/internal/proto"
	"github.com/realyoussefhossam/betmonster/internal/shared/config"
	"github.com/realyoussefhossam/betmonster/internal/shared/logging"
	"github.com/realyoussefhossam/betmonster/internal/wallet"
	"github.com/realyoussefhossam/betmonster/internal/wallet/rates"
	"github.com/realyoussefhossam/betmonster/internal/wallet/xcash"
)

func splitTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func main() {
	cfg := config.LoadWallet()
	logger := logging.New()

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to open database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if err := db.Ping(); err != nil {
		logger.Error("failed to ping database", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if err := runMigrations(cfg.DatabaseURL); err != nil {
		logger.Error("failed to run migrations", slog.String("error", err.Error()))
		os.Exit(1)
	}

	store := wallet.NewPGStore(db)
	xc := xcash.NewClient(cfg.XCashBaseURL, cfg.XCashAppID, cfg.XCashHMACKey)
	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	validator := xcash.NewWebhookValidator(cfg.XCashWebhookSecret).WithRedis(redisClient)
	pairs := splitTrim(cfg.SupportedPairs)
	svc := wallet.NewService(store, xc, validator, pairs)

	cacheTTL := 30 * time.Second
	if v := os.Getenv("RATES_CACHE_TTL_SECONDS"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil {
			cacheTTL = time.Duration(secs) * time.Second
		}
	}
	rateCache := rates.NewCache(cacheTTL)
	aggregator := rates.NewAggregator(rateCache,
		rates.NewForexChain(
			rates.NewOpenExchange(),
			rates.NewCoinbaseForex(),
			rates.NewFawazAhmed0(),
			rates.NewMoneyConvert(),
		),
		rates.NewBinance(),
		rates.NewCoinbase(),
		rates.NewKraken(),
		rates.NewKuCoin(),
	)

	grpcServer := grpc.NewServer()
	proto.RegisterWalletServiceServer(grpcServer, wallet.NewGRPCServer(svc, aggregator))

	go startHealthServer(logger, cfg.Port)

	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		logger.Error("failed to listen", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("wallet gRPC starting", slog.String("addr", ":50051"))
	if err := grpcServer.Serve(listener); err != nil {
		logger.Error("wallet gRPC stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func runMigrations(databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	driver, err := pgx.WithInstance(db, &pgx.Config{})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithDatabaseInstance("file://wallet/migrations", "pgx", driver)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

func startHealthServer(logger *slog.Logger, port string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"wallet"}`))
	})
	logger.Info("wallet health starting", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		logger.Error("wallet health stopped", slog.String("error", err.Error()))
	}
}
