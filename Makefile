.PHONY: build test migrate proto dev

build:
	mkdir -p bin
	go build -o bin/gateway ./cmd/gateway
	go build -o bin/wallet ./cmd/wallet

test:
	go test ./...

migrate:
	./scripts/migrate.sh up

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		internal/proto/wallet.proto

dev:
	./scripts/dev-up.sh
