package main

import (
	"database/sql"
	"log/slog"
	"net"
	"net/http"
	"os"

	"google.golang.org/grpc"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/realyoussefhossam/betmonster/internal/proto"
	"github.com/realyoussefhossam/betmonster/internal/shared/config"
	"github.com/realyoussefhossam/betmonster/internal/shared/logging"
	"github.com/realyoussefhossam/betmonster/internal/wallet"
	"github.com/realyoussefhossam/betmonster/internal/wallet/xcash"
)

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
	validator := xcash.NewWebhookValidator(cfg.XCashWebhookSecret)
	svc := wallet.NewService(store, xc, validator)
	grpcServer := grpc.NewServer()
	proto.RegisterWalletServiceServer(grpcServer, wallet.NewGRPCServer(svc))

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
