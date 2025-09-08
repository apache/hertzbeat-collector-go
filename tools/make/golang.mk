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

GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

##@ Golang

.PHONY: fmt
fmt: ## Golang fmt
	@$(LOG_TARGET)
	go fmt ./...

.PHONY: vet
vet: ## Golang vet
	@$(LOG_TARGET)
	go vet ./...

.PHONY: dev
dev: ## Golang dev, run main by run.
	@$(LOG_TARGET)
	go run cmd/main.go server --config etc/hertzbeat-collector.yaml

.PHONY: prod
prod: ## Golang prod, run bin by run.
	@$(LOG_TARGET)
	bin/collector server --config etc/hertzbeat-collector.yaml

.PHONY: build
build: GOOS = linux
build: ## Golang build
	@$(LOG_TARGET)
	@version=$$(cat VERSION); \
	# todo; 添加交叉编译支持
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o bin/collector -ldflags "$(GO_LDFLAGS)" cmd/main.go

.PHONY: init
init: ## install base. For proto compile.
	@$(LOG_TARGET)
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest

.PHONY: api
# generate api proto
api: API_PROTO_FILES := $(wildcard api/*.proto)
api: ## compile api proto files
	@$(LOG_TARGET)
	protoc --proto_path=./api \
 	       --go_out=paths=source_relative:./api \
 	       --go-http_out=paths=source_relative:./api \
 	       --go-grpc_out=paths=source_relative:./api \
	       $(API_PROTO_FILES)

.PHONY: go-lint
go-lint: ## run golang lint
	@$(LOG_TARGET)
	golangci-lint run --config tools/linter/golang-ci/.golangci.yml

.PHONY: test
test: ## run golang test
	@$(LOG_TARGET)
	go test -v ./...
