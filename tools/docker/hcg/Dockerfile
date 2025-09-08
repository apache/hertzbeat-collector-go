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

FROM docker.io/library/busybox@sha256:ab33eacc8251e3807b85bb6dba570e4698c3998eca6f0fc2ccb60575a563ea74 AS builder

# prepare hertzbeat data dir
RUN mkdir -p /var/hertzbeat

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/base-nossl:nonroot@sha256:8981b63f968e829d21351ea9d28cc21127e5f034707f1d8483d2993d9577be0b

COPY --from=builder /var/hertzbeat/ /var/hertzbeat/

# copy binary to image, run make build to generate binary.
COPY bin /usr/local/bin/
COPY etc /var/hertzbeat/config/

USER 65532:65532

ENTRYPOINT ["/usr/local/bin/collector", "server", "--config", "/var/hertzbeat/config/hertzbeat-collector.yaml"]
