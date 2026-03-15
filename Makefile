.PHONY: build install run lint

build:
	go build -o canopy ./cmd/canopy

install:
	go install ./cmd/canopy

run:
	go run ./cmd/canopy

lint:
	golangci-lint run
