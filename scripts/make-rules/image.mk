# ==============================================================================
# Makefile helper functions for docker image
#
DOCKER := docker
DOCKER_SUPPORTED_API_VERSION ?= 1.32

REGISTRY_PREFIX ?= omnimam
BASE_IMAGE = registry.cn-hangzhou.aliyuncs.com/eazycloud/ubuntu:24.04

# EXTRA_ARGS ?= --no-cache
EXTRA_ARGS ?= --pull=false
_DOCKER_BUILD_EXTRA_ARGS := --build-arg BASE_IMAGE=${BASE_IMAGE}

ifdef HTTP_PROXY
_DOCKER_BUILD_EXTRA_ARGS += --build-arg HTTP_PROXY=${HTTP_PROXY}
endif

ifneq ($(EXTRA_ARGS), )
_DOCKER_BUILD_EXTRA_ARGS += $(EXTRA_ARGS)
endif

GO_DOCKER_VERSION ?= 1.26
GOIMAGE_VERSION ?= golang:$(GO_DOCKER_VERSION)

# Determine image files by looking into build/docker/*/Dockerfile
IMAGES_DIR ?= $(wildcard ${ROOT_DIR}/build/docker/*)
# Determine images names by stripping out the dir names
IMAGES ?= $(filter-out tools,$(foreach image,${IMAGES_DIR},$(notdir ${image})))

ifeq (${IMAGES},)
  $(error Could not determine IMAGES, set ROOT_DIR or run in source dir)
endif

.PHONY: image.verify
image.verify:
	$(eval API_VERSION := $(shell $(DOCKER) version | grep -E 'API version: {1,6}[0-9]' | head -n1 | awk '{print $$3} END { if (NR==0) print 0}' ))
	$(eval PASS := $(shell echo "$(API_VERSION) > $(DOCKER_SUPPORTED_API_VERSION)" | bc))
	@if [ $(PASS) -ne 1 ]; then \
		$(DOCKER) -v ;\
		echo "Unsupported docker version. Docker API version should be greater than $(DOCKER_SUPPORTED_API_VERSION)"; \
		exit 1; \
	fi


.PHONY: image.daemon.verify
image.daemon.verify:
	$(eval PASS := $(shell $(DOCKER) version | grep -q -E 'Experimental: {1,5}true' && echo 1 || echo 0))
	@if [ $(PASS) -ne 1 ]; then \
		echo "Experimental features of Docker daemon is not enabled. Please add \"experimental\": true in '/etc/docker/daemon.json' and then restart Docker daemon."; \
		exit 1; \
	fi

# Verify dockerbuildx is installed
.PHONY: image.dockerbuildx.verify
image.dockerbuildx.verify:
	$(eval PASS := $(shell $(DOCKER) buildx version > /dev/null 2>&1  && echo 1 || echo 0))
	@if [ $(PASS) -ne 1 ]; then \
		echo "docker buildx plugin not exist "; \
		exit 1; \
	fi

# Installing QEMU static binaries
.PHONY: image.dockerbuildx.prerequisite
image.dockerbuildx.prerequisite: image.verify image.dockerbuildx.verify
	@docker run --rm --privileged tonistiigi/binfmt:latest --install all >/dev/null

.PHONY: image.build
image.build: image.verify go.build.verify $(addprefix image.build., $(addprefix $(subst /,_,$(PLATFORM))., $(IMAGES)))


.PHONY: image.push
image.push: image.verify go.build.verify $(addprefix image.push., $(addprefix $(subst /,_,$(PLATFORM))., $(IMAGES)))

.PHONY: image.push.%
image.push.%: image.build.%
	$(eval IMAGE := $(word 2,$(subst ., ,$*)))
	$(eval RULEPLATFORM := $(word 1,$(subst ., ,$*)))
	$(eval ARCH := $(word 2,$(subst _, ,$(RULEPLATFORM))))
	$(eval IMAGETAG := $(REGISTRY_PREFIX)/$(IMAGE):$(VERSION)-$(ARCH))

	@echo "===========> Pushing image $(IMAGETAG)"
	$(DOCKER) push $(IMAGETAG)

# 只允许构建当前系统架构镜像. 不支持异构镜像
.PHONY: image.build.%
image.build.%: go.build.%
	$(eval IMAGE := $(word 2,$(subst ., ,$*)))
	$(eval RULEPLATFORM := $(word 1,$(subst ., ,$*)))
	$(eval IMAGE_PLAT := $(subst _,/,$(RULEPLATFORM)))
	$(eval OS := $(word 1,$(subst _, ,$(RULEPLATFORM))))
	$(eval ARCH := $(word 2,$(subst _, ,$(RULEPLATFORM))))
	$(eval IMAGETAG := $(REGISTRY_PREFIX)/$(IMAGE):$(VERSION)-$(ARCH))
	@echo "===========> Building docker image $(IMAGETAG) for command $(IMAGE) $(VERSION) in platform $(IMAGE_PLAT)"
	@if [[ $(shell $(GO) env GOHOSTOS) != $(OS) || $(shell $(GO) env GOHOSTARCH) != $(ARCH) ]] ; then \
		echo "target $(OS)/$(ARCH) doesn't match host os/arch, use image.build.multiarch for different arch image" ; \
		exit 1; \
	fi
	@mkdir -p $(TMP_DIR)/$(IMAGE)
	@# 由于docker build和docker buildx build在处理$TARGETPLATFORM参数时无法兼容。如果通过--build-arg TARGETPLATFORM传递平台参数
	@# 且在Dockerfile顶部设置`ARG TARGETPLATFORM`，会导致docker buildx build无法正确赋值TARGETPLATFORM。
	@# 为了兼容性, 这里采用外部sed直接替换, 毕竟这个规则只支持单一架构
	@# s#\$$TARGETPLATFORM#
	@cat $(ROOT_DIR)/build/docker/$(IMAGE)/Dockerfile.build\
		| sed -e "s#\$$TARGETPLATFORM#$(IMAGE_PLAT)#g" -e "s#__COMMAND__#$(IMAGE)#g" >$(TMP_DIR)/$(IMAGE)/Dockerfile
	@cp $(OUTPUT_DIR)/configs/$(IMAGE).yaml $(TMP_DIR)/$(IMAGE)/ || true
	@cp -rf $(OUTPUT_DIR)/platforms $(TMP_DIR)/$(IMAGE)/
	@DST_DIR=$(TMP_DIR)/$(IMAGE) $(ROOT_DIR)/build/docker/$(IMAGE)/build.sh 2>/dev/null || true
	$(eval BUILD_SUFFIX := $(_DOCKER_BUILD_EXTRA_ARGS) -t $(IMAGETAG) $(TMP_DIR)/$(IMAGE))
	@$(DOCKER) build $(BUILD_SUFFIX)
	@rm -rf $(TMP_DIR)/$(IMAGE)


.PHONY: image.build.multiarch
image.build.multiarch: image.dockerbuildx.prerequisite go.build.verify  \
					$(foreach p,$(subst /,_,$(PLATFORMS)),$(addprefix go.build., $(addprefix $(p)., $(IMAGES)))) \
					$(addprefix image.build.multiarch., $(IMAGES))

# 支持多架构镜像构建，依赖于本地go进行交叉编译
.PHONY: image.build.multiarch.%
image.build.multiarch.%: image.dockerbuildx.prerequisite
	$(eval IMAGE := $(word 1,$(subst ., ,$*)))
	$(eval BUILDPLTFORM := $(word 1,$(subst $(SPACE),$(COMMA),$(PLATFORMS))))
	$(eval IMAGETAG := $(REGISTRY_PREFIX)/$(IMAGE):$(VERSION))
	@echo "===========> Building docker image $(IMAGETAG) for command $(IMAGE) $(VERSION) in platforms $(BUILDPLTFORM)"
	@mkdir -p $(TMP_DIR)/$(IMAGE)
	@cat $(ROOT_DIR)/build/docker/$(IMAGE)/Dockerfile.build\
		| sed -e "s#__COMMAND__#$(IMAGE)#g" >$(TMP_DIR)/$(IMAGE)/Dockerfile
	@cp $(OUTPUT_DIR)/configs/$(IMAGE).yaml $(TMP_DIR)/$(IMAGE)/ || true
	@cp -rf $(OUTPUT_DIR)/platforms $(TMP_DIR)/$(IMAGE)/
	@docker buildx build $(_DOCKER_BUILD_EXTRA_ARGS)\
		--output type=registry \
		--platform  $(BUILDPLTFORM)  \
		--progress plain \
		-t $(IMAGETAG) \
		-f $(TMP_DIR)/$(IMAGE)/Dockerfile $(TMP_DIR)/$(IMAGE)
	@rm -rf $(TMP_DIR)/$(IMAGE)



# 支持多架构镜像构建，采用镜像进行代码编译
.PHONY: image.gobuild.multiarch
image.gobuild.multiarch: image.dockerbuildx.prerequisite $(addprefix image.gobuild.multiarch., $(IMAGES))


.PHONY: image.gobuild.multiarch.%
image.gobuild.multiarch.%: image.dockerbuildx.prerequisite
	$(eval IMAGE := $(word 1,$(subst ., ,$*)))
	$(eval IMAGETAG := $(REGISTRY_PREFIX)/$(IMAGE):$(VERSION))
	$(eval BUILDPLTFORM := $(word 1,$(subst $(SPACE),$(COMMA),$(PLATFORMS))))
	@echo "===========> Building docker image $(IMAGETAG) for command $(IMAGE) $(VERSION) in platforms $(BUILDPLTFORM)"
	@mkdir -p $(TMP_DIR)/$(IMAGE)
	@# Dockerfile ARG参数的作用域问题导致CMD和ENTRYPOINT中的参数无法替换，因此采用sed提前替换
	@cat $(ROOT_DIR)/build/docker/$(IMAGE)/Dockerfile.gobuild\
		| sed "s#__COMMAND__#$(IMAGE)#g" >$(TMP_DIR)/$(IMAGE)/Dockerfile
	@docker buildx build \
		--output type=registry  \
		--platform  $(BUILDPLTFORM)  \
		--build-arg GOLANG_IMAGE=$(GOIMAGE_VERSION) \
		--build-arg BASE_IMAGE=$(BASE_IMAGE) \
		--build-arg GO_BUILDFLAGS='$(GO_BUILD_FLAGS)' \
		--build-arg GO_PROXY=$(GOPROXY) \
		--progress plain \
		-t $(REGISTRY_PREFIX)/$(IMAGE):$(VERSION) \
		-f $(TMP_DIR)/$(IMAGE)/Dockerfile .
	@rm -rf $(TMP_DIR)/$(IMAGE)
