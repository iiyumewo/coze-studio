# Coze Studio MCP 集成工作交接记录

> 状态：开发中的交接基线
>
> 记录日期：2026-08-21
>
> 用途：在切换开发机器或大模型会话后，快速恢复项目背景、已完成工作、设计决策和下一步边界。

## 1. 接手时必须先遵守的协作规则

本项目要求严格小步推进：

1. 先讨论当前要解决的问题，双方确认后才能修改代码。
2. 只实现本轮明确讨论通过的内容，不主动补齐完整链路。
3. 修改前说明预计涉及的文件、职责和范围。
4. 每完成一个小步骤就停下来，逐文件用一句话说明改动，等待 Review 后再继续。
5. 不要因为某个功能最终需要列表、编辑、删除、Agent 调用等能力，就在当前步骤顺手实现它们。
6. 生成代码与手写代码需要明确区分，避免大量生成文件掩盖真正需要 Review 的逻辑。
7. 当前工作分支是 `feat/hz-agent`，不要在 `main` 上直接开发。

## 2. 项目与目标背景

仓库是 Coze Studio 开源版，主要目录包括：

```text
backend/   Go 后端
frontend/  React 前端 Monorepo
idl/       Thrift API 定义
docker/    Docker、本地中间件和 Atlas 数据库结构
helm/      Kubernetes/Helm 部署资源
docs/      设计和交接文档
```

当前总体目标是在 Coze Studio 开源版中逐步增加类似 Dify 的 MCP 集成能力。

当前阶段只关注：

```text
用户在资源库配置一个 Streamable HTTP MCP Server
→ Coze 后端测试连接并发现工具
→ 测试成功后保存工作空间级 MCP 配置
```

暂时不实现 Agent 绑定和运行时工具调用。

## 3. 当前 Git 状态

接手时应先运行：

```bash
git branch --show-current
git status --short
git log --oneline main..HEAD
```

记录本文时的状态：

```text
当前分支：feat/hz-agent
相对 main：领先 2 个提交
```

已经提交：

```text
5f8528f4 feat(mcp): add MCP resource configuration UI
15a60e75 feat(mcp): add MCP server storage schema
```

尚未提交的文档：

```text
docs/business-system-integration-and-billing-design.md
docs/mcp-integration-progress-handoff.md
```

除非用户明确要求，不要擅自提交这两份文档。

## 4. 已经完成：MCP 前端配置交互

前端已经在资源库增加 MCP 服务创建入口和配置弹窗。

### 4.1 当前用户交互

用户可以配置：

```text
服务 URL：必填，只允许 http:// 或 https://
服务名称：选填，计划在后端连接后自动获取默认名称
认证方式：无认证 / 自定义请求头
请求头名称：例如 Authorization
请求头值：例如 Bearer secret
```

按钮文案为“测试并添加”。提交期间有 loading，失败会在弹窗内显示错误，成功会提示发现的工具数量。

### 4.2 前端已经约定的 API

接口尚未由后端实现，但前端契约已经存在：

```http
POST /api/mcp_manage/v1/mcp_servers/test_and_create
```

请求示例：

```json
{
  "space_id": "123",
  "name": "MySQL MCP",
  "server_url": "http://127.0.0.1:3000/mcp",
  "transport": "streamable_http",
  "authentication": {
    "type": "header",
    "header_name": "Authorization",
    "header_value": "Bearer secret"
  }
}
```

无认证时：

```json
{
  "authentication": {
    "type": "none"
  }
}
```

预期成功响应：

```json
{
  "code": 0,
  "msg": "",
  "data": {
    "id": "1930000000000000000",
    "name": "MySQL MCP",
    "tool_count": 5
  }
}
```

### 4.3 已修改的前端文件

1. `frontend/packages/arch/bot-api/src/mcp-api.ts`：定义 MCP 认证、测试并创建请求及响应类型，并封装接口调用。
2. `frontend/packages/arch/bot-api/src/index.ts`：从 Bot API 公共入口导出 MCP API 和类型。
3. `frontend/packages/studio/workspace/entry-base/src/pages/library/mcp/create-mcp-server-modal.tsx`：实现 MCP 配置弹窗、校验、loading、错误和成功反馈。
4. `frontend/packages/studio/workspace/entry-base/src/pages/library/mcp/create-mcp-server.ts`：把表单值转换为接口参数并校验响应。
5. `frontend/packages/studio/workspace/entry-base/src/pages/library/mcp/__tests__/create-mcp-server.test.ts`：覆盖 URL、无认证、自定义 Header 和异常响应。
6. `frontend/packages/studio/workspace/entry-base/src/pages/library/hooks/use-entity-configs/use-mcp-config.tsx`：管理 MCP 创建菜单和弹窗状态。
7. `frontend/packages/studio/workspace/entry-base/src/pages/library/index.tsx`：导出 `useMcpConfig`。
8. `frontend/packages/studio/workspace/entry-adapter/src/pages/library/index.tsx`：将 MCP 配置接入资源库页面。
9. `frontend/packages/arch/resources/studio-i18n-resource/src/locales/zh-CN.json`：增加 MCP 中文文案。
10. `frontend/packages/arch/resources/studio-i18n-resource/src/locales/en.json`：增加 MCP 英文文案。
11. `frontend/packages/arch/resources/studio-i18n-resource/src/locale-data.d.ts`：增加 MCP 国际化键类型。

### 4.4 前端验证结果

```text
Vitest：新增 4 个测试全部通过
ESLint：相关包 0 error；输出的 warnings 来自仓库已有文件
Git hooks：license、lint-staged、format 和 commit message 检查通过
```

## 5. 已经完成：`mcp_server` 表

当前只设计一张 MCP 业务表，不增加“服务定义表 + 空间关联表”。每个工作空间配置一次 MCP Server，就产生一条 `mcp_server` 记录。

这意味着：

```text
space A + 相同 URL → space A 自己的一条配置和认证信息
space B + 相同 URL → space B 自己的一条配置和认证信息
```

Coze 记录层面实现空间隔离，但如果两条记录最终连接同一个 MySQL MCP Server，而该 Server 使用同一个 MySQL 账号，那么底层 MySQL 数据仍然共享。底层数据隔离需要 MCP Server 或 MySQL 账号本身提供，Coze 的 `space_id` 不能自动改变远端数据库权限。

### 5.1 表字段

```text
id                  MCP Server ID
space_id            所属工作空间
creator_id          Coze 创建者用户 ID
name                展示名称
description         描述
server_url          Streamable HTTP 地址
transport           当前固定为 streamable_http
auth_type           none 或 header
encrypted_auth      加密后的认证数据
tools               发现到的工具定义 JSON 快照
protocol_version    协商出的 MCP 协议版本
server_version      MCP Server 实现版本
status              1 启用，2 禁用
validation_status   最近一次连接检测状态
last_error          最近一次连接错误
last_checked_at     最近检测时间，毫秒
created_at          创建时间，毫秒
updated_at          更新时间，毫秒
deleted_at          软删除时间
```

索引：

```text
PRIMARY KEY (id)
idx_mcp_server_space_updated_at (space_id, updated_at)
idx_mcp_server_creator_id (creator_id)
```

### 5.2 数据库文件

1. `docker/atlas/migrations/20260821150000_add_mcp_server.sql`：已有数据库的增量迁移。
2. `docker/atlas/migrations/atlas.sum`：Atlas 迁移校验值。
3. `docker/atlas/opencoze_latest_schema.hcl`：Atlas 最新目标结构。
4. `docker/volumes/mysql/schema.sql`：Docker 全新部署初始化结构。
5. `helm/charts/opencoze/files/mysql/schema.sql`：Helm 全新部署初始化结构。

### 5.3 本地数据库状态

记录本文时，本机 `coze-mysql` 的 `opencoze` 数据库已经存在 `mcp_server` 表，Atlas dry-run 输出：

```text
Schema is synced, no changes to be made
```

本地 Navicat/MySQL 连接配置：

```text
Host: 127.0.0.1
Port: 3306
Database: opencoze
User: coze
Password: coze123
SSL: off
```

可以用以下命令检查：

```bash
docker exec coze-mysql mysql -ucoze -pcoze123 -D opencoze \
  -e "SHOW TABLES LIKE 'mcp_server'; SHOW CREATE TABLE mcp_server\\G"
```

## 6. 已确认的后端实现原则

以下原则已经讨论清楚，但代码尚未实现：

1. MCP 接口自动经过 Coze Web Session 登录校验。
2. `creator_id` 必须从当前 session 获取，不能由浏览器提交。
3. 后端必须检查当前用户对 `space_id` 的工作空间权限，语义上使用写权限。
4. 只支持 Streamable HTTP，不支持由服务端启动 stdio 子进程。
5. 使用 MCP 官方 Go SDK，不手写 JSON-RPC 和协议协商。
6. MCP Go SDK 是新的 Go Module 依赖，需要锁定明确版本并更新 `backend/go.mod`、`backend/go.sum`。
7. 测试连接时完成协议协商并执行 `tools/list`，不能只做 HTTP 200 探活。
8. 自定义 Header 需要附加到本次测试会话的所有 MCP HTTP 请求。
9. 禁止自动跟随 HTTP 重定向，避免认证 Header 泄露给另一个地址。
10. 设置明确连接和总请求超时，不能让接口无限等待。
11. 当前自托管场景允许访问 `127.0.0.1` 和私有网络，因为目标 MCP Server 可能部署在同机或内网。
12. 面向不可信多用户开放前，需要另行增加地址白名单、出口代理或 SSRF 防护；当前步骤不顺手实现。
13. 认证 Header 不得写入日志、错误消息或接口响应。
14. 只有连接、协议协商和 `tools/list` 全部成功后才能保存数据库；测试失败不能留下记录。
15. 保存时认证信息必须加密，不能将 `header_value` 明文写入 `encrypted_auth`。
16. 建议为 MCP 使用独立的 `MCP_AES_AUTH_SECRET`，不要与插件凭据密钥混用；新增配置前仍应向用户再次确认文件范围。
17. 用户填写的名称优先；未填写时使用 MCP Server 返回的名称；仍为空时再使用 URL 主机名作为兜底。
18. 同一 URL 可以在不同空间或同一空间以不同认证信息重复配置，因此当前不增加 URL 唯一索引。

## 7. 下一步必须拆成两个小步骤

用户明确要求不要一次实现完整接口链路。后续按以下顺序推进。

### 7.1 第一步：只实现 MCP 客户端适配器

第一步只做内部能力，不开放 HTTP 路由、不写数据库：

```text
引入并锁定 MCP 官方 Go SDK
实现 Streamable HTTP 连接
实现 none / custom header
完成协议协商
执行 tools/list
返回服务信息、协议版本和工具快照
实现超时和禁止重定向
增加单元测试
```

完成后必须停下来给用户 Review，不得继续实现第二步。

第一步建议验收条件：

```text
无认证 MCP Server 可以成功发现工具
自定义 Header 会正确发送且不会被记录到日志
连接失败返回可理解错误
超时可以终止连接
重定向会被拒绝
测试结束后会关闭 MCP 会话
没有新增 HTTP 接口
没有写入 mcp_server
```

### 7.2 第二步：实现测试并保存接口

仅在第一步 Review 通过后继续：

```text
增加 Thrift IDL
通过 Hertz 工具生成 API model、handler 和 route
增加 Application Service 编排
读取登录 session 中的 creator_id
校验 workspace 权限
调用第一步的 MCP 客户端
加密认证信息
通过 Repository/DAO 写入 mcp_server
返回 id、name、tool_count
增加接口和持久化测试
```

数据库写入规则：

```text
连接失败       → 不保存
协议协商失败   → 不保存
tools/list 失败 → 不保存
加密失败       → 不保存
数据库失败     → 返回保存失败
全部成功       → 创建一条 mcp_server
```

## 8. 当前明确不做

在用户另行讨论确认前，不得实现：

```text
MCP 列表查询
MCP 详情、编辑和删除
MCP 启用/禁用
资源库列表聚合和搜索
Agent 或 Workflow 绑定 MCP
Agent 运行时调用 MCP 工具
OAuth 动态客户端注册
OAuth 授权回调
工具列表定时刷新
健康检查定时任务
审计日志和监控面板
完整 SSRF 白名单系统
```

## 9. 已确认的 Coze 后端结构信息

接手模型不要凭空设计另一套框架，应遵循仓库已有结构：

```text
IDL                 idl/*.thrift
生成 API Model      backend/api/model/
HTTP Handler        backend/api/handler/coze/
生成 Router         backend/api/router/coze/
Application 编排    backend/application/
Domain Entity       backend/domain/*/entity/
Repository 接口     backend/domain/*/repository/
DAO                 backend/domain/*/internal/dal/
外部 MCP 客户端     backend/infra/
依赖初始化          backend/application/application.go
```

重要现状：

1. `backend/main.go` 全局安装了 `middleware.SessionAuthMW()`。
2. 当前登录用户可以通过 `backend/application/base/ctxutil` 获取。
3. 工作空间权限使用 `permission.DefaultSVC().CheckAuthz(...)` 检查。
4. API 路由和请求模型来自 Thrift/Hertz 生成，不能只手写路由而绕开 IDL。
5. 当前 `backend/go.mod` 中没有 MCP SDK。
6. 当前后端没有已经实现的 MCP Client、MCP Application Service 或 MCP Repository。

## 10. 本地开发与验证

本地开发环境使用：

```bash
make debug
```

主要地址：

```text
Coze Web/API: http://localhost:8888
MySQL:       127.0.0.1:3306
Redis:       127.0.0.1:6379
Milvus:      127.0.0.1:19530
MinIO API:   http://127.0.0.1:9000
MinIO UI:    http://127.0.0.1:9001
```

前端主应用并非使用 Vite 构建：

```text
Rush     → Monorepo 调度
Rsbuild  → 开发服务器和生产构建
Rspack   → 底层打包器
Vitest   → 单元测试
```

后端 Go 命令在 `backend/` 目录执行。Go 依赖由 `go.mod`、`go.sum` 管理，并下载到用户级 Module Cache，不会产生项目级 `node_modules`。

## 11. 相关文档

业务系统账号、历史会话和 Token 计费设计见：

```text
docs/business-system-integration-and-billing-design.md
```

其中已经确定：

```text
业务系统负责租户、账号、登录和权益
Coze 作为 Agent 执行引擎
业务后端持有 Coze PAT
external_user_id 用于业务用户会话隔离
chat_id 作为逐轮 Token 结算幂等键
第一版按轮后结算，不做 Token 预扣
```

该计费方案与当前 MCP 后端接口尚无直接代码耦合，不要在实现 MCP 客户端时顺手实现计费。

## 12. 新环境恢复工作建议

新模型或新机器开始工作时依次执行：

```text
1. 阅读本文。
2. 阅读 docs/business-system-integration-and-billing-design.md。
3. 检查当前分支必须是 feat/hz-agent。
4. 检查 main..HEAD 的两笔 MCP 提交。
5. 检查工作区，保护尚未提交的文档和用户改动。
6. 确认本地 make debug 可以启动。
7. 在修改代码前，再向用户复述“第一步 MCP 客户端适配器”的文件范围。
8. 只完成第一步并停下等待 Review。
```

不要把本文中的“下一步设计”误认为已经实现。当前真实完成状态仍然是：

```text
已完成：前端 MCP 配置交互、前端 API 契约、mcp_server 表结构
未完成：MCP Go SDK接入、连接测试客户端、后端接口、认证加密、数据库写入
```
