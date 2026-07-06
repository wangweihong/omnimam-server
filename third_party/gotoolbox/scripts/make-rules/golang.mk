# ==============================================================================
# Makefile helper functions for golang
#

GO := go
GO_SUPPORTED_VERSIONS ?= 1.13|1.14|1.15|1.16|1.17|1.18|1.19|1.20

# 获取实际的git信息,在编译链接阶段替换到版本包中变量
GO_LDFLAGS += -X $(VERSION_PACKAGE).GitVersion=$(VERSION) \
	-X $(VERSION_PACKAGE).GitCommit=$(GIT_COMMIT) \
	-X $(VERSION_PACKAGE).GitTreeState=$(GIT_TREE_STATE) \
	-X $(VERSION_PACKAGE).BuildDate=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
ifneq ($(DLV),)
	GO_BUILD_FLAGS += -gcflags "all=-N -l"
	LDFLAGS = ""
endif
GO_BUILD_FLAGS += -ldflags "$(GO_LDFLAGS)"

# root makefile define root package path
ifeq ($(ROOT_PACKAGE),)
	$(error the variable ROOT_PACKAGE must be set prior to including golang.mk)
endif

GOPATH := $(shell go env GOPATH)
ifeq ($(origin GOBIN), undefined)
	GOBIN := $(GOPATH)/bin
endif

GOPROXY := $(shell go env GOPROXY)


EXCLUDE_TESTS=$(ROOT_PACKAGE)/test $(ROOT_PACKAGE)/pkg/log $(ROOT_PACKAGE)/third_party $(ROOT_PACKAGE)/tools $(ROOT_PACKAGE)/examples

.PHONY: go.lint
go.lint: tools.verify.golangci-lint
	@echo "===========> Run golangci to lint source codes"
	@golangci-lint run -c $(ROOT_DIR)/.golangci.yaml $(ROOT_DIR)/...


# @set -o pipefail;: 这个部分设置pipefail选项，它确保管道中的任何一个命令失败时整个命令返回非零退出码。@符号表示在运行这个命令时不在终端显示该命令本身。
# $(GO) test -race -cover -coverprofile=$(OUTPUT_DIR)/coverage.out -timeout=10m -shuffle=on -short -v: 这个部分是go test命令的调用，用于运行Go测试。其中的参数含义如下：
#	-race：启用数据竞争检测。
#	-cover：生成测试覆盖率报告。
#	-coverprofile=$(OUTPUT_DIR)/coverage.out：指定生成的测试覆盖率文件的路径和名称。
#	-timeout=10m：设置测试超时时间为10分钟。
#	-shuffle=on：开启测试顺序的随机化。
#	-short：运行短时间运行的测试，排除长时间运行的测试。
#	-v：输出详细的测试信息。
# go list ./...| egrep -v "$(subst ' ','|',$(sort $(EXCLUDE_TESTS)))"：这个部分使用go list命令列出所有Go包，并通过管道将结果传递给egrep命令进行过滤。其中的参数含义如下：
# 	go list ./...：列出当前目录及其子目录下的所有Go包。
#	egrep -v "$(subst $(SPACE),'|',$(sort $(EXCLUDE_TESTS)))"：使用egrep命令对列出的包进行过滤，排除在EXCLUDE_TESTS变量中列出的测试包。$(sort $(EXCLUDE_TESTS))将EXCLUDE_TESTS中的文件名按字母顺序排序，$(subst ' ','|',...)将空格替换为竖线（|）作为egrep命令中的正则表达式的分隔符。
#	2>&1：将标准错误（stderr）重定向到标准输出（stdout）。
# tee >(go-junit-report --set-exit-code >$(OUTPUT_DIR)/report.xml): 这个部分使用tee命令将标准输出的内容复制到两个地方。首先，通过>(...)语法将标准输出重定向到go-junit-report命令，该命令会将测试结果转换为JUnit格式的XML报告，并通过--set-exit-code选项设置退出码，然后将结果写入$(OUTPUT_DIR)/report.xml文件中。其次，标准输出继续传递到后续的管道或重定向操作。
.PHONY: go.test
go.test: tools.verify.go-junit-report
	@echo "===========> Run unit test"
	@echo "EXCLUDE_TESTS: $(EXCLUDE_TESTS)"
	@set -o pipefail;$(GO) test -race -cover -coverprofile=$(OUTPUT_DIR)/coverage.out \
		-timeout=10m -shuffle=on -short -v `go list ./...|\
		egrep -v $(subst $(SPACE),'|',$(sort $(EXCLUDE_TESTS)))` 2>&1 | \
		tee >(go-junit-report --set-exit-code >$(OUTPUT_DIR)/report.xml)
	@sed -i '/mock_.*.go/d' $(OUTPUT_DIR)/coverage.out # remove mock_.*.go files from test coverage
	@$(GO) tool cover -html=$(OUTPUT_DIR)/coverage.out -o $(OUTPUT_DIR)/coverage.html

.PHONY: go.test.cover
go.test.cover: go.test
	@$(GO) tool cover -func=$(OUTPUT_DIR)/coverage.out | \
		awk -v target=$(COVERAGE) -f $(ROOT_DIR)/scripts/coverage.awk

.PHONY: go.updates
go.updates: tools.verify.go-mod-outdated
	@$(GO) list -u -m -json all | go-mod-outdated -update -direct
