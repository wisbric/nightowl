.PHONY: build test test-integration lint fmt sqlc migrate-up migrate-down docker clean check

BIN := bin/opswatch
DATABASE_URL ?= postgres://opswatch:opswatch@localhost:5432/opswatch?sslmode=disable

build:
	go build -trimpath -o $(BIN) ./cmd/opswatch

test:
	go test -race -count=1 ./...

test-integration:
	go test -race -count=1 -tags=integration ./...

lint:
	golangci-lint run ./...

fmt:
	goimports -w -local github.com/wisbric/opswatch .
	gofmt -s -w .

sqlc:
	sqlc generate

migrate-up:
	migrate -database "$(DATABASE_URL)" -path migrations/global up

migrate-down:
	migrate -database "$(DATABASE_URL)" -path migrations/global down

docker:
	docker build -t opswatch:dev .

clean:
	rm -rf bin/ coverage.out internal/db/

check: lint test
	go vet ./...
