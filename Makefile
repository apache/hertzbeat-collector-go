.PHONY:
fmt:
	go fmt ./...

.PHONY:
lint:
	golangci-lint run

.PHONY:
vet:
	go vet ./...

.PHONY:
dev:
	go run ./cmd/main.go

.PHONY:
build:
	go build -o bin/app ./cmd/main.go

.PHONY:
test:
	go test -v ./...
