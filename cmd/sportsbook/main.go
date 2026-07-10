package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"google.golang.org/grpc"

	pb "github.com/realyoussefhossam/betmonster/internal/proto"
	"github.com/realyoussefhossam/betmonster/internal/shared/config"
	"github.com/realyoussefhossam/betmonster/internal/shared/logging"
	"github.com/realyoussefhossam/betmonster/internal/sportsbook"
)

func main() {
	cfg := config.LoadSportsbook()
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

	store := sportsbook.NewPGStore(db)

	walletClient, err := sportsbook.NewGRPCWalletClient(cfg.WalletServiceAddr)
	if err != nil {
		logger.Error("failed to connect wallet service", slog.String("error", err.Error()))
		os.Exit(1)
	}

	oddsfeedClient, err := sportsbook.NewGRPCOddsFeedClient(cfg.OddsFeedServiceAddr)
	if err != nil {
		logger.Error("failed to connect oddsfeed service; will retry lazily", slog.String("error", err.Error()))
	}

	svc := sportsbook.NewService(store, walletClient, oddsfeedClient)

	grpcServer := grpc.NewServer()
	pb.RegisterSportsbookServiceServer(grpcServer, sportsbook.NewGRPCServer(svc))

	go startHealthServer(logger, cfg.Port)

	scheduler := sportsbook.NewScheduler(svc, time.Duration(cfg.SettleIntervalSeconds)*time.Second, logger)
	go scheduler.Start(context.Background())

	listener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		logger.Error("failed to listen", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("sportsbook gRPC starting", slog.String("addr", ":"+cfg.GRPCPort))
	if err := grpcServer.Serve(listener); err != nil {
		logger.Error("sportsbook gRPC stopped", slog.String("error", err.Error()))
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
	m, err := migrate.NewWithDatabaseInstance("file://internal/sportsbook/migrations", "pgx", driver)
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
		w.Write([]byte(`{"status":"healthy","service":"sportsbook"}`))
	})
	logger.Info("sportsbook health starting", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		logger.Error("sportsbook health stopped", slog.String("error", err.Error()))
	}
}
