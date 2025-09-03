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

##@ Golang

.PHONY:
fmt: ## Golang fmt
	go fmt ./...

.PHONY:
vet: ## Golang vet
	go vet ./...

.PHONY:
dev: ## Golang dev, run main by run.
	go run ./cmd/main.go

.PHONY: build
# build
build: ## Golang build
	@version=$$(cat VERSION); \
	mkdir -p bin/ && go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/ ./...

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
	       --openapi_out=fq_schema_naming=true,default_response=false:. \
	       $(API_PROTO_FILES)

.PHONY: go-lint
go-lint: ## run golang lint
	golangci-lint run --config ./tools/linter/golangci-lint/.golangci.yml

.PHONY: test
test: ## run golang test
	go test -v ./...

.PHONY: golang-all
golang-all: ## run fmt lint vet build api test
golang-all: fmt lint vet build api test
