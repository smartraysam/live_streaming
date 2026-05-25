.PHONY: run build test docs local-up local-down local-init local-run run-real

ENABLE_AUTH ?= false
AWS_REGION ?= eu-west-1
LOCALSTACK_ENDPOINT ?= http://127.0.0.1:4566

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
	AWS_ACCESS_KEY_ID=test \
	AWS_SECRET_ACCESS_KEY=test \
	AWS_REGION=$(AWS_REGION) \
	AWS_ENDPOINT_URL=$(LOCALSTACK_ENDPOINT) \
	ENABLE_AUTH=$(ENABLE_AUTH) \
	USE_MOCK_IVS=true \
	go run ./cmd/server

run-real:
	AWS_REGION=$(AWS_REGION) \
	ENABLE_AUTH=$(ENABLE_AUTH) \
	USE_MOCK_IVS=false \
	AWS_ENDPOINT_URL= \
	go run ./cmd/server
