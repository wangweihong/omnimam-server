#!/usr/bin/env bash

# 可以通过make INSTALL_DIR=xxx的方式设置INSTALL_DIR的值, 其他变量同理。

# 项目源码根目录
SOURCE_ROOT=$(dirname "${BASH_SOURCE[0]}")/../..
# 生成文件存放目录
# 如果未指定变量OUT_DIR, 则采用默认值_output
LOCAL_OUTPUT_ROOT="${SOURCE_ROOT}/${OUT_DIR:-_output}"


# 设置安装目录
# 如果未指定变量INSTALL_DIR, 则采用默认值/tmp/installation
readonly INSTALL_DIR=${INSTALL_DIR:-/tmp/installation}
mkdir -p ${INSTALL_DIR}
readonly ENV_FILE=${SOURCE_ROOT}/scripts/install/environment.sh

# omnimam 配置
readonly omnimam_ROOT_DIR=${omnimam_ROOT_DIR:-/var/lib/omnimam}
readonly omnimam_DATA_DIR=${omnimam_DATA_DIR:-${omnimam_ROOT_DIR}/data} # omnimam 各组件数据目录
readonly omnimam_INSTALL_DIR=${omnimam_INSTALL_DIR:-${omnimam_ROOT_DIR}/bin} # omnimam 安装文件存放目录
readonly omnimam_CONFIG_DIR=${omnimam_CONFIG_DIR:-${omnimam_ROOT_DIR}/conf} # omnimam 配置文件存放目录
readonly omnimam_LOG_DIR=${omnimam_LOG_DIR:-/var/log/omnimam} # omnimam 日志文件存放目录
readonly omnimam_DEBUG_DIR=${omnimam_DEBUG_DIR:-${omnimam_ROOT_DIR}/debug} # omnimam 调试信息文件存放目录
readonly CA_FILE=${CA_FILE:-${omnimam_CONFIG_DIR}/cert/ca.pem} # ca

# database 配置
readonly APISERVER_DATABASE_TYPE=${APISERVER_DATABASE_TYPE:-postgresql}
readonly APISERVER_DATABASE_NAME=${APISERVER_DATABASE_NAME:-omnimam}
readonly APISERVER_POSTGRES_HOST=${APISERVER_POSTGRES_HOST:-127.0.0.1}
readonly APISERVER_POSTGRES_PORT=${APISERVER_POSTGRES_PORT:-5432}
readonly APISERVER_POSTGRES_USER=${APISERVER_POSTGRES_USER:-omnimam}
readonly APISERVER_POSTGRES_PASSWORD=${APISERVER_POSTGRES_PASSWORD:-omnimam}
readonly APISERVER_MYSQL_HOST=${APISERVER_MYSQL_HOST:-127.0.0.1}
readonly APISERVER_MYSQL_USER=${APISERVER_MYSQL_USER:-omnimam}
readonly APISERVER_MYSQL_PASSWORD=${APISERVER_MYSQL_PASSWORD:-omnimam}

# apiserver 配置
readonly APISERVER_RUNTIME_DEBUG_OUTPUT_DIR=${APISERVER_RUNTIME_DEBUG_OUTPUT_DIR:-${omnimam_DEBUG_DIR}/apiserver}
readonly APISERVER_INSECURE_BIND_ADDRESS=${APISERVER_INSECURE_BIND_ADDRESS:-0.0.0.0}
readonly APISERVER_INSECURE_BIND_PORT=${APISERVER_INSECURE_BIND_PORT:-8080}
readonly APISERVER_SECURE_BIND_ADDRESS=${APISERVER_SECURE_BIND_ADDRESS:-0.0.0.0}
readonly APISERVER_SECURE_BIND_PORT=${APISERVER_SECURE_BIND_PORT:-8443}
readonly APISERVER_SECURE_TLS_CERT_FILE=${APISERVER_SECURE_TLS_CERT_FILE:-${omnimam_CONFIG_DIR}/cert/apiserver.crt}
readonly APISERVER_SECURE_TLS_CERT_KEY=${APISERVER_SECURE_TLS_CERT_KEY:-${omnimam_CONFIG_DIR}/cert/apiserver.key}
readonly APISERVER_ASSET_UPLOAD_CHUNK_TEMP_DIR=${APISERVER_ASSET_UPLOAD_CHUNK_TEMP_DIR:-${omnimam_DATA_DIR}/upload-parts}
readonly APISERVER_ASSET_UPLOAD_CHUNK_CLEANUP_HOURS=${APISERVER_ASSET_UPLOAD_CHUNK_CLEANUP_HOURS:-24}
readonly APISERVER_ENGINE_HEALTH_INTERVAL=${APISERVER_ENGINE_HEALTH_INTERVAL:-30s}
readonly APISERVER_MCP_PUBLIC_BASE_URL=${APISERVER_MCP_PUBLIC_BASE_URL:-http://127.0.0.1:8080}

# workflow runtime 配置
readonly APISERVER_WORKFLOW_RUNTIME_ENABLED=${APISERVER_WORKFLOW_RUNTIME_ENABLED:-true}
readonly APISERVER_CONDUCTOR_BASE_URL=${APISERVER_CONDUCTOR_BASE_URL:-http://127.0.0.1:8081/api}
# genconfig 要求占位变量非空；默认值包含 YAML 空字符串引号，生成后仍表示未配置认证。
readonly APISERVER_CONDUCTOR_AUTH_KEY=${APISERVER_CONDUCTOR_AUTH_KEY:-\"\"}
readonly APISERVER_CONDUCTOR_AUTH_SECRET=${APISERVER_CONDUCTOR_AUTH_SECRET:-\"\"}
