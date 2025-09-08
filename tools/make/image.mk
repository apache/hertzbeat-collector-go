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
# This is a wrapper to build and push docker image
#

# All make targets related to docker image are defined in this file.

REGISTRY ?= docker.io

TAG ?= $(shell git rev-parse HEAD)

DOCKER := docker
DOCKER_SUPPORTED_API_VERSION ?= 1.32

IMAGES_DIR ?= $(wildcard tools/docker/hcg)

IMAGES ?= hertzbeat-collector-go
IMAGE_PLATFORMS ?= amd64 arm64

BUILDX_CONTEXT = hcg-build-tools-builder

##@ Image

# todo: multi-platform build

.PHONY: image-build
image-build: ## Build docker image
image-build: IMAGE_PLATFORMS = ${shell uname -m}
image-build:
	@$(LOG_TARGET)
	make build
	$(DOCKER) buildx create --name $(BUILDX_CONTEXT) --use; \
	$(DOCKER) buildx use $(BUILDX_CONTEXT); \
	$(DOCKER) buildx build --load \
	 -t $(REGISTRY)/${IMAGES}:$(TAG) \
	 --platform linux/${IMAGE_PLATFORMS} \
	 --file $(IMAGES_DIR)/Dockerfile . ; \
	 $(DOCKER) buildx rm $(BUILDX_CONTEXT)

.PHONY: image-push
image-push: ## Push docker image
image-push:
	@$(LOG_TARGET)
	$(DOCKER) push $(REGISTRY)/$${image}:$(TAG)-$${platform}; \
