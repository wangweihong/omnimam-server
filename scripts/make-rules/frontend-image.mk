# ==============================================================================
# Independent frontend image rules.
#
# Frontend and Infrastructure runtime base images have no repository Go binary.
# Define IMAGES_DIR before image.mk is loaded so backend image rules ignore them.

ifeq ($(origin IMAGES_DIR),undefined)
IMAGES_DIR := $(filter-out \
	$(ROOT_DIR)/build/docker/frontend \
	$(ROOT_DIR)/build/docker/coding-agent \
	$(ROOT_DIR)/build/docker/hermes-agent \
	$(ROOT_DIR)/build/docker/appstudio-nginx \
	$(ROOT_DIR)/build/docker/appstudio-node,\
	$(wildcard $(ROOT_DIR)/build/docker/*))
endif

FRONTEND_IMAGE ?= frontend
FRONTEND_VERSION ?= $(VERSION)
FRONTEND_REGISTRY_PREFIX ?= $(REGISTRY_PREFIX)
FRONTEND_DOCKERFILE ?= $(ROOT_DIR)/build/docker/frontend/Dockerfile.build
FRONTEND_IMAGE_TAG ?= $(FRONTEND_REGISTRY_PREFIX)/$(FRONTEND_IMAGE):$(FRONTEND_VERSION)
FRONTEND_NODE_IMAGE ?= node:22-alpine
FRONTEND_NGINX_IMAGE ?= nginx:1.27-alpine
FRONTEND_PULL ?= 0
DOCKER ?= docker
ifeq ($(FRONTEND_PULL),1)
FRONTEND_PULL_ARG := --pull
endif

.PHONY: frontend.image
frontend.image:
	@echo "===========> Building frontend docker image $(FRONTEND_IMAGE_TAG)"
	@$(DOCKER) build \
		$(FRONTEND_PULL_ARG) \
		$(_DOCKER_BUILD_EXTRA_ARGS) \
		--build-arg NODE_IMAGE=$(FRONTEND_NODE_IMAGE) \
		--build-arg NGINX_IMAGE=$(FRONTEND_NGINX_IMAGE) \
		-t $(FRONTEND_IMAGE_TAG) \
		-f $(FRONTEND_DOCKERFILE) \
		$(ROOT_DIR)

.PHONY: frontend.image.push
frontend.image.push: frontend.image
	@echo "===========> Pushing frontend docker image $(FRONTEND_IMAGE_TAG)"
	@$(DOCKER) push $(FRONTEND_IMAGE_TAG)
