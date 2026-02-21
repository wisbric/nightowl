.PHONY: build test test-integration lint fmt sqlc migrate-up migrate-down docker clean check seed

BIN := bin/nightowl
DATABASE_URL ?= postgres://nightowl:nightowl@localhost:5432/nightowl?sslmode=disable

build:
	go build -trimpath -o $(BIN) ./cmd/nightowl

test:
	go test -race -count=1 ./...

test-integration:
	go test -race -count=1 -tags=integration ./...

lint:
	golangci-lint run ./...

fmt:
	goimports -w -local github.com/wisbric/nightowl .
	gofmt -s -w .

sqlc:
	sqlc generate

migrate-up:
	migrate -database "$(DATABASE_URL)" -path migrations/global up

migrate-down:
	migrate -database "$(DATABASE_URL)" -path migrations/global down

docker:
	docker build -t nightowl:dev .

seed:
	go run ./cmd/nightowl -mode seed

clean:
	rm -rf bin/ coverage.out internal/db/

check: lint test
	go vet ./...
