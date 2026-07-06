#!/usr/bin/env bash

# genconfig.sh scripts/install/environment.sh configs/component.yaml
# 读取environment.sh中的环境变量并解析到config/example-server模板, 返回解析后的结果
env_file="$1"
template_file="$2"

SOURCE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${SOURCE_ROOT}/scripts/lib/init.sh"

if [ $# -ne 2 ];then
    lib::log::error "Usage: genconfig.sh scripts/install/environment.sh configs/component.yaml"
    exit 1
fi

source  $env_file
declare -A envs

# s/^[^#].*${\(.*\)}.*/\1/p 指的是 ${...}形式表示的变量
for env in $(sed -n 's/^[^#].*${\(.*\)}.*/\1/p' ${template_file})
do
    # env的值为变量名
    # $(eval echo \$${env})是获取变量env存储的变量名的值
    # 确认$env代表的变量是否为空
    if [ -z "$(eval echo \$${env})" ];then
        lib::log::error "environment variable '${env}' not set"
        missing=true
    fi
done

if [ "${missing}" ];then
    lib::log::error 'You may run `source $1` to set these environment'
    exit 1
fi

eval "cat << EOF
$(cat ${template_file})
EOF"