# ==============================================================================
# Independent Infrastructure runtime image rules.

DOCKER ?= docker
INFRASTRUCTURE_IMAGES_PULL ?= 0
ifeq ($(INFRASTRUCTURE_IMAGES_PULL),1)
INFRASTRUCTURE_IMAGES_PULL_ARG := --pull
endif

CODING_AGENT_DOCKERFILE ?= $(ROOT_DIR)/build/docker/coding-agent/Dockerfile
CODING_AGENT_BASE_IMAGE ?= ghcr.io/anomalyco/opencode:1.18.13
CODING_AGENT_IMAGE ?= omnimam/coding-agent:1.18.13

HERMES_AGENT_DOCKERFILE ?= $(ROOT_DIR)/build/docker/hermes-agent/Dockerfile
HERMES_AGENT_BASE_IMAGE ?= nousresearch/hermes-agent:v2026.8.3
HERMES_AGENT_IMAGE ?= omnimam/hermes-agent:v2026.8.3

APPSTUDIO_NGINX_DOCKERFILE ?= $(ROOT_DIR)/build/docker/appstudio-nginx/Dockerfile
APPSTUDIO_NGINX_BASE_IMAGE ?= nginx:1.27-alpine
APPSTUDIO_NGINX_IMAGE ?= omnimam/appstudio-nginx:1.27-alpine

APPSTUDIO_NODE_DOCKERFILE ?= $(ROOT_DIR)/build/docker/appstudio-node/Dockerfile
APPSTUDIO_NODE_BASE_IMAGE ?= node:22-alpine
APPSTUDIO_NODE_IMAGE ?= omnimam/appstudio-node:22-alpine

.PHONY: infrastructure.images
infrastructure.images: coding-agent.image hermes-agent.image appstudio-nginx.image appstudio-node.image

.PHONY: coding-agent.image
coding-agent.image:
	@echo "===========> Building coding-agent image $(CODING_AGENT_IMAGE)"
	@$(DOCKER) build \
		$(INFRASTRUCTURE_IMAGES_PULL_ARG) \
		--build-arg OPENCODE_IMAGE=$(CODING_AGENT_BASE_IMAGE) \
		-t $(CODING_AGENT_IMAGE) \
		-f $(CODING_AGENT_DOCKERFILE) \
		$(dir $(CODING_AGENT_DOCKERFILE))

.PHONY: hermes-agent.image
hermes-agent.image:
	@echo "===========> Building Hermes agent image $(HERMES_AGENT_IMAGE)"
	@$(DOCKER) build \
		$(INFRASTRUCTURE_IMAGES_PULL_ARG) \
		--build-arg HERMES_AGENT_IMAGE=$(HERMES_AGENT_BASE_IMAGE) \
		-t $(HERMES_AGENT_IMAGE) \
		-f $(HERMES_AGENT_DOCKERFILE) \
		$(dir $(HERMES_AGENT_DOCKERFILE))

.PHONY: appstudio-nginx.image
appstudio-nginx.image:
	@echo "===========> Building AppStudio Nginx image $(APPSTUDIO_NGINX_IMAGE)"
	@$(DOCKER) build \
		$(INFRASTRUCTURE_IMAGES_PULL_ARG) \
		--build-arg NGINX_IMAGE=$(APPSTUDIO_NGINX_BASE_IMAGE) \
		-t $(APPSTUDIO_NGINX_IMAGE) \
		-f $(APPSTUDIO_NGINX_DOCKERFILE) \
		$(dir $(APPSTUDIO_NGINX_DOCKERFILE))

.PHONY: appstudio-node.image
appstudio-node.image:
	@echo "===========> Building AppStudio Node image $(APPSTUDIO_NODE_IMAGE)"
	@$(DOCKER) build \
		$(INFRASTRUCTURE_IMAGES_PULL_ARG) \
		--build-arg NODE_IMAGE=$(APPSTUDIO_NODE_BASE_IMAGE) \
		-t $(APPSTUDIO_NODE_IMAGE) \
		-f $(APPSTUDIO_NODE_DOCKERFILE) \
		$(dir $(APPSTUDIO_NODE_DOCKERFILE))
