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

FROM docker.m.daocloud.io/library/busybox AS builder
# FROM busybox:latest AS builder

# prepare hertzbeat data dir
RUN mkdir -p /var/hertzbeat

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/base-nossl:nonroot@sha256:907e776d3a92b2a4c82e02ee282a00bcb3a193dce3adc902e85b7beb7f3fd9dc

COPY --from=builder /var/hertzbeat/ /var/hertzbeat/

# copy binary to image, run make build to generate binary.
# Use TARGETARCH to copy the correct binary for the target platform
ARG TARGETARCH
COPY bin/linux/${TARGETARCH}/collector /usr/local/bin/collector
COPY etc /var/hertzbeat/config/

USER 65532:65532

ENTRYPOINT ["/usr/local/bin/collector", "server", "--config", "/var/hertzbeat/config/hertzbeat-collector.yaml"]
