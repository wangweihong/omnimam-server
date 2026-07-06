#!/usr/bin/env bash

# This will canonicalize the path
# 源代码根目录
SCRIPT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")"/.. && pwd -P)
source "${SCRIPT_ROOT}/scripts/lib/init.sh"

# Here we map the output directories across both the local and remote _output
# directories:
#
# *_OUTPUT_ROOT    - the base of all output in that environment.
# *_OUTPUT_SUBPATH - location where golang stuff is built/cached.  Also
#                    persisted across docker runs with a volume mount.
# *_OUTPUT_BINPATH - location where final binaries are placed.  If the remote
#                    is really remote, this is the stuff that has to be copied
#                    back.
# OUT_DIR can come in from the Makefile, so honor it.
# 可以通过make OUT_DIR=xxx的方式设置OUT_DIR的值, 其他类似定义的变量同理。
# 如果未指定变量OUT_DIR, 则采用默认值_output.(相对于${SCRIPT_ROOT}的路径)
readonly LOCAL_OUTPUT_ROOT="${SCRIPT_ROOT}/${OUT_DIR:-_output}"
# 也可以采用以下绝对路径的方式
#readonly LOCAL_OUTPUT_ROOT="${LOCAL_OUTPUT_ROOT:-${SCRIPT_ROOT}/_output}"
readonly LOCAL_OUTPUT_SUBPATH="${LOCAL_OUTPUT_ROOT}/platforms"
readonly LOCAL_OUTPUT_BINPATH="${LOCAL_OUTPUT_SUBPATH}"
readonly LOCAL_OUTPUT_GOPATH="${LOCAL_OUTPUT_SUBPATH}/go"
readonly LOCAL_OUTPUT_IMAGE_STAGING="${LOCAL_OUTPUT_ROOT}/images"