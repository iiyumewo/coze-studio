# Coze 对话用量与 OpenAPI 现状参考

> 文档性质：现状说明
>
> 适用范围：当前 Coze Studio 开源版、自部署后端和 Web Chat SDK
>
> 本文只记录当前源码中的数据结构、Token usage 来源和可用接口，不描述业务扣费实现方案。

## 1. 核心结论

1. Coze 的会话、消息和 Agent Run 记录主要存储在业务数据库中，不是 Elasticsearch。
2. `conversation` 表示一条多轮会话，侧边栏中的一条历史记录对应一个 `conversation_id`。
3. 一次用户提问或一次 `/v3/chat` 执行对应一个 Agent Run，外部接口中通常使用 `chat_id` 标识该次运行。
4. Token 用量主要记录在 `run_record.usage` JSON 字段中，按一次 Agent Run 保存。
5. 一个 Conversation 可以包含多个 RunRecord；会话累计用量需要汇总其下所有 RunRecord。
6. Conversation 删除采用软删除，删除后历史记录不再出现在正常会话列表中，但数据库记录不会立即物理删除。
7. `GET /v3/chat/retrieve` 可以根据 `conversation_id` 和 `chat_id` 查询单次运行的最终状态和 usage。
8. 当前源码没有发现面向业务系统的标准计费、余额或 Token 配额接口。

## 2. 标识定义

| 标识 | 含义 | 主要来源 |
| --- | --- | --- |
| `tenant_id` | 业务系统租户标识 | 业务系统 |
| `account_id` | 业务系统账号标识 | 业务系统 |
| `user_id` | Coze OpenAPI 接收的外部用户标识，用于对话隔离 | `/v3/chat` 请求体 |
| `creator_id` | Coze 内部创建者或 PAT 所属用户 ID | Coze 数据库 |
| `conversation_id` | 一条多轮会话的 ID | `conversation` 表 |
| `section_id` | Conversation 内的一段上下文 ID | `conversation.section_id`、消息记录 |
| `chat_id` / `run_id` | 一次 Agent Run 的 ID | `run_record.id`、消息 `chat_id` |
| `bot_id` / `agent_id` | 智能体 ID | 请求参数和数据库 |

推荐将业务用户映射为稳定的外部 `user_id`，例如：

```text
tenant_100:account_200
```

当前开源版请求中，`user_id` 主要作为会话数据的隔离字段；它不等同于 Coze 登录用户，也不能替代 API Token 鉴权。

## 3. Conversation 表

表名：

```text
opencoze.conversation
```

主要字段：

| 字段 | 类型 | 含义 |
| --- | --- | --- |
| `id` | `bigint` | Conversation ID |
| `connector_id` | `bigint` | 发布渠道 ID |
| `agent_id` | `bigint` | Bot/Agent ID |
| `scene` | `int` | 会话场景 |
| `section_id` | `bigint` | 当前 section ID |
| `creator_id` | `bigint` | Coze 内部创建者 ID |
| `ext` | `text` | 扩展信息 |
| `status` | `tinyint` | `1=正常，2=已删除` |
| `created_at` | `bigint` | 创建时间，毫秒时间戳 |
| `updated_at` | `bigint` | 更新时间，毫秒时间戳 |
| `name` | `varchar` | 会话名称 |
| `user_id` | `varchar` | 外部用户标识 |

源码：[`conversation.gen.go`](../backend/domain/conversation/conversation/internal/dal/model/conversation.gen.go)

删除行为：

- 删除接口将 `status` 更新为 `ConversationStatusDeleted`。
- 会话列表只查询 `status=ConversationStatusNormal` 的记录。
- 当前删除逻辑不会直接删除 `conversation` 行。
- Conversation 表本身不包含 Token usage 汇总字段。

## 4. Message 表

表名：

```text
opencoze.message
```

主要字段：

| 字段 | 类型 | 含义 |
| --- | --- | --- |
| `id` | `bigint` | 消息 ID |
| `run_id` | `bigint` | 所属 Agent Run ID |
| `conversation_id` | `bigint` | 所属 Conversation ID |
| `user_id` | `varchar` | 外部用户标识 |
| `agent_id` | `bigint` | Agent/Bot ID |
| `role` | `varchar` | `user`、`assistant` 等角色 |
| `content_type` | `varchar` | 消息内容类型 |
| `content` | `text` | 消息内容 |
| `message_type` | `varchar` | 消息类型 |
| `ext` | `text` | JSON 扩展信息 |
| `section_id` | `bigint` | 所属 section |
| `status` | `tinyint` | 消息状态 |
| `created_at` | `bigint` | 创建时间 |
| `updated_at` | `bigint` | 更新时间 |

源码：[`message.gen.go`](../backend/domain/conversation/message/internal/dal/model/message.gen.go)

回答消息的 `ext` 可能包含以下用量字段：

```json
{
  "input_tokens": "100",
  "output_tokens": "50",
  "token": "150"
}
```

这些字段来源于模型响应的 usage，适合消息展示和调试，不应单独作为最终账务来源。

相关字段定义：[`message_ext.go`](../backend/domain/conversation/message/entity/message_ext.go)

## 5. RunRecord 表

表名：

```text
opencoze.run_record
```

该表记录一次 Agent Run 的执行状态，是当前源码中最接近单次 Token usage 事实记录的表。

主要字段：

| 字段 | 类型 | 含义 |
| --- | --- | --- |
| `id` | `bigint` | Run ID，对外通常对应 `chat_id` |
| `conversation_id` | `bigint` | 所属 Conversation |
| `section_id` | `bigint` | 所属 section |
| `agent_id` | `bigint` | Agent/Bot ID |
| `user_id` | `varchar` | 外部用户标识 |
| `source` | `tinyint` | 执行来源 |
| `status` | `varchar` | Run 状态 |
| `creator_id` | `bigint` | Coze 内部创建者 ID |
| `created_at` | `bigint` | 创建时间 |
| `updated_at` | `bigint` | 更新时间 |
| `failed_at` | `bigint` | 失败时间 |
| `completed_at` | `bigint` | 完成时间 |
| `last_error` | `text` | 最后错误 |
| `chat_request` | `text` | 原始请求 |
| `ext` | `text` | 扩展信息 |
| `usage` | `json` | Token usage JSON |

源码：[`run_record.gen.go`](../backend/domain/conversation/agentrun/internal/dal/model/run_record.gen.go)

数据库结构：[`opencoze_latest_schema.hcl`](../docker/atlas/opencoze_latest_schema.hcl#L3093)

当前 Run 状态包括：

```text
Created
InProgress
Completed
Failed
Expired
Cancelled
RequiresAction
Deleted
```

## 6. Usage 结构

Coze 跨模块使用的 Run usage 结构为：

```json
{
  "llm_prompt_tokens": 100,
  "llm_completion_tokens": 50,
  "llm_total_tokens": 150,
  "workflow_tokens": null,
  "workflow_cost": null
}
```

字段定义：[`agent_run.go`](../backend/crossdomain/agentrun/model/agent_run.go#L47)

字段含义：

| 字段 | 含义 |
| --- | --- |
| `llm_prompt_tokens` | 输入 Token |
| `llm_completion_tokens` | 输出 Token |
| `llm_total_tokens` | 输入和输出的总 Token |
| `workflow_tokens` | 可选的 Workflow Token 信息 |
| `workflow_cost` | 可选的 Workflow 成本信息 |

OpenAPI 对外返回时会映射为：

| Run usage | OpenAPI 字段 |
| --- | --- |
| `llm_prompt_tokens` | `usage.input_count` |
| `llm_completion_tokens` | `usage.output_count` |
| `llm_total_tokens` | `usage.token_count` |

API 定义：[`run.thrift`](../idl/conversation/run.thrift#L197)

## 7. Usage 产生和持久化过程

```text
大模型返回 ResponseMeta.Usage
    ↓
handlerUsage 提取 prompt/completion/total
    ↓
AgentRuntime 保存 Usage
    ↓
Run 完成时 StepToComplete
    ↓
RunRecordRepo.UpdateByID
    ↓
run_record.usage JSON 持久化
```

模型 usage 提取逻辑：[`message_builder.go`](../backend/domain/conversation/agentrun/internal/message_builder.go#L180)

回答消息扩展字段写入：[`message_event.go`](../backend/domain/conversation/agentrun/internal/message_event.go#L215)

Run 完成和 usage 更新：[`run_process_event.go`](../backend/domain/conversation/agentrun/internal/run_process_event.go#L65)

对于 Workflow，源码中存在 `TokenCollector`，会收集多个模型调用的输入和输出 Token：

[`collect_token.go`](../backend/domain/workflow/internal/execute/collect_token.go#L32)

## 8. 可用 OpenAPI

### 8.1 创建 Conversation

```http
POST /v1/conversation/create
```

主要字段：

```json
{
  "bot_id": "7676009291924373504",
  "connector_id": "999",
  "user_id": "tenant_100:account_200",
  "messages": []
}
```

返回新建的 `conversation_id` 和 `last_section_id`。

### 8.2 获取 Conversation 列表

```http
GET /v1/conversations
```

主要查询参数：

```text
page_num
page_size
sort_order
sort_field
bot_id
connector_id
user_id
```

当前列表默认只返回正常状态的 Conversation。

### 8.3 查询 Conversation 详情

```http
GET /v1/conversation/retrieve?conversation_id={conversation_id}
```

返回 Conversation 基本信息，包括 `id`、`last_section_id`、`name`、`user_id` 等。

### 8.4 查询 Conversation 消息

```http
POST /v1/conversation/message/list
```

主要字段：

```json
{
  "conversation_id": "xxx",
  "section_id": "xxx",
  "limit": 50,
  "before_id": "xxx",
  "after_id": "xxx"
}
```

返回消息列表。消息的 `extra_info` 可能包含 `input_tokens`、`output_tokens` 和 `token`。

### 8.5 发起 Chat Run

```http
POST /v3/chat
```

主要请求字段：

```json
{
  "bot_id": "7676009291924373504",
  "conversation_id": "xxx",
  "user_id": "tenant_100:account_200",
  "stream": true,
  "connector_id": "999",
  "additional_messages": []
}
```

字段说明：

| 字段 | 说明 |
| --- | --- |
| `bot_id` | 必填，Agent/Bot ID |
| `conversation_id` | 可选，不传时可能创建新 Conversation |
| `user_id` | 必填，外部用户标识 |
| `stream` | 是否使用流式响应 |
| `connector_id` | 发布渠道，Web SDK 当前常用 `999` |
| `additional_messages` | 本轮附加消息 |
| `custom_variables` | 自定义变量 |
| `extra_params` | 下游插件或 Workflow 参数 |

接口定义：[`run.thrift`](../idl/conversation/run.thrift#L149)

### 8.6 查询单次 Run 的最终结果和 usage

```http
GET /v3/chat/retrieve?conversation_id={conversation_id}&chat_id={chat_id}
```

返回：

```json
{
  "code": 0,
  "data": {
    "id": "chat_id",
    "conversation_id": "conversation_id",
    "bot_id": "bot_id",
    "status": "completed",
    "usage": {
      "token_count": 150,
      "input_count": 100,
      "output_count": 50
    }
  }
}
```

这是当前公开接口中查询单次最终 usage 的主要入口。

### 8.7 清空当前上下文

```http
POST /v1/conversations/{conversation_id}/clear
```

Conversation ID 不变，创建或切换到新的 section。

### 8.8 重命名 Conversation

```http
PUT /v1/conversations/{conversation_id}
```

请求体：

```json
{
  "name": "新的会话名称"
}
```

### 8.9 删除 Conversation

```http
DELETE /v1/conversations/{conversation_id}
```

当前实现是软删除，不应将该接口理解为物理清除全部消息和 RunRecord。

## 9. 流式响应中的 usage

`POST /v3/chat` 在流式模式下通过 SSE 返回事件，当前主要事件包括：

```text
conversation.chat.created
conversation.chat.in_progress
conversation.message.delta
conversation.message.completed
conversation.chat.completed
conversation.chat.failed
error
done
```

回答消息完成事件的 `meta_data` 可能包含：

```text
input_tokens
output_tokens
token
```

当前源码中，Run 完成事件的 usage 传递路径不如 `/v3/chat/retrieve` 明确；因此流式响应更适合实时展示和获取 `chat_id`，最终 usage 应以 RunRecord 或 retrieve 接口为准。

## 10. Workflow 相关补充表

对于 Workflow/App 场景，数据库中还存在执行级别和节点级别的 Token 字段。

### `workflow_execution`

记录一次 Workflow 执行，包含：

```text
id
workflow_id
space_id
operator_id
connector_id
status
created_at
updated_at
input_tokens
output_tokens
```

### `node_execution`

记录 Workflow 每个节点的执行，包含：

```text
id
execute_id
node_id
node_type
status
input_tokens
output_tokens
```

表结构位置：[`opencoze_latest_schema.hcl`](../docker/atlas/opencoze_latest_schema.hcl#L2427)

这些字段主要用于 Workflow 执行记录和节点详情，不应与 Bot `/v3/chat` 的 `run_record.usage` 简单混为同一张表。

## 11. 当前边界和注意事项

1. Elasticsearch 不是当前对话 Token usage 的主要事实存储。
2. Conversation 列表删除只改变可见性，不代表相关 RunRecord 已删除。
3. `run_record.usage` 是 JSON 字段，当前没有单独的总 Token 数值列。
4. 成功完成的 Run 会明确写入 usage。
5. 失败、取消或异常中断的 Run 是否保留完整的已消耗 usage，需要结合具体执行路径验证。
6. 单次 Agent Run 内包含多次模型调用时，需要确认最终 usage 是否为累计值。
7. Prompt Token 可能包含历史消息、系统提示词、工具定义和检索内容，不等于当前用户输入文本的 Token 数。
8. MCP、embedding、rerank、图片生成等资源的消耗不应默认视为已包含在 `llm_total_tokens` 中。
9. `user_id` 是外部数据隔离标识，不是 Coze API Token，也不能单独作为安全认证边界。
10. 当前源码没有发现标准的业务余额、Token 配额或第三方计费回调接口。

## 12. 源码索引

- Conversation 模型：[`conversation.gen.go`](../backend/domain/conversation/conversation/internal/dal/model/conversation.gen.go)
- Message 模型：[`message.gen.go`](../backend/domain/conversation/message/internal/dal/model/message.gen.go)
- RunRecord 模型：[`run_record.gen.go`](../backend/domain/conversation/agentrun/internal/dal/model/run_record.gen.go)
- Run usage 类型：[`agent_run.go`](../backend/crossdomain/agentrun/model/agent_run.go)
- Token 提取：[`message_builder.go`](../backend/domain/conversation/agentrun/internal/message_builder.go)
- Run 完成持久化：[`run_process_event.go`](../backend/domain/conversation/agentrun/internal/run_process_event.go)
- OpenAPI Chat：[`agent_run_service.go`](../backend/api/handler/coze/agent_run_service.go)
- OpenAPI Run 查询：[`openapi_agent_run.go`](../backend/application/conversation/openapi_agent_run.go)
- Conversation OpenAPI：[`conversation_service.thrift`](../idl/conversation/conversation_service.thrift)
- Run OpenAPI：[`run.thrift`](../idl/conversation/run.thrift)
