.PHONY: build test migrate proto dev integration-test

build:
	mkdir -p bin
	go build -o bin/gateway ./cmd/gateway
	go build -o bin/wallet ./cmd/wallet
	go build -o bin/oddsfeed ./cmd/oddsfeed
	go build -o bin/sportsbook ./cmd/sportsbook

test:
	go test ./...

migrate:
	./scripts/migrate.sh up

integration-test:
	./scripts/test-integration.sh

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		internal/proto/wallet.proto \
		internal/proto/oddsfeed.proto \
		internal/proto/sportsbook.proto

dev:
	./scripts/dev-up.sh
