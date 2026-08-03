# 功能

用于存放容器编排部署模板和配置，例如 docker-compose、kubernetes/helm、Terraform、Bosh。

## Docker Compose 快速启动

先构建后端镜像和独立 frontend 镜像：

```bash
make configs
make image VERSION=latest REGISTRY_PREFIX=omnimam
make frontend.image FRONTEND_VERSION=latest FRONTEND_REGISTRY_PREFIX=omnimam
```

如果当前网络无法直接拉取 Docker Hub 的 `node` 或 `nginx` 镜像，可以覆盖 frontend base image：

```bash
make frontend.image FRONTEND_VERSION=latest FRONTEND_REGISTRY_PREFIX=omnimam \
  FRONTEND_NODE_IMAGE=<your-registry>/node:22-alpine \
  FRONTEND_NGINX_IMAGE=<your-registry>/nginx:1.27-alpine
```

`frontend.image` 默认使用本地已有 base image，不强制 `docker pull`。如果需要构建前刷新 base image：

```bash
make frontend.image FRONTEND_VERSION=latest FRONTEND_REGISTRY_PREFIX=omnimam FRONTEND_PULL=1
```

启动 PostgreSQL、持久化 Redis 队列、Conductor、`apiserver`、`taskworker` 和独立 `notificationworker`：

```bash
docker compose -f deployments/docker-compose.yaml up -d
```

Conductor 的业务元数据和运行历史保存在独立 PostgreSQL 数据库，延迟任务与 Scheduler
队列使用开启 AOF 的 Redis。该组合用于保证六段秒级 cron 按期触发，并避免 PostgreSQL
Queue 的 unack 回收周期放大短周期调度延迟。RedisQueueDAO 使用 Jedis，Redis 可用性由
Compose healthcheck 独立检查，不开启需要 RedissonClient 的 Conductor redis-lock health indicator。

`notificationworker` 不连接 Conductor，也不挂载素材目录。它独立消费 Notification Center
source event、执行规则和保留清理，并将 Notification Outbox 投影为统一 SSE UserEvent。
升级到包含该进程的版本时必须同时更新 `taskworker`，不要让仍内置通知 consumer group
的旧 `taskworker` 与新 `notificationworker` 长期混跑。

默认访问地址：

- Web Console: `http://localhost:9990`
- API Server: `http://localhost:8080`

## 清空测试数据库

仅在测试环境需要重新执行 migration 时，可删除
`deployments_omnimam_postgres_data` 中 OmniMAM 数据库 `public` schema 的所有表：

```bash
deployments/postgres/drop-all-tables.sh --force
```

脚本要求 `omnimam-postgres` 容器正在运行，并会确认容器实际挂载了目标 volume。
它不会删除 volume，也不会清理独立的 `conductor` 数据库。可通过
`OMNIMAM_POSTGRES_CONTAINER` 和 `OMNIMAM_POSTGRES_VOLUME` 覆盖容器名和 volume 名。

如果只需要重建 frontend 镜像：

```bash
make frontend.image FRONTEND_VERSION=latest FRONTEND_REGISTRY_PREFIX=omnimam
docker compose -f deployments/docker-compose.yaml up -d --force-recreate frontend
```
