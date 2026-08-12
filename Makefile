.PHONY: fmt fmt-check vet test build check run

fmt:
	go fmt ./...

fmt-check:
	@files=$$(mktemp) && trap 'rm -f "$$files"' EXIT && \
		find . -name '*.go' -not -path './vendor/*' -exec gofmt -l {} + > "$$files" && \
		test ! -s "$$files"

vet:
	go vet ./...

test:
	go test ./...

build:
	go build -o encloud-tui ./cmd/encloud-tui

check: fmt-check vet test build

run:
	go run ./cmd/encloud-tui
