.PHONY: build test race vet lint fmt run clean

build:
	go build -trimpath -o bin/permguard ./cmd/permguard

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w cmd internal

run:
	go run ./cmd/permguard

clean:
	$(RM) bin/permguard
