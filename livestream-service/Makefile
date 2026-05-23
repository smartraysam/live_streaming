.PHONY: run build test docs

run:
	go run ./cmd/server

build:
	go build ./...

test:
	go test ./...

docs:
	swag init -g cmd/server/main.go
