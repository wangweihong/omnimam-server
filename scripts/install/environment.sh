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
readonly APISERVER_SECURE_REQUIRED=${APISERVER_SECURE_REQUIRED:-false}
readonly APISERVER_SECURE_TLS_CERT_FILE=${APISERVER_SECURE_TLS_CERT_FILE:-${omnimam_CONFIG_DIR}/cert/apiserver.crt}
readonly APISERVER_SECURE_TLS_CERT_KEY=${APISERVER_SECURE_TLS_CERT_KEY:-${omnimam_CONFIG_DIR}/cert/apiserver.key}
readonly APISERVER_ASSET_UPLOAD_CHUNK_TEMP_DIR=${APISERVER_ASSET_UPLOAD_CHUNK_TEMP_DIR:-${omnimam_DATA_DIR}/upload-parts}
readonly APISERVER_ASSET_UPLOAD_CHUNK_CLEANUP_HOURS=${APISERVER_ASSET_UPLOAD_CHUNK_CLEANUP_HOURS:-24}
readonly APISERVER_ENGINE_HEALTH_INTERVAL=${APISERVER_ENGINE_HEALTH_INTERVAL:-30s}
readonly APISERVER_MCP_PUBLIC_BASE_URL=${APISERVER_MCP_PUBLIC_BASE_URL:-http://127.0.0.1:8080}
readonly APISERVER_APPSTUDIO_WEBHOOK_BASE_URL=${APISERVER_APPSTUDIO_WEBHOOK_BASE_URL:-http://127.0.0.1:8080}

# workflow runtime 配置
readonly APISERVER_WORKFLOW_RUNTIME_ENABLED=${APISERVER_WORKFLOW_RUNTIME_ENABLED:-true}
readonly APISERVER_CONDUCTOR_BASE_URL=${APISERVER_CONDUCTOR_BASE_URL:-http://127.0.0.1:8081/api}
readonly APISERVER_INFRASTRUCTURE_CLIENT_BASE_URL=${APISERVER_INFRASTRUCTURE_CLIENT_BASE_URL:-http://127.0.0.1:8082}
readonly APISERVER_INFRASTRUCTURE_CLIENT_TOKEN=${APISERVER_INFRASTRUCTURE_CLIENT_TOKEN:-omnimam-local-development-infrastructure-service-token-change-me}
# 本地开发固定 OPAQUE setup；生产环境应覆盖为稳定且受保护的部署密钥。
readonly APISERVER_AUTH_JWT_SECRET=${APISERVER_AUTH_JWT_SECRET:-omnimam-local-development-jwt-secret-change-me}
readonly APISERVER_OPAQUE_SERVER_SETUP=${APISERVER_OPAQUE_SERVER_SETUP:-010020abc0c2392ed018c208ad37c5244d207242b90d3ba4ba4f27c1b0ac6e777bd800002074739b77bd3c26c247dad430b6d3a110cc20d1f25ea1a6f209d96bdfad51503f0040e1735bdb4d9b7cef37fe280642b65f448a0c498cc0d7d67b78d9209d03d669b65ae8be55d6b4d7b325b0f4ae7ee7769b1b8ff9fcb4ab3e6e0a1f468ac866a7f90000}
# genconfig 要求占位变量非空；默认值包含 YAML 空字符串引号，生成后仍表示未配置认证。
readonly APISERVER_CONDUCTOR_AUTH_KEY=${APISERVER_CONDUCTOR_AUTH_KEY:-\"\"}
readonly APISERVER_CONDUCTOR_AUTH_SECRET=${APISERVER_CONDUCTOR_AUTH_SECRET:-\"\"}

# Docker Compose 配置统一由本文件提供；source 后导出给 Compose 插值使用。
readonly OMNIMAM_VERSION="${OMNIMAM_VERSION:-dev}"
readonly OMNIMAM_ENVIRONMENT="${OMNIMAM_ENVIRONMENT:-development}"
readonly OMNIMAM_AUTH_JWT_SECRET="${OMNIMAM_AUTH_JWT_SECRET:-${APISERVER_AUTH_JWT_SECRET}}"
readonly OMNIMAM_OPAQUE_SERVER_SETUP="${OMNIMAM_OPAQUE_SERVER_SETUP:-${APISERVER_OPAQUE_SERVER_SETUP}}"
readonly OMNIMAM_INFRA_SERVICE_TOKEN="${OMNIMAM_INFRA_SERVICE_TOKEN:-${APISERVER_INFRASTRUCTURE_CLIENT_TOKEN}}"
readonly OMNIMAM_MCP_PUBLIC_BASE_URL="${OMNIMAM_MCP_PUBLIC_BASE_URL:-https://apiserver:8443}"
readonly OMNIMAM_APPSTUDIO_WEBHOOK_BASE_URL="${OMNIMAM_APPSTUDIO_WEBHOOK_BASE_URL:-https://apiserver:8443}"
readonly OMNIMAM_MCP_RUNTIME_CA_FILE="${OMNIMAM_MCP_RUNTIME_CA_FILE:-/etc/omnimam/mcp-tls/ca.crt}"
readonly OMNIMAM_DOCKER_API_VERSION="${OMNIMAM_DOCKER_API_VERSION:-v1.45}"
readonly OMNIMAM_STORAGE_ROOT="${OMNIMAM_STORAGE_ROOT:-/var/lib/omnimam/assets}"
if [[ -z "${OMNIMAM_INFRA_PROFILE_IMAGES:-}" ]]; then
    OMNIMAM_INFRA_PROFILE_IMAGES='{"agent.hermes@1.0":"omnimam/hermes-agent:v2026.8.3","agent.coding@1.0":"omnimam/coding-agent:1.18.13","appstudio.preview.static-web@1.0":"omnimam/appstudio-nginx:1.27-alpine","appstudio.preview.web-backend@1.0":"omnimam/appstudio-node:22-alpine","appstudio.build.static-web@1.0":"omnimam/appstudio-node:22-alpine","appstudio.build.web-backend@1.0":"omnimam/appstudio-node:22-alpine","appstudio.production.static-web@1.0":"omnimam/appstudio-nginx:1.27-alpine","appstudio.production.web-backend@1.0":"omnimam/appstudio-nginx:1.27-alpine"}'
fi
readonly OMNIMAM_INFRA_PROFILE_IMAGES

export OMNIMAM_VERSION
export OMNIMAM_ENVIRONMENT
export OMNIMAM_AUTH_JWT_SECRET
export OMNIMAM_OPAQUE_SERVER_SETUP
export OMNIMAM_INFRA_SERVICE_TOKEN
export OMNIMAM_MCP_PUBLIC_BASE_URL
export OMNIMAM_APPSTUDIO_WEBHOOK_BASE_URL
export OMNIMAM_MCP_RUNTIME_CA_FILE
export OMNIMAM_DOCKER_API_VERSION
export OMNIMAM_STORAGE_ROOT
export OMNIMAM_INFRA_PROFILE_IMAGES
