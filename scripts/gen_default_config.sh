#!/usr/bin/env bash

# 代码根目录
SOURCE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${SOURCE_ROOT}/scripts/common.sh"
# 模板路径
readonly LOCAL_CONFIG_TEMPLATE_PATH="${SOURCE_ROOT}/configs"
readonly LOCAL_CONFIG_ENV_PATH="${SOURCE_ROOT}/scripts/install/environment.sh"

config_dir=${1}
components=${2}

if [ $# -ne 2 ];then
    lib::log::error "Usage: generate_default_config.sh output_dir \"example-server example-cli\""
    exit 1
fi

# 这段脚本的作用是调用./genconfig.sh将指定组件模板目录上各个配置文件模板替换变量值生成到输出目录
# 替换成install/environment.sh中定义的默认值

mkdir -p  ${config_dir}

cd ${SOURCE_ROOT}/scripts

for comp in ${components}
do
  lib::log::info "generate config from template ${LOCAL_CONFIG_TEMPLATE_PATH}/${comp}.yaml to ${config_dir}/${comp}.yaml"
  ./genconfig.sh ${LOCAL_CONFIG_ENV_PATH} ${LOCAL_CONFIG_TEMPLATE_PATH}/${comp}.yaml > ${config_dir}/${comp}.yaml
done
