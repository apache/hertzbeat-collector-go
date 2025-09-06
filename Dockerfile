# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership.  The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.


# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# 安装 git 以支持 go mod 下载私有依赖（如有需要）
RUN apk add --no-cache git

# 复制 go.mod 和 go.sum 并下载依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建二进制文件
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o collector ./cmd/main.go

# Production stage
FROM alpine:3.19

WORKDIR /app

# 拷贝构建产物
COPY --from=builder /app/collector /app/collector

# 如有配置文件等需要一并拷贝
# COPY --from=builder /app/etc /app/etc

# 暴露端口（如有需要）
# EXPOSE 1158

ENTRYPOINT ["./collector"]
