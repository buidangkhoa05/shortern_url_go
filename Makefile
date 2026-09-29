.PHONY: run build test test-integration docker-up docker-down migrate-up migrate-down

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

test:
	go test ./...

test-integration:
	go test -tags integration ./...

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down
