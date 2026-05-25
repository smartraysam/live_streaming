.PHONY: run build test docs local-up local-down local-init local-run

run:
	go run ./cmd/server

build:
	go build ./...

test:
	go test ./...

docs:
	swag init -g cmd/server/main.go

local-up:
	docker compose -f docker-compose.local.yml up -d

local-down:
	docker compose -f docker-compose.local.yml down -v

local-init:
	chmod +x scripts/init-localstack.sh
	AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test ./scripts/init-localstack.sh

local-run:
	AWS_ACCESS_KEY_ID=$(AWS_ACCESS_KEY_ID) \
	AWS_SECRET_ACCESS_KEY=$(AWS_SECRET_ACCESS_KEY) \
	AWS_REGION=us-west-1 \
	AWS_ENDPOINT_URL=http://localhost:4566 \
	USE_MOCK_IVS=true \
	go run ./cmd/server
