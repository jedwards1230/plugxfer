.PHONY: build fmt-check test vet tidy-check check

build:
	go build -o plugxfer ./cmd/plugxfer

fmt-check:
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

test:
	go test -race ./...

vet:
	go vet ./...

tidy-check:
	go mod tidy

check: fmt-check tidy-check vet test build
