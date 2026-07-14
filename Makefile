.PHONY: build fmt-check test vet check

build:
	go build -o plugxfer ./cmd/plugxfer

fmt-check:
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

test:
	go test -race ./...

vet:
	go vet ./...

check: fmt-check vet test build
