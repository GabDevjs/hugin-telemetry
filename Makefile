.PHONY: build check fmt-check
build:
	CGO_ENABLED=0 go build -trimpath -o bin/hugin ./cmd/hugin
fmt-check:
	@test -z "$$(gofmt -s -l .)"
check: fmt-check
	go vet ./...
	go test -race ./...
