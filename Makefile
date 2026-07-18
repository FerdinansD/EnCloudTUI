.PHONY: fmt vet test build run

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...

build:
	go build ./cmd/encloud-tui

run:
	go run ./cmd/encloud-tui
