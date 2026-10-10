# Coze Studio 生产部署说明（HZ 定制版）

本文适用于以下部署方式：

```text
Jenkins 构建 hz-agent-server 和 hz-agent-web 镜像
运维单独提供 MySQL、Redis、Elasticsearch、MinIO、Milvus 和消息队列
目标服务器只使用 docker run 启动前后端容器
```

## 1. Jenkins 构建镜像

Jenkins 必须在项目根目录执行，两个 Dockerfile 都使用项目根目录 `.` 作为构建上下文：

```bash
docker build \
  -f backend/Dockerfile \
  -t hz-agent-server:latest \
  .

docker build \
  -f frontend/Dockerfile \
  -t hz-agent-web:latest \
  .
```

镜像名称约定如下：

```text
后端镜像：hz-agent-server:latest
前端镜像：hz-agent-web:latest
```

如果 Jenkins 构建节点和目标服务器架构不同，例如目标服务器是 `linux/amd64`，可以使用 `docker buildx build --platform linux/amd64 ... --load` 构建两个镜像。

## 2. 服务器准备

目标服务器需要安装 Docker Engine，并准备一份与镜像代码版本对应的源码目录，例如：

```text
/opt/coze/coze-studio/
```

目录中至少需要保留以下文件，因为它们会被挂载到容器：

```text
docker/.env.prod
backend/conf/
docker/nginx/nginx.conf
docker/nginx/conf.d/default.conf
docker/volumes/elasticsearch/es_index_schema/coze_resource.index-template.json
```

## 3. 配置外部中间件

编辑：

```text
docker/.env.prod
```

将其中的默认容器服务名替换成运维提供的真实地址和凭据，至少确认：

```text
MYSQL_HOST、MYSQL_PORT、MYSQL_DSN
REDIS_ADDR、REDIS_DB、REDIS_PASSWORD
ES_ADDR、ES_USERNAME、ES_PASSWORD
MINIO_ENDPOINT、MINIO_API_HOST、MINIO_AK、MINIO_SK
MILVUS_ADDR、MILVUS_USER、MILVUS_PASSWORD
MQ_NAME_SERVER 及消息队列认证信息
模型服务地址、模型名称和 API Key
```

库表初始化和更新由运维单独处理，不在本部署文档中展开。

`docker/.env.prod` 是 Go 应用读取的 shell 风格配置，包含 `export` 和变量引用，不能直接作为 `docker run --env-file` 使用。后端会通过挂载到 `/app/.env` 的文件读取它。

## 4. 同步 Elasticsearch 资源模板

资源库使用 `coze_resource` 索引，模板文件为：

```text
docker/volumes/elasticsearch/es_index_schema/coze_resource.index-template.json
```

在运维提供的 Elasticsearch 上注册模板：

```bash
curl -fsS \
  -u '<ES_USERNAME>:<ES_PASSWORD>' \
  -X PUT '<ES_ADDR>/_index_template/coze_resource' \
  -H 'Content-Type: application/json' \
  --data-binary @docker/volumes/elasticsearch/es_index_schema/coze_resource.index-template.json
```

如果 ES 没有启用账号认证，可以去掉 `-u` 参数。模板使用 `smartcn` 分析器，外部 Elasticsearch 需要具备 `analysis-smartcn` 插件；如果 `coze_resource` 索引已经存在，更新模板不会自动修改已有索引的 mapping。

## 5. 导入镜像和准备网络

在目标服务器的源码目录执行：

```bash
cd /opt/coze/coze-studio

docker load -i hz-agent-server.tar.gz
docker load -i hz-agent-web.tar.gz
docker pull python:3.12-slim-bookworm

mkdir -p /data/hz-agent/coze/runtime/skills /data/hz-agent/coze/runtime/conversations
docker network inspect coze-network >/dev/null 2>&1 || docker network create coze-network
```

## 6. 启动后端容器

后端需要加入 `coze-network`，并挂载生产配置、Docker Socket 和 Skill runtime 目录：

```bash
docker run -d \
  --pull never \
  --name hz-agent-server \
  --restart always \
  --network coze-network \
  --network-alias coze-server \
  -v "$(pwd)/docker/.env.prod:/app/.env:ro" \
  -v "$(pwd)/backend/conf:/app/resources/conf:ro" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /data/hz-agent/coze/runtime:/data/hz-agent/coze/runtime:rw \
  hz-agent-server:latest
```

## 7. 启动前端容器

前端负责静态页面和 API 反向代理，默认只监听服务器本机的 `8888` 端口：

```bash
docker run -d \
  --pull never \
  --name hz-agent-web \
  --restart always \
  --network coze-network \
  -p 127.0.0.1:8888:80 \
  -v "$(pwd)/docker/nginx/nginx.conf:/etc/nginx/nginx.conf:ro" \
  -v "$(pwd)/docker/nginx/conf.d/default.conf:/etc/nginx/conf.d/default.conf:ro" \
  hz-agent-web:latest
```

前端 Nginx 通过 `coze-network` 中的 `coze-server:8888` 网络别名访问后端容器 `hz-agent-server`。外部 HTTPS 网关可以代理到服务器的 `127.0.0.1:8888`。

如果没有外部反向代理且需要直接从其他机器访问，可将端口映射改为 `-p 0.0.0.0:8888:80`，并配置防火墙限制来源。

## 8. 更新容器

只更新后端时：

```bash
docker load -i hz-agent-server.tar.gz
docker rm -f hz-agent-server
# 再次执行第 6 节的后端 docker run 命令
```

只更新前端时：

```bash
docker load -i hz-agent-web.tar.gz
docker rm -f hz-agent-web
# 再次执行第 7 节的前端 docker run 命令
```

重新创建应用容器不会删除外部中间件数据，也不要删除 `/data/hz-agent/coze/runtime`。

## 9. 查看和停止服务

```bash
docker ps --filter name=hz-agent-server --filter name=hz-agent-web
docker logs -f hz-agent-server
docker logs -f hz-agent-web
docker stop hz-agent-web hz-agent-server
```

## 10. MCP 和 Skill 注意事项

MCP Server 如果运行在另一台机器，配置运维提供的实际 HTTP 地址，不要在 `hz-agent-server` 容器中把 `localhost` 当成另一台机器。

Skill 沙箱需要以下资源：

```text
/var/run/docker.sock
/data/hz-agent/coze/runtime/skills
/data/hz-agent/coze/runtime/conversations
python:3.12-slim-bookworm
```

Docker Socket 权限较高，允许 Coze Server 调用宿主机 Docker Engine。不要把 Socket 挂载到 Skill Python 容器，也不要对公网开放 Docker TCP 端口 `2375`。

## 11. 基本验收

```text
1. hz-agent-web 和 hz-agent-server 状态为 running
2. 可以打开登录页面
3. 模型可以正常调用
4. MCP 资源可以创建和测试
5. 智能体可以调用 MCP 工具
6. Skill 可以创建 Python 容器并执行
7. Skill 可以读写当前 Conversation 工作目录
8. 重启前后端后数据仍然存在
```
