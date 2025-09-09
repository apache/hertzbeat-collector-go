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

# Multi-stage build for Go application
FROM golang:1.23-alpine AS builder

# Set working directory
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy source code
COPY . .

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o hertzbeat-collector-go ./examples/main.go

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1000 appgroup && adduser -u 1000 -G appgroup -s /bin/sh -D appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/hertzbeat-collector-go .

# Copy configuration file
COPY --from=builder /app/etc/hertzbeat-collector.yaml ./etc/

# Change ownership
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port (if needed for metrics or health checks)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD pgrep hertzbeat-collector-go || exit 1

# Default environment variables
ENV IDENTITY=hertzbeat-collector-go
ENV MODE=public
ENV MANAGER_HOST=127.0.0.1
ENV MANAGER_PORT=1158
ENV MANAGER_PROTOCOL=netty
ENV COLLECTOR_NAME=hertzbeat-collector-go
ENV COLLECTOR_IP=0.0.0.0
ENV COLLECTOR_PORT=8080
ENV LOG_LEVEL=info

# Run the application
CMD ["./hertzbeat-collector-go"]