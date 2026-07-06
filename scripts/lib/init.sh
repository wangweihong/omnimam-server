#!/usr/bin/env bash

set -o errexit
set +o nounset
set -o pipefail

SCRIPT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
# 这样做的目的是更好的找到出错的脚本路径
source "${SCRIPT_ROOT}/scripts/lib/log.sh"
source "${SCRIPT_ROOT}/scripts/lib/util.sh"

# 安装脚本错误控制
# 当脚本出错时, 打印对应的错误栈。效果如下
#mkdir: cannot create directory ‘’: No such file or directory
#!!! [0721 03:21:02] Call tree:
#!!! [0721 03:21:02]  1: ./gencert.sh:78 generate_self_signed_certificate(...)
lib::log::install_errexit