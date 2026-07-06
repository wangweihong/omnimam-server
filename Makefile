# .DEFAULT_GOAL为makefile自带变量, 用于设置默认目标
# https://www.gnu.org/software/make/manual/html_node/Special-Variables.html
.DEFAULT_GOAL := all

# Build options
# 代码根目录
ROOT_PACKAGE=github.com/wangweihong/omnimam
# 程序版本代码所在目录
VERSION_PACKAGE=github.com/wangweihong/gotoolbox/backend/pkg/version

.PHONY: all
all: tidy gen proto format lint cover build

include scripts/make-rules/common.mk # make sure include common.mk at the first include line
include scripts/make-rules/golang.mk
include scripts/make-rules/frontend-image.mk
include scripts/make-rules/image.mk
include scripts/make-rules/tools.mk
include scripts/make-rules/gen.mk
include scripts/make-rules/dependencies.mk
include scripts/make-rules/swagger.mk
include scripts/make-rules/proto.mk
include scripts/make-rules/certs.mk
include scripts/make-rules/example.mk

# Usage

define USAGE_OPTIONS

Options:
  DEBUG            Whether to generate debug symbols. Default is 0.
  BINS             The binaries to build. Default is all of cmd.
                   This option is available when using: make build/build.multiarch
                   Example: make build BINS="apiserver taskworker"
  IMAGES           Backend images to make. Default is all dir under build/docker/*.
                   This option is available when using: make image/image.multiarch/push/
                   Example: make image.multiarch IMAGES="apiserver taskworker"
  FRONTEND_IMAGE   Frontend image name. This option is available when using:
                   make frontend.image/frontend.image.push
                   Example: make frontend.image FRONTEND_VERSION=latest
  REGISTRY_PREFIX  Docker registry prefix. Default is "".
                   Example: make push REGISTRY_PREFIX=harbor.registry.wang/exampled VERSION=v1.6.2
  PLATFORMS        The multiple platforms to build. Default is linux/amd64 and linux/arm64.
                   This option is available when using: make build.multiarch/image.build.multiarch/build.image.multiarch
                   Example: make image.build.multiarch IMAGES="apiserver taskworker" PLATFORMS="linux/amd64 linux/arm64".
                   Support PLATFORMS check `go tool dist list` shows.
  VERSION          The version information compiled into binaries.
                   The default is obtained from gsemver or git.
  V                Set to 1 enable verbose build. Default is 0.
endef
export USAGE_OPTIONS

## build: Build source code for host platform.
.PHONY: build
build:
	@$(MAKE) go.build

## verify: Run read-only quality gates without generating or formatting files.
.PHONY: verify
verify: lint test build

## build.multiarch: Build source code for multiple platforms. See option PLATFORMS.
.PHONY: build.multiarch
build.multiarch:
	@$(MAKE) go.build.multiarch

## image: Build docker images for host arch.
.PHONY: image
image:
	@$(MAKE) image.build

.PHONY: compose
compose: image frontend.image
	@OMNIMAM_IMAGE_TAG=$(VERSION)-$(shell $(GO) env GOHOSTARCH) \
		OMNIMAM_FRONTEND_IMAGE_TAG=$(FRONTEND_VERSION) \
		OMNIMAM_REGISTRY_PREFIX=$(REGISTRY_PREFIX) \
		docker compose -f deployments/docker-compose.yaml down
	@OMNIMAM_IMAGE_TAG=$(VERSION)-$(shell $(GO) env GOHOSTARCH) \
		OMNIMAM_FRONTEND_IMAGE_TAG=$(FRONTEND_VERSION) \
		OMNIMAM_REGISTRY_PREFIX=$(REGISTRY_PREFIX) \
		docker compose -f deployments/docker-compose.yaml up -d

## gobuild.push.multiarch: Build source code in docker golang container and docker image for multiple platforms, push images to registry. See option PLATFORMS.
.PHONY: gobuild.push.multiarch
gobuild.push.multiarch:
	@$(MAKE) image.gobuild.multiarch

## push: Build docker images for host arch and push images to registry.
.PHONY: push
push:
	@$(MAKE) image.push

## push.multiarch: Build docker images for multiple platforms and push images to registry. See option PLATFORMS.
.PHONY: push.multiarch
push.multiarch:
	@$(MAKE) image.build.multiarch

## clean: Remove all files that are created by building.
.PHONY: clean
clean:
	@echo "===========> Cleaning all build output"
	@-rm -vrf $(OUTPUT_DIR)

## lint: Check syntax and styling of go sources.
.PHONY: lint
lint:
	@$(MAKE) go.lint

## test: Run unit test.
.PHONY: test
test:
	@$(MAKE) go.test

## cover: Run unit test and get test coverage.
.PHONY: cover
cover:
	@$(MAKE) go.test.cover


## format: Gofmt (reformat) package sources (exclude vendor dir if existed).
# 注意泛型在1.18后才支持，老版本的工具gofmt/goimport检测泛型会出错,需要升级
.PHONY: format
format: tools.verify.golines tools.verify.goimports
	@echo "===========> Formatting codes"
	$(FIND) -type f -name '*.go' | $(XARGS) gofmt -s -w
	$(FIND) -type f -name '*.go' | $(XARGS) goimports -w -local $(ROOT_PACKAGE)
	$(FIND) -type f -name '*.go' | $(XARGS) golines -w --max-len=120 --reformat-tags --shorten-comments --ignore-generated .
	$(GO) mod edit -fmt


## dependencies: Install necessary dependencies.
.PHONY: dependencies
dependencies:
	@$(MAKE) dependencies.run

## tools: Install dependent tools.
.PHONY: tools
tools:
	@$(MAKE) tools.install

## check-updates: Check outdated dependencies of the go projects.
.PHONY: check-updates
check-updates:
	@$(MAKE) go.updates

## tidy: Go mod tidy
.PHONY: tidy
tidy:
	@echo "===========> Run go mod tidy"
	@$(GO) mod tidy -compat=1.26

## gen: Generate all necessary files, such as error code files.
.PHONY: gen
gen:
	@echo "===========> Run gen"
	@$(MAKE) gen.run

## ca: Generate CA files for all components.
# 可以通过make ca CERTIFICATES_SUBJECT=192.168.134.139,127.0.0.1来覆写证书主体
# 可以通过make ca CERTIFICATES=apiserver来覆写证书对象
.PHONY: ca
ca:
	@$(MAKE) gen.ca

## proto: Generate Proto file for gRPC service.
.PHONY: proto
proto:
	@$(MAKE) proto.gen

## configs: Generate application default configs files.
.PHONY: configs
configs:
	@$(MAKE) gen.defaultconfigs

## help: Show this help info.
# 这里会提取target上一行的\#\#注释并生成到Makefile help文档中
.PHONY: help
help: Makefile
	@echo -e "\nUsage: make <TARGETS> <OPTIONS> ...\n\nTargets:"
	@sed -n 's/^##//p' $< | column -t -s ':' | sed -e 's/^/ /'
	@echo "$$USAGE_OPTIONS"
