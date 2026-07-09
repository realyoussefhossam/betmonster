package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"google.golang.org/grpc"

	"github.com/realyoussefhossam/betmonster/internal/oddsfeed"
	"github.com/realyoussefhossam/betmonster/internal/oddsfeed/providers/azuro"
	"github.com/realyoussefhossam/betmonster/internal/oddsfeed/providers/mock"
	pb "github.com/realyoussefhossam/betmonster/internal/proto"
	"github.com/realyoussefhossam/betmonster/internal/shared/config"
	"github.com/realyoussefhossam/betmonster/internal/shared/logging"
)

func main() {
	cfg := config.LoadOddsFeed()
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

	store := oddsfeed.NewPGStore(db)
	cache := oddsfeed.NewCache(cfg.RedisAddr, time.Duration(cfg.SyncIntervalSeconds)*time.Second)
	bus, err := oddsfeed.NewEventBus(cfg.NATSURL, logger)
	if err != nil {
		logger.Error("failed to connect to nats", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer bus.Close()

	providerNames := splitTrim(cfg.Providers)
	providers := buildProviders(providerNames, cfg, logger)
	if len(providers) == 0 {
		logger.Error("no providers configured")
		os.Exit(1)
	}
	svc := oddsfeed.NewService(store, providers, cache, bus, logger)

	grpcServer := grpc.NewServer()
	pb.RegisterOddsFeedServiceServer(grpcServer, oddsfeed.NewGRPCServer(svc))

	go startHealthServer(logger, cfg.Port)

	for _, name := range providerNames {
		if err := svc.SyncProvider(context.Background(), name); err != nil {
			logger.Error("initial sync failed", slog.String("provider", name), slog.String("error", err.Error()))
		}
	}

	scheduler := oddsfeed.NewScheduler(svc, providerNames, time.Duration(cfg.SyncIntervalSeconds)*time.Second, logger)
	go scheduler.Start(context.Background())

	ws := oddsfeed.NewWebSocketWorker(svc, providers, logger, time.Duration(cfg.WSReconnectMaxSeconds)*time.Second)
	go ws.Start(context.Background())

	listener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		logger.Error("failed to listen", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("oddsfeed gRPC starting", slog.String("addr", ":"+cfg.GRPCPort))
	if err := grpcServer.Serve(listener); err != nil {
		logger.Error("oddsfeed gRPC stopped", slog.String("error", err.Error()))
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
	m, err := migrate.NewWithDatabaseInstance("file://internal/oddsfeed/migrations", "pgx", driver)
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
		w.Write([]byte(`{"status":"healthy","service":"oddsfeed"}`))
	})
	logger.Info("oddsfeed health starting", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		logger.Error("oddsfeed health stopped", slog.String("error", err.Error()))
	}
}

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

func buildProviders(names []string, cfg config.OddsFeed, logger *slog.Logger) []oddsfeed.FeedProvider {
	providers := make([]oddsfeed.FeedProvider, 0, len(names))
	for _, name := range names {
		switch name {
		case "mock":
			providers = append(providers, mock.New())
		case "azuro":
			providers = append(providers, azuro.New(cfg.AzuroGraphURL, cfg.AzuroWSURL, cfg.AzuroEnvironment))
		default:
			logger.Error("unknown provider, skipping", slog.String("provider", name))
		}
	}
	return providers
}
