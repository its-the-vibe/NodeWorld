.PHONY: build test vet fmt run

build:
	go build -o bin/nodeworld .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

run:
	go run . $(ARGS)
