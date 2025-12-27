#
# Licensed to the Apache Software Foundation (ASF) under one or more
# contributor license agreements.  See the NOTICE file distributed with
# this work for additional information regarding copyright ownership.
# The ASF licenses this file to You under the Apache License, Version 2.0
# (the "License"); you may not use this file except in compliance with
# the License.  You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

VERSION_PACKAGE := hertzbeat.apache.org/hertzbeat-collector-go/internal/cmd/version

GO_LDFLAGS += -X $(VERSION_PACKAGE).hcgVersion=$(shell cat VERSION) \
	-X $(VERSION_PACKAGE).gitCommitID=$(GIT_COMMIT)

GIT_COMMIT:=$(shell git rev-parse HEAD)

##@ Golang

.PHONY: fmt
fmt: ## Golang fmt
	go fmt ./...

.PHONY: vet
vet: ## Golang vet
	go vet ./...

.PHONY: dev-run
dev-run: ## Golang dev, run main by run.
	go run cmd/main.go server --config etc/hertzbeat-collector.yaml

.PHONY: prod-run
prod-run: ## Golang prod, run bin by run.
	bin/collector server --config etc/hertzbeat-collector.yaml

# 默认使用当前系统平台，可以通过参数覆盖：make build GOOS=linux GOARCH=amd64
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

.PHONY: build
# build
build: ## Golang build, support cross-compile: make build GOOS=linux GOARCH=amd64
	@version=$$(cat VERSION); \
	echo "Building for $(GOOS)/$(GOARCH)..."; \
	mkdir -p bin/$(GOOS)/$(GOARCH); \
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o bin/$(GOOS)/$(GOARCH)/collector -ldflags "$(GO_LDFLAGS)" cmd/main.go

.PHONY: all-platform-build
all-platform-build: ## Build for all platforms (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64)
	@echo "Building for all platforms..."
	@$(MAKE) build GOOS=linux GOARCH=amd64
	@$(MAKE) build GOOS=linux GOARCH=arm64
	@$(MAKE) build GOOS=darwin GOARCH=amd64
	@$(MAKE) build GOOS=darwin GOARCH=arm64
	@$(MAKE) build GOOS=windows GOARCH=amd64
	@echo "All platform builds completed. Binaries in bin/"
	@find bin -type f -name collector | xargs ls -lh

.PHONY: init
init: ## install base. For proto compile.
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest

.PHONY: api
# generate api proto
api: API_PROTO_FILES := $(wildcard api/*.proto)
api: ## compile api proto files
	protoc --proto_path=./api \
 	       --go_out=paths=source_relative:./api \
 	       --go-http_out=paths=source_relative:./api \
 	       --go-grpc_out=paths=source_relative:./api \
	       $(API_PROTO_FILES)

.PHONY: go-lint
go-lint: ## run golang lint
	golangci-lint run --config tools/linter/golang-ci/.golangci.yml

.PHONY: test
test: ## run golang test
	go test -v ./...

.PHONY: golang-all
golang-all: ## run fmt lint vet build api test
golang-all: fmt go-lint vet build api test
