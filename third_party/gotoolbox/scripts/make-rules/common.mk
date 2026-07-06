
SHELL := /bin/bash
OLD_SHELL := $(SHELL)
SHELL = $(OLD_SHELL)

# Makefile settings
ifndef V
MAKEFLAGS += --no-print-directory
endif

ifdef DEBUG
# https://www.cmcrossroads.com/article/tracing-rule-execution-gnu-make
# replace shell with debug Makefile log
SHELL = $(warning Building $@$(if $<, (from $<))$(if $?, ($? newer)))$(OLD_SHELL) -x
endif

## include the common make file
## MAKEFILE_LIST: makefile自带的环境变量，包含所有的makefile文件
COMMON_SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))

# 代码目录
ifeq ($(origin ROOT_DIR),undefined)
ROOT_DIR := $(abspath $(shell cd $(COMMON_SELF_DIR)/../.. && pwd -P))
endif

# 输出目录, 包括制品, 测试覆盖率报告等
ifeq ($(origin OUTPUT_DIR),undefined)
OUTPUT_DIR := $(ROOT_DIR)/_output
$(shell mkdir -p $(OUTPUT_DIR))
endif
ifeq ($(origin TOOLS_DIR),undefined)
TOOLS_DIR := $(OUTPUT_DIR)/tools
$(shell mkdir -p $(TOOLS_DIR))
endif
ifeq ($(origin TMP_DIR),undefined)
TMP_DIR := $(OUTPUT_DIR)/tmp
$(shell mkdir -p $(TMP_DIR))
endif

ifeq ($(origin CONFIG_DIR),undefined)
CONFIG_DIR := $(OUTPUT_DIR)/configs
$(shell mkdir -p $(CONFIG_DIR))
endif

ifeq ($(origin CERTIFICATE_DIR),undefined)
CERTIFICATE_DIR := $(OUTPUT_DIR)/cert
$(shell mkdir -p $(CERTIFICATE_DIR))
endif


# set the version number. you should not need to do this
# for the majority of scenarios.
ifeq ($(origin VERSION), undefined)
VERSION := $(shell git describe --tags --always --match='v*')
endif
# Check if the tree is dirty.  default to dirty
GIT_TREE_STATE:="dirty"
ifeq (, $(shell git status --porcelain 2>/dev/null))
	GIT_TREE_STATE="clean"
endif
GIT_COMMIT:=$(shell git rev-parse HEAD)

# Minimum test coverage
ifeq ($(origin COVERAGE),undefined)
COVERAGE := 60
endif

# The OS must be linux when building docker images
PLATFORMS ?= linux/amd64 linux/arm64
# The OS can be linux/windows/darwin when building binaries
# PLATFORMS ?= darwin/amd64 windows/amd64 linux/amd64 linux/arm64

# Set a specific PLATFORM
ifeq ($(origin PLATFORM), undefined)
	ifeq ($(origin GOOS), undefined)
		GOOS := $(shell go env GOOS)
	endif
	ifeq ($(origin GOARCH), undefined)
		GOARCH := $(shell go env GOARCH)
	endif
	PLATFORM := $(GOOS)/$(GOARCH)
else
	GOOS := $(word 1, $(subst /, ,$(PLATFORM)))
	GOARCH := $(word 2, $(subst /, ,$(PLATFORM)))
endif

# Linux command settings
FIND := find . ! -path './third_party/*' ! -path './vendor/*'
XARGS := xargs --no-run-if-empty

# 保证脚本可执行
MAKE_HOOK_EXECUTABLE:= $(shell chmod +x ./scripts/githooks/*)
#    # Copy githook scripts when execute makefile
    # 采取这种方式, 可以实现git hook的统一和强制. 当执行Make任意规则时,强制进行拷贝。因此不需要单独的规则来拷贝
COPY_GITHOOK:=$(shell cp -f ./scripts/githooks/* .git/hooks/)


# Specify tools severity, include: BLOCKER_TOOLS, CRITICAL_TOOLS, TRIVIAL_TOOLS.
# Missing BLOCKER_TOOLS can cause the CI flow execution failed, i.e. `make all` failed.
# Missing CRITICAL_TOOLS can lead to some necessary operations failed. i.e. `make release` failed.
# TRIVIAL_TOOLS are Optional tools, missing these tool have no affect.
BLOCKER_TOOLS ?= gsemver golines go-junit-report golangci-lint goimports codegen deepcopy-gen
CRITICAL_TOOLS ?= swagger mockgen gotests git-chglog  go-mod-outdated go-gitlint
# 可选工具集，缺少不影响
TRIVIAL_TOOLS ?=

COMMA := ,
EMPTY :=
SPACE := $(EMPTY) $(EMPTY)

