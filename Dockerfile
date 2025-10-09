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

FROM golang:1.25-alpine3.22 AS golang-builder

ARG GOPROXY
# ENV GOPROXY ${GOPROXY:-direct}
# ENV GOPROXY=https://proxy.golang.com.cn,direct

ENV GOPATH /go
ENV GOROOT /usr/local/go
ENV PACKAGE hertzbeat.apache.org/hertzbeat-collector-go
ENV BUILD_DIR /app

COPY . ${BUILD_DIR}
WORKDIR ${BUILD_DIR}
RUN apk --no-cache add build-base git bash golangci-lint

RUN make init && \
    make fmt && \
    make go-lint &&\
    make build

RUN chmod +x bin/collector

FROM alpine

ARG TIMEZONE
ENV TIMEZONE=${TIMEZONE:-"Asia/Shanghai"}

RUN apk update \
    && apk --no-cache add \
        bash \
        ca-certificates \
        curl \
        dumb-init \
        gettext \
        openssh \
        sqlite \
        gnupg \
        tzdata \
    && ln -sf /usr/share/zoneinfo/${TIMEZONE} /etc/localtime \
    && echo "${TIMEZONE}" > /etc/timezone

COPY --from=golang-builder /app/bin/collector /usr/local/bin/collector
COPY --from=golang-builder /app/etc/hertzbeat-collector.yml /etc/hertzbeat-collector.yml

EXPOSE 8090
ENTRYPOINT ["collector", "server", "--config", "/etc/hertzbeat-collector.yml"]
