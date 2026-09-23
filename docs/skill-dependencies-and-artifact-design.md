# Coze Skill 依赖管理与产物存储设计

> 状态：依赖预装 Phase 1 已实现并完成本地 Docker 与前端验证；产物管理主干已确认，仍有 B3-B5 的实现细节待处理。
>
> 记录日期：2026-09-17（初版）／2026-09-18（第 4 节重写）／2026-09-18（复审修正）／2026-09-18（依赖方案定稿）／2026-09-20（Phase 1 实现与验证）
>
> 关联文档：本文是 `docs/skills-integration-roadmap.md` 中 V3（受控执行隔离）、V4（分布式工作区）、V5（生产安全治理）的详细设计补充，不重复路线文档已有的阶段划分。
>
> 本文档为持续更新文档，讨论有新结论时直接修订对应章节，并在末尾「变更记录」追加一行。

## 1. 背景与两个待解决问题

当前 Skill 脚本执行链路存在两个独立但相关的问题：

**问题 A：Skill 脚本无法使用第三方依赖**

执行容器断网且只读，脚本一旦 `import requests` 就报 `ModuleNotFoundError`。现有测试 Skill 的 `SKILL.md` 中已明确记录了这一预期失败。

**问题 B：Skill 产物没有管理机制**

脚本写出的文件留在宿主的会话工作目录里，既没有登记、没有对外引用方式，也永不清理。模型无法在后续轮次可靠引用这些文件，前端也无法下载。

两个问题的共同根源是：Skill 执行环境是按「一次性、无状态、纯计算」设计的，而实际需求已经超出这个假设。

## 2. 现状代码事实

以下均为已核实的代码事实，是本文所有推理的基础。

### 2.1 执行容器约束栈

`backend/infra/scriptrunner/impl/docker/runner.go` 的 `dockerArgs()` 生成的启动参数：

| 约束 | 值 | 作用 |
| --- | --- | --- |
| `--network` | `none` | 完全断网 |
| `--read-only` | 开启 | 根文件系统只读 |
| `--tmpfs /tmp` | `rw,noexec,nosuid,nodev,size=64m` | 唯一可写区，禁执行 |
| `--cap-drop` | `ALL` | 丢弃全部 capability |
| `--security-opt` | `no-new-privileges` | 禁止提权 |
| `--user` | 非 root（宿主 uid/gid 或 65532） | 非特权身份 |
| `--pids-limit` | 64 | 防 fork 炸弹 |
| `--memory` | 256m | 内存上限 |
| `--cpus` | 0.5 | CPU 上限 |
| `--pull` | `never` | 镜像必须已在宿主本地 |

镜像默认 `python:3.12-slim-bookworm`，由 `COZE_SKILL_RUNNER_DOCKER_IMAGE` 配置（`impl/impl.go:45`、`types/consts/consts.go:100`）。

挂载只有两个：

- Skill 目录 → `/opt/skill`，**只读**
- 会话工作目录 → `/workspace`，**可读写**

**结论：隔离基线本身做得很好，问题在于四个约束叠加导致依赖无解。**

### 2.2 工作目录机制

`runner.go:116`：

```go
workingDir := filepath.Join(r.config.WorkspaceRoot, strconv.FormatInt(request.ConversationID, 10))
```

关键事实：

- 路径**只由 conversationID 决定，与 skillID 无关** → 同一会话内所有 Skill 共享同一个可写目录。这是有意设计，用于支持多 Skill 协作，不是缺陷
- `runner.go:251` 的清理逻辑只执行 `docker rm -f`，**工作目录本身永不删除**
- 没有任何容量限制：`--memory` 限的是内存，`--tmpfs size=64m` 只限 `/tmp`，对 `/workspace` 无约束
- Docker 的 `--storage-opt size=` 对 bind mount 无效，无法用启动参数解决
- `runner.go:117-126` 建目录后，**仅当 backend 以 root 运行时**才把属主 chown 给沙箱用户（非 root 场景为 65532）；backend 非 root 时该分支跳过，容器内用户对 0750 的宿主目录写不进去（见 T28）

生产部署下（`docker/docker-compose.prod-hz.yml:10`）该目录为宿主 `/data/hz-agent/coze/runtime/conversations/<会话ID>`。

**以上四条现状将被本设计改变**（见 D9、D10、D22）：工作目录改为按 run 创建与销毁、路径改为按 run 隔离、增加单次 run 配额、`HOME` 改指 `.home/` 且 `.tmp/` 按 run 清理。本节记录的是**改造前**的事实基线。

### 2.3 工具注册与模型可见性

`backend/domain/agent/singleagent/internal/agentflow/agent_flow_builder.go`：

- `:114-165` 六类工具分别构造：plugin、MCP、**skill**、workflow、database、agent variable
- `:173-190` 全部转为 Eino 的 `tool.BaseTool` 统一接口，追加进 `agentTools`
- `:192` `validateUniqueToolNames` 校验重名
- `:210-213` 交给 `react.NewAgent`，之后由 Eino 自动序列化为 OpenAI function calling 的 `tools` 字段

Skill 工具在 `backend/domain/agent/singleagent/internal/agentflow/node_tool_skill.go:38` 的 `newSkillTools()` 中构造，产出 `skill_read` 与 `script_run` 两个工具，通过 `utils.InferTool(名称, 描述, 实现函数)` 反射生成 JSON Schema。

**新增工具只需三步：定义入参 struct、实现 `Invoke`、在 `newSkillTools` 中多调一次 `InferTool`。无需改动任何模型调用逻辑。**

编码约定（从现有代码归纳）：

- 工具描述就是提示词，用于引导模型行为顺序
- 返回值用扁平键值对纯文本，不用嵌套 JSON
- 失败时返回错误描述字符串 + `nil` error，不要返回真 error（否则会中断整个 agent 流程）

**一处现存描述与本设计直接冲突**：`node_tool_skill.go:99` 的 `script_run` 描述结尾写着

> The script runs in the current conversation's working directory, which can be reused by later calls.

D9 落地后工作目录按 run 销毁，`conversation's` 与 `reused by later calls` 两处措辞都会**主动引导模型以为跨 run 持久**，正好是 4.11 那个静默丢历史场景的放大器。改造时必须同步重写（T27），不能只改代码不改描述。

### 2.4 历史消息与工具返回值的跨 run 重放

已完整验证的链路：

1. `message_builder.go:437` `historyPairs()` 明确处理 `MessageTypeFunctionCall` 与 `MessageTypeToolResponse`，两者**均被持久化**
2. `run.go:200` `getHistory()` 按 `Limit: conversationTurns` 取最近 N 个 run 的全部消息
3. `message_builder.go:465` `transMessageToSchemaMessage()` 从每条消息的 `ModelContent` 字段 JSON 反序列化，**原文还原**；仅跳过 `MessageTypeVerbose` 与 `MessageTypeFlowUp`（这两类是给用户看的展示消息）
4. `agent_flow_runner.go:275` `preHandlerHistory()` **不过滤工具消息**，只对 User 消息做预处理

**结论：工具返回值是对话历史的一部分，跨 run 原样重发。这是「产物信息通过工具返回值传递」这一设计成立的根本依据。**

两个失效边界：

- **历史轮数限制**：`agent_info.go:29` 读取 agent 配置的「短期记忆策略 → 历史轮数」，默认 100 轮（`entity/const.go:19`）。实际部署常调小以控制 token 成本，调小后早期产物信息即从模型视野消失
- **落单调用被丢弃**：`message_builder.go:452-460` 中，若某 run 的 `FunctionCall` 消息数为奇数（调用未配对响应，例如超时或取消），该落单调用会被丢弃。因为向模型 API 发送无响应的 `tool_calls` 会直接报错

### 2.5 消息结构已原生支持文件

- `backend/api/model/conversation/run/run.go:33` 定义 `ContentTypeFile = "file"`
- `run.go:501` 消息内容结构含 `File *File` 字段
- `run.go:1012` 的 `File` 结构为 `FileKey` / `FileName` / `FileType` / `FileSize` / `FileURL`

**即：Coze 在「用户上传文件给 agent」这条链路上已经采用不透明 key + 预签名 URL 的设计。缺失的只是反方向——Skill 产物如何进入这套体系。**

### 2.6 现有文件表不适合承载 Skill 产物

`files` 表（`backend/domain/upload/entity/file.go:19`）的实际用途经调用链核实为四类：

1. OpenAPI 文件上传（`bot_open_api_service.go:86` → `UploadFileOpen`），`Source` 字段唯一枚举值 `FileSourceAPI = 1` 印证了这是它最初的设计目的
2. 各类实体图标（`knowledge_service.go:504` → `GetIconForDataset`）
3. 前端图片上传（`/api/common/upload/apply_upload_action`，两段式）
4. 大文件分片上传（上限 200MB，`application/upload/consts.go`）

表结构：`id / name / file_size / tos_uri / status / comment / source / creator_id(有索引) / content_type / coze_account_id / created_at / updated_at / deleted_at(软删除)`。

另：`UploadSessionKey` 与 `GetObjInfoBySessionKey` 两个函数**全仓无调用者**，是死代码或预留能力，不可依赖。

**结论：不复用此表。理由是生命周期完全不同——图标与知识库文档是永久资产，误删是生产事故；Skill 产物是临时产物，需 TTL 与批量清理，量级可能高出数百倍。混表会让清理任务依赖单个 `WHERE source = ?` 条件保命，风险不可接受。内容存储与预签名逻辑复用 `backend/infra/storage`，一行不重写。**

### 2.7 现有 Skill 的实际行为

对 `bin/runtime/skills/7676006817838661632/` 下 5 个 Skill 的核查结果：仅 2 个写文件，且都与「平台强加目录约定」的方案冲突。

**`build_report.py`**：

```python
if pathlib.Path(filename).name != filename:
    raise ValueError("filename must not contain a directory")   # 显式拒绝带目录的路径
path = pathlib.Path.cwd() / filename
path.write_text(content, encoding="utf-8")
print(json.dumps({"filename": filename, "content": content}))    # 内容同时输出到 stdout
```

**`notes.py`**：

```python
path = pathlib.Path.cwd() / "notes.txt"
with path.open("a", encoding="utf-8") as file:    # 追加模式
    file.write(params["note"] + "\n")
notes = path.read_text(encoding="utf-8").splitlines()   # 读回全部历史
```

`notes.py` 的核心功能是**跨多次调用累积状态**，完全依赖工作目录持久存在。任何「按 run 清理工作目录」的方案都会静默破坏它——不报错，只是历史记录消失。

另：改造前 `requirements.txt` 在 Go 代码中**一处都未被引用**，只是 `SKILL.md` 中的文档约定；第一版改造将改为读取管理员维护的全局 requirements 文件。

### 2.8 权限模型

`backend/application/skill/authorization.go:28` 的 `authorizeWorkspaceAccess()` 仅校验**空间成员身份**，不校验角色。即空间内任何成员都能创建 Skill。若未来向业务方或租户开放 Skill 上传，此处是比沙箱更优先需要收紧的环节。

### 2.9 runID 在 Agent 执行链上不可达（直接影响 T3）

按 run 隔离工作目录要求执行端知道 runID，但**执行链上五层结构体全都没有这个字段**，已逐一核实：

| 层 | 结构体 | 现有标识字段 |
| --- | --- | --- |
| 跨域编排 | `crossdomain/agent/contract.go:39` `AgentRuntime` | `ConversationId` |
| 跨域模型 | `crossdomain/agent/model/single_agent.go:109` `ExecuteRequest` | `ConversationID` |
| 流程构建 | `agentflow/agent_flow_builder.go:37` `Config` | `ConversationID` |
| 工具持有 | `node_tool_skill.go:32` `skillToolConfig` | `conversationID` |
| 沙箱入口 | `infra/scriptrunner/script.go:35` `RunRequest` | `ConversationID` |

runID 只存在于**更上层的 agentrun 域**：消息体带 `RunID`（`message_event.go` 一带），`run.go:170-181` 的收尾 `defer` 处 `art` 持有完整 RunMeta。

**由此产生一个必须写进设计的不对称**：

```
收尾端（run.go 的 defer）→ 有 runID + conversationID   ✅ 产物收集/上传没问题
执行端（script_run 工具内）→ 只有 conversationID        ❌ 拼不出按 run 隔离的路径
```

两端必须指向同一个目录，所以 **T3 不是「纯路径设计」，而是一次穿透 5 层的字段透传**，工作量与回归面都比原先描述的大。

**必须避开的捷径**：在 `BuildAgent` 内生成随机 UUID 当作「本次执行 ID」——零透传、看着干净，但是错的。中断恢复（`ResumeInfo`）会重新走一遍 `StreamExecute` → `BuildAgent`，UUID 随之改变，恢复后拿到的是全新空目录，中断前产出的文件全部消失且不报错。要让 ID 在同一 run 内稳定，只能透传真实 runID。

## 3. 问题 A：依赖管理方案

### 3.1 生态调研结论（2026-09）

| 方案 | 许可 | 隔离机制 | 结论 |
| --- | --- | --- | --- |
| Daytona | **2026-06 起闭源**，核心开发转入私有仓库 | gVisor | 排除 |
| E2B | infra Apache-2.0，**microVM runtime 闭源** | Firecracker | 自托管需 Nomad + KVM，过重，排除 |
| microsandbox | Apache-2.0，8.3k★，Rust | libkrun microVM | 需 KVM/嵌套虚拟化且仍标 experimental，排除 |
| Kata Containers | Apache-2.0 | 真 VM | 需 `/dev/kvm`，阿里云 ECS 普通实例大概率不支持 |
| **gVisor** | **Apache-2.0，Google 维护** | **用户态内核** | **不需要 KVM，Docker 加 `--runtime=runsc` 即可，唯一现实可用** |
| DifySandbox | Dify 修改版 Apache（含多租户条款） | chroot + seccomp | 隔离弱于现状，仅借鉴其依赖管道 |

**待确认**：阿里云 ECS 上 `ls /dev/kvm` 是否存在。若存在，Kata / Firecracker / microsandbox 重新进入候选。

### 3.2 为什么不复用 Coze 自带的 Pyodide 沙箱

Coze 另有一套 `infra/coderunner` 走 `deno run jsr:@langchain/pyodide-sandbox@0.0.4`，通过 `find_imports` 静态扫描 import 再用 `micropip.install` 装包。对 Skill 场景有五个硬伤：

1. 无脚本文件概念，Skill 的多文件结构（如 `build_report.py` 依赖 `report_format.py`）无法承载
2. 工作区不留存
3. 无子进程能力
4. 网络受限：`loadPyodide` 未传 `indexURL`，默认走 jsdelivr（这正是 `CODE_RUNNER_ALLOW_NET=cdn.jsdelivr.net` 的由来）；任意 PyPI 包需 `pypi.org` + `files.pythonhosted.org`，被 Deno 的 allow-net 堵死
5. 默认内存仅 100MB（`base.go:73`）

**决策：不改动此方案，避免影响其余 Coze 逻辑。**

附带发现的隐患：`backend/Dockerfile:38` 安装 deno，`:40` 构建期预热，但**未锁定版本**，与代码中写死的 `@0.0.4` 可能不一致。

### 3.3 选定方案：全局依赖预装 + 启动时构建 + 执行期只读挂载

第一版不按 Skill 动态解析依赖，也不在每次脚本执行时安装。由管理员维护一份全局 `python-requirements.txt`，Coze 后端启动时校验并在需要时启动一次性 builder 容器：

```
管理员维护 python-requirements.txt
    ↓
Coze 后端启动
    → 计算 requirements 内容 + 运行镜像 digest 的 fingerprint
    → fingerprint 未变化：复用当前依赖目录
    → fingerprint 变化：启动 deps-builder 容器
        → 只访问配置的 pip 镜像
        → pip install --target <staging>
        → 写 manifest 和 .complete
        → 原子切换为 current
    ↓
一次性执行容器（network=none、根只读）
    → /opt/skill：Skill 代码，只读
    → /opt/skill-deps：全局依赖目录，只读
    → PYTHONPATH=/opt/skill-deps
```

依赖构建是部署/启动阶段的动作，不是脚本调用动作。修改清单后重启后端即可触发重建；没有变更则跳过。builder 不得直接修改正在使用的目录，必须先写 staging，校验成功后再切换版本。

fingerprint 使用两层 SHA-256：先计算 requirements 文件原始字节的 SHA-256，再计算 `SHA256(requirementsHash + NUL + Docker image ID + NUL + pip mirror URL)`，最终以 64 位十六进制字符串作为 `versions/<fingerprint>` 目录名。Docker image ID 来自 `docker image inspect`，因此同一 tag 指向新镜像时也会触发重建；requirements 中空格、注释或顺序变化同样会触发重建。

`versions/` 是原子发布与回滚缓冲区，不是无限历史。每次成功激活 current 后执行 best-effort GC，默认总共保留 2 个完整版本：current 和最新一个旧版本；残留的 `.staging-*` 与不完整版本也会清理。保留数量可通过 `COZE_SKILL_RUNNER_DEPENDENCY_VERSIONS_TO_KEEP` 调整，最小为 1。

**第一版依赖边界**：

- 只有管理员可以修改全局清单；Skill 作者不能通过上传包追加依赖
- 支持普通 pip requirements；版本号可选。不写版本号时，首次构建解析当时可用的最新兼容版本
- 首次解析结果写入内部 manifest，后续重启复用该结果；需要升级时修改清单或显式重建
- 所有 Skill 共享一套当前依赖环境，同一个包在当前环境中只允许一个生效版本
- 依赖不上传 MinIO，也不安装到宿主机系统 Python；只写入受控 runtime 目录或 Docker volume
- builder 与执行容器必须使用同一个镜像 digest，保证 Python、glibc、CPU 架构一致

第一版配置项：`COZE_SKILL_RUNNER_REQUIREMENTS`（默认 `<backend working directory>/resources/skill-runtime/python-requirements.txt`）、`COZE_SKILL_RUNNER_DEPENDENCY_ROOT`（默认 `<backend working directory>/runtime/skill-deps`）、`COZE_SKILL_RUNNER_PIP_MIRROR`（默认清华 PyPI 源 `https://pypi.tuna.tsinghua.edu.cn/simple`）、`COZE_SKILL_RUNNER_BUILD_NETWORK` 和 `COZE_SKILL_RUNNER_DEPENDENCY_VERSIONS_TO_KEEP`（默认 `2`，保留 current + 一个旧版本）。后端启动时 requirements 文件不存在或为空则跳过构建。`BUILD_NETWORK` 默认仅为本地开发便利，生产环境必须配置只允许访问 pip 镜像的专用网络。

**明确不做**：不根据 Skill 中的 `import` 自动推导依赖；不为每个 Skill 维护独立环境；不在执行容器中联网安装。

**仓库布局与实际启动行为（Phase 1 已落地）**：

- 全局清单纳入仓库的 `backend/skill-runtime/python-requirements.txt`，当前示例仅包含 `requests==2.32.4`。Skill 包内的 `requirements.txt` 不读取、不合并，也不能由 Skill 作者追加依赖。
- `backend/conf` 是会被生产环境整体覆盖的动态资源目录，requirements 不放在其中。`make debug` 会把 `backend/skill-runtime` 复制到 `bin/resources/skill-runtime`；生产镜像将其放在 `/app/resources/skill-runtime/python-requirements.txt`，不受 `/app/resources/conf` 挂载影响。
- 后端在 `application.Init` 的基础设施初始化之前执行依赖准备。Docker Runner 才执行 builder；选择 `local` Runner 时跳过构建，但本地执行仍会尝试把 `current` 目录加入 `PYTHONPATH`。
- 默认源固定为清华 PyPI 源；只有显式设置 `COZE_SKILL_RUNNER_PIP_MIRROR` 才覆盖它，不继承宿主进程的 `PIP_INDEX_URL`。因此启动环境中的其他全局 pip 配置不会悄悄改变依赖来源。
- 当前默认 `COZE_SKILL_RUNNER_BUILD_NETWORK=bridge` 只服务本地开发便利。生产部署必须改成仅允许访问受控 pip 镜像的网络；清华公共源适合作为默认下载源，不等同于生产包白名单或私有镜像治理。
- manifest 会记录 fingerprint、requirements hash、运行镜像 ID、pip mirror 和构建时间；`current` 是指向完整版本目录的相对符号链接，执行容器只读挂载 `/opt/skill-deps` 并通过 `PYTHONPATH` 使用它。

**已完成验证**：针对性 Go 单元测试、Docker 集成测试和 `go vet` 已通过。Docker 集成测试实际用 builder 构建 `requests==2.32.4`，再用执行容器导入并运行脚本；本地 `8888` 前端登录后，通过已绑定的 `dependency-e2e` Skill 执行 `check.py`，返回 `requests` 版本 `2.32.4`、退出码 `0` 和正确 JSON 输出。完整 `go test ./...` 仍受仓库已有的 mockey/Go runtime 链接错误及一个无关 JSON 数值类型测试失败影响，未作为本次改造的通过标准。

### 3.4 安全模型：四道防线

**必须理解的核心事实：`pip install` 会执行代码。** 安装 sdist 包时 pip 会运行其 `setup.py`，即任意代码执行。因此「安装」这个动作本身的威胁高于「运行用户脚本」。

| 阶段 | 威胁 | 防线 | 隔离是否有效 |
| --- | --- | --- | --- |
| 安装时 | `setup.py` 执行任意代码打穿宿主 | 构建容器隔离 | 有效 |
| 安装时 | 装入本身恶意的包 | **内网镜像 + 包白名单** | **无效，只能靠管控** |
| 运行时 | 脚本本身作恶 | 执行容器（现状已强） | 有效 |
| 运行时 | 恶意库窃取数据外带 | **只有白名单能防** | **无效** |

**关于「断网即安全」的常见误解**：断网防的是「往外连」，防不住数据外带。恶意库可以读取 `/opt/skill`（Skill 全部代码与配置，常含硬编码密钥）和 `/workspace`（本会话前几轮产物），再将数据编码进 **stdout**——而 stdout 会被 backend 收集、进入模型上下文、最终显示到前端。**它不需要网络。** 1MB 输出上限不构成障碍，base64 后可承载大量数据，且可分多次调用带出。

因此：**审核必须发生在安装之前**。一旦装入全局依赖目录，运行期会无条件信任它。

**构建容器的管控要求**：

- requirements 内容由后端从镜像内读取后通过 stdin 传给 builder；输出只写入本次 staging 目录，不挂载整个 runtime 根目录
- 网络不可只配 pip 镜像参数了事，须显式封禁云 metadata 地址与其余全部内网地址，仅放行镜像站
- CPU / 内存 / 磁盘配额，防止单个包撑爆磁盘
- 非 root、超时、用完销毁；构建容器不挂载 Docker Socket
- 安装前执行包名与版本白名单校验；未固定版本时记录 resolver 结果，避免每次重启无声漂移

**为什么必须用容器而不能在宿主直接装**：Coze backend 进程持有宿主 Docker 的完全控制权（`docker-compose.prod-hz.yml:8` 挂载 `/var/run/docker.sock`）。在宿主直接 `pip install`，等于让陌生人的 `setup.py` 以该身份执行——它可以直接调用 docker 起一个特权容器挂载宿主根目录，**取得宿主 root，全程无需任何逃逸**。

另两个使用容器的理由（与安全无关但同样重要）：

- **环境一致性**：保证构建与运行的 Python / glibc / 架构完全一致（见 3.3 硬约束）
- **不污染宿主**：builder 销毁后只留下受控依赖目录，不修改宿主系统 Python

### 3.5 运行时暴露面说明

去掉 `--network none` 后容器落到默认 bridge（`172.17.0.x`，NAT 出宿主），将可访问宿主所在 VPC 的一切，包括：

- 内网 MySQL、Redis、MinIO、Coze backend 端口
- **阿里云 ECS metadata 服务 `100.100.100.200`** —— `/latest/meta-data/ram/security-credentials/<RoleName>` 返回该实例 RAM 角色的 STS 临时凭证，取得后可直接操作授权给这台 ECS 的全部云资源，**无需逃逸容器**。Capital One 2019 年数据泄露即此路径
- 若 dockerd 监听 TCP 2375/2376，可经 bridge 网关 `172.17.0.1` 访问，等于直接取得宿主 root

**因此本方案坚持运行期保持 `--network none` 不变。**

### 3.6 gVisor 说明与接入

**定位**：Google 开源（Apache-2.0）的用户态 Linux 内核，用 Go 重写了 Linux 系统调用层。不是「更强的容器」，而是在容器与真内核之间插入一层假内核。

**三个组件**：

- **Sentry**：用户态内核，自行实现 syscall 接口、内存管理、文件系统与 TCP/IP 栈（netstack）。脚本以为在与 Linux 通信，实际在与一个 Go 程序通信
- **Gofer**：文件代理。Sentry 无权直接访问宿主磁盘，所有文件操作经 9P 协议委托给独立的受限 Gofer 进程
- **runsc**：OCI 兼容运行时，**runc 的直接替代品**，因此接入成本极低

**双层防御**：攻击面从「整个 Linux 内核（约 350 syscall，C 语言）」收敛为「Sentry（约 250 syscall，Go，内存安全）」；Sentry 自身再被 seccomp 限制为只能向宿主内核发约 70 个白名单 syscall。打穿需要先找到 Sentry 的洞，再找到一个受限 syscall 的宿主内核洞。

**代价**：syscall/IO 密集慢 10-30%，网络吞吐降 20-40%（netstack 为用户态实现），少数 ioctl 与 `/proc`、`/sys` 行为不支持，仅运行于 Linux。

**为什么适合本场景**：gVisor 最大的短板是网络性能，而 Skill 执行容器**本来就断网**，恰好完全避开。剩余开销仅为冷启动多约 100ms，而 Skill 是 60 秒超时的短任务、输出为文本、工作区文件很小，实际影响可忽略。

**生产验证**：Google 的 App Engine 标准环境、Cloud Run、Cloud Functions、Gemini Code Assist 代码沙箱均运行于 gVisor，是目前公开可查规模最大的不可信代码沙箱部署。

**接入方式**：`dockerArgs()` 中增加可配置的 `--runtime` 参数，新增环境变量 `COZE_SKILL_RUNNER_DOCKER_RUNTIME`。留空即现状，填 `runsc` 即切换，可随时回滚。

**大陆部署障碍**：gVisor 发布包与 apt 源均托管在 `storage.googleapis.com`，阿里云 ECS 大陆节点无法直接拉取。需经代理下载后转存内网 OSS 或自建 apt 源，或用 Go 从源码编译。

**本地开发环境**：Mac 上 Docker Desktop 在 Linux VM 内运行容器，安装 runsc 需进入该 VM，较为别扭。建议本地保持 runc、仅生产启用 runsc；因 runtime 为配置项，两侧行为一致，不影响调试。

### 3.7 Dify 方案参考与取舍

Dify Sandbox 的依赖管道是**全局依赖环境**，不是按 Skill 分别安装：

1. **镜像构建期**：`production.dockerfile` 预装少量基础包；`dependencies/python-requirements.txt` 是部署时可覆盖的额外清单
2. **容器启动期**：`server.go` 的 `Run()` 起 `go initDependencies()`，在 Sandbox 容器内部执行 pip 安装
3. **环境准备**：将 Python 库准备到同一个 Sandbox 执行环境
4. **执行期**：所有代码节点共享这套环境，没有按请求或按 Skill 激活不同包版本的机制

Coze 第一版借鉴「全局清单 + 安装阶段集中处理 + 执行期只读使用」，但不照搬 Dify 在共享 Sandbox 内直接执行 pip 的做法：Coze 仍使用一次性执行容器，pip 安装由独立、受限的 builder 容器完成。

**Dify 的安全弱点（须避免照搬）**：

- `pip install` **在 sandbox 容器内以 root 直接执行，零隔离**——不在 chroot 内、不受 seccomp 约束；且清单文件来自宿主挂载目录，能写该目录者重启容器即得 root
- 设计错位：用 chroot + seccomp 严防用户代码，却让安装代码裸奔 root，而安装的包恰恰是陌生人写的
- chroot 不是安全边界（业界共识）
- seccomp 白名单约 40 个 syscall，含 `openat`/`read`/`write`/`ioctl`/`mmap`/`socket`，且 FAQ 明确指导「遇 operation not permitted 就往 `ALLOWED_SYSCALLS` 里加」，白名单会被业务需求持续撑大
- 所有租户代码共享同一容器进程组，一次逃逸影响全部
- ticker 周期重装使攻击窗口反复打开，且 pip 会检查新版本，意味着今天安全的包明天被投毒后会自动装入
- 实证：GitHub issue #177「Security issue on sandbox?」报告 command injection / RCE；Imperva 研究团队曾挖到跨租户信息泄露漏洞；社区 issue #32987 请求增加 QEMU microVM 可插拔后端，等于官方承认 chroot + seccomp 不足

**Dify 之所以敢这样做，是威胁模型不同**：其 Code 节点的代码由开发者在工作流编排界面自己编写，能写代码者已持有后台权限，属半可信。隔离目标是「防手滑 + 基本多租户隔离」，不是「防恶意攻击者」。

**取舍结论**：

| 借鉴 | 不借鉴 |
| --- | --- |
| `requirements.txt` 集中声明 + 安装期集中处理 | 单容器多租户共享（Coze 的 per-run 独立容器更强） |
| `PIP_MIRROR_URL` 走内网/国内镜像 | chroot 当安全边界（Coze 有真容器 + 可上 gVisor） |
| 硬链接 + `chmod 444` 只读，运行时不可篡改 | pip 无沙箱 root 直跑（须放独立构建容器） |
| Squid `deny to_private_networks` 的 SSRF 白名单 | ticker 周期性重装 |
| 依赖清单纳入版本管理与审核 | 宿主挂载清单文件（改为纳入 Git 走 code review） |

**总体判断：Coze 现有执行容器的隔离基线已高于 DifySandbox。需要补的只有「依赖如何进入」这一件事，机制可借鉴，运行模型不可照搬。**

### 3.8 分阶段实施

**Phase 1：全局预装依赖（当前实现范围）**

1. 新增全局 requirements 路径配置，默认指向管理员维护的 `python-requirements.txt`
2. 新增依赖根目录配置，保存 `versions/`、`current` 和构建 manifest
3. 后端启动时计算 requirements 内容 + 运行镜像 digest 的 fingerprint
4. fingerprint 未变化时复用当前目录；变化时启动独立 deps-builder 容器
5. builder 使用同一执行镜像、内部 pip 镜像、资源限制和 staging 挂载，成功后原子切换 `current`
6. Docker 执行 Runner 将 `current` 只读挂载到 `/opt/skill-deps`，设置 `PYTHONPATH`
7. 切换成功后执行 best-effort GC，只保留配置数量的完整版本；默认保留 current 和最近一个旧版本
8. 构建失败阻止后端完成启动，避免服务在依赖不完整时运行

第一阶段不包含：按 Skill 解析依赖、依赖 artifact 上传 MinIO、多个依赖环境并存、gVisor 接入。

**Phase 2：多环境与分布式执行（后续）**

只有当不同 Skill 需要互不兼容的包版本，或服务扩展到多实例后，才考虑按依赖 fingerprint 管理多个环境、共享 artifact 和跨节点分发。

## 4. 问题 B：产物与工作区文件管理

### 4.1 首要设计原则

**平台必须适配 Skill，不能要求 Skill 适配平台。** Skill 可能由客户上传，平台对其内容完全不可控。

由此推导出一条硬约束：**任何需要 Skill 作者配合的方案都不成立**，包括但不限于要求产物写入指定子目录、要求在 SKILL.md 中声明状态文件、要求脚本读取平台注入的特殊字段。

参照 Dify 的 `AgentSandboxInfo(workspace_cwd=".")`：只给代码一个工作目录，不规定目录内部结构，其余全由平台负责。

**已否决的方案**：

| 方案 | 否决理由 |
| --- | --- |
| 要求产物写入 `output/` 子目录 | `build_report.py` 中 `if Path(filename).name != filename: raise ValueError` 显式拒绝带目录路径，该约定会直接使其崩溃；且无法约束客户上传的 Skill |
| 预取文件放 `.inputs/` 子目录 + 注入 `_files` 路径映射 | 机制上与平铺无本质区别（都是往同一挂载目录写文件），但第三方 Skill 无法得知该约定，需模型转接路径，反而更复杂。改为原名平铺 |
| 在 SKILL.md 中声明持久状态文件，由平台自动预取 | 需 Skill 作者配合，违反首要原则 |

### 4.2 工作目录生命周期：按 run 创建与销毁

**已确认决策：工作目录不持久，按 run 创建、run 结束即销毁。**

```
run 开始  → 创建空工作目录
run 内部  → 任意多次工具调用共享该目录
run 结束  → 收集产物上传 → 销毁目录
```

**理由一：磁盘占用从「不可预测的累积」变为「可计算的瞬时值」**

| 方案 | 峰值磁盘占用 | 是否可控 |
| --- | --- | --- |
| 会话级持久 + TTL | 活跃会话数 × 平均目录大小 | 否，会话数单调增长，需 TTL + 清理任务 + 配额三重机制勉强兜住 |
| **按 run 创建销毁** | **并发 run 数 × 单次配额** | **是，两个因子均可控（限流 + 配额），严格有界** |

**理由二（更关键）：宿主目录的持久性是「单机部署下的假象」**

工作目录路径由 conversationID 决定，落在某台机器的本地磁盘上。一旦多实例部署（对应 `skills-integration-roadmap.md` V4），请求可能落到任意实例：

- 第 1 轮落在实例 A，文件写在 A 的磁盘
- 第 2 轮落在实例 B，B 上没有该文件 → **历史静默丢失，不报错**

即：**任何依赖宿主目录持久性的功能，在分布式下必然随机失效。** 现在改为按 run 销毁，是在提前消除一个未来必然爆发的问题，而非单纯的磁盘优化。

**理由三：架构分层因此变干净**

```
对象存储（MinIO） = 唯一持久层，承载产物与跨 run 状态
       ↕
宿主工作目录      = 纯 scratch space，生命周期等于一次 run
       ↕
容器内的 .        = 该 scratch space 的视图
```

核心认知：**不存在「宿主目录需要持久」这个需求，只存在「状态需要持久」这个需求；而状态持久应由对象存储承担，不该由某台机器的磁盘承担。**

这与「无状态服务 + 外部存储」是同一模式，只是缩到单机尺度：状态外置后，执行环境才能随意销毁重建。

**必须配套解决：并发隔离**

当前工作目录路径为 `<根>/<会话ID>`。改为按 run 创建销毁后，同一会话的并发 run 会**撞到同一目录**，且一个 run 销毁目录时另一个可能正在使用，导致数据损坏。

并发是真实可能的：用户快速连发消息、前端重试、取消后立刻重发。

**因此路径必须改为按 run 隔离**，建议 `<根>/<会话ID>/<runID>`。具体格式待拍板（见 T3）；runID 目前在执行链上取不到，透传成本见 2.9。

**尚未覆盖的第四种状态：中断（interrupt）**

D12 只列了 `completed` / `failed` / `cancelled`，但 agent 还有「中断等用户回答」这一形态（`message_event.go:340` 一带）。它两头不讨好：

- 当作 run 收尾 → 目录被销毁，而 resume 是**重新走一遍执行链**（见 2.9），恢复后是空目录，中断前的产出只能靠 file_key 预取找回
- 不当作收尾 → 目录一直挂着直到用户回答；用户不回答就永不释放，4.2 承诺的「并发 run 数 × 单次配额」上界被打破

需明确取舍（T29）。实际使用中尚未遇到过中断场景，可先按「当作收尾 + 依赖预取找回」处理，但必须给中断态目录留兜底 TTL，否则上界不成立。

### 4.3 容器内路径语义

执行容器内有两个挂载点，职责严格分离：

| 容器内路径 | 权限 | 内容 | 宿主对应 |
| --- | --- | --- | --- |
| `/opt/skill` | 只读 | Skill 的脚本与配置 | Skill 目录 |
| `/workspace` | 可读写 | 脚本读写的数据文件 | 工作目录 |

关键事实：

- **`--workdir` 设为 `/workspace`**，即脚本的 `.` 与 `Path.cwd()` 指向工作目录，**不是** Skill 目录
- **脚本以绝对路径调用**：`python /opt/skill/<skill>/<脚本名>`
- 此处没有 shell 会话，是一次性进程执行；`--workdir` 设置的是进程 cwd（等价于 fork+exec 前 chdir）

代码目录只读是重要的安全设计：**防止脚本篡改自身或其他 Skill 的代码**，否则可留持久化后门。

**一个影响依赖方案的细节**：`build_report.py` 中 `from report_format import format_report` 能正常工作，尽管 cwd 是 `/workspace` 而两个文件都在 `/opt/skill/<skill>/`。原因是 **Python 放进 `sys.path[0]` 的是「脚本所在目录」而非 cwd**。

推论：全局依赖目录位于 `/opt/skill-deps/`，而 `sys.path[0]` 是 `/opt/skill/<skill>/`。解释器不会自动搜索独立挂载点，因此必须在执行容器中设置 `PYTHONPATH=/opt/skill-deps`。

### 4.4 工作目录的污染源与治理

工作目录是宿主真实目录，容器内对它的写入会直接落到宿主磁盘。除脚本主动写的文件外，还有若干非预期来源。

| 污染源 | 现状 | 结论 |
| --- | --- | --- |
| Python 字节码缓存 `__pycache__/`、`*.pyc` | 启动参数已设 `PYTHONDONTWRITEBYTECODE=1` | 已挡住，无需处理 |
| 临时文件 | `TMPDIR` 已指向工作目录下 `.tmp/` | 已隔离，但**从不清理**，需按 run 清理 |
| **各类库的缓存（最大隐患）** | `HOME` 被设为工作目录根 | **需治理，见下** |
| 脚本自身的中间文件（`temp.json`、`debug.log`） | 无法与真产物区分 | 只能靠配额兜底 |

**关于 `HOME` 指向工作目录根**

这样设置本身是合理的 workaround：容器根文件系统只读，默认 HOME 不可写，许多库会直接崩溃，只能指向唯一可写位置。

副作用是所有写 `$HOME` 的库缓存全部落在工作目录根上，典型包括：

- `.cache/` — pip、matplotlib、huggingface、各类 HTTP 库
- `.config/`、`.local/`
- `.matplotlib/` — **首次 import matplotlib 即生成字体缓存，数 MB 至数十 MB**

`--read-only` 只约束容器根文件系统，对挂载进来的工作目录无效，因此这些内容确实写入宿主磁盘。

**对产物识别的直接影响**：差集会将这些缓存全部判为产物并上传。一个 import 了 matplotlib 的脚本会产生数十个字体缓存文件。

当前它们恰好都以 `.` 开头，会被 4.6 的过滤规则排除——但这是**巧合而非设计**。

**建议改动（一行）**：将 `HOME` 指向工作目录下的 `.home/` 子目录，与 `.tmp/` 保持一致。此后所有库缓存集中于 `.home/`，工作目录根只剩脚本显式写入的文件，不再依赖「缓存目录恰好带点前缀」这一巧合。

### 4.5 文件拉取（预取）设计

**时机**：预取发生在容器启动**之前**，由平台执行；产物收集发生在 run 结束之后。二者不可颠倒。

**按需，不全量**。全量拉取的四个问题：

1. **直接破坏 4.2 的磁盘可控性**：全量拉取会让单次 run 的瞬时占用变回「会话文件总量」，而会话文件只增不减，等于把刚消除的不可控累积又引回来
2. 性能不可接受：会话可能积累数十文件、数百 MB，每次调用全量拉取，下载量随会话单调上升，而实际需要的通常只有一两个
3. 语义错误：模型已点名要用哪个文件，全量拉取等于把按需加载退化为预加载一切
4. 污染产物识别：拉入的文件全会被差集判为候选产物，需全部登记进预取清单才能排除

**命名：原名平铺到工作目录根**

- 脚本 `open("data.csv")` 即可读到，**第三方 Skill 零适配**，符合 4.1 首要原则
- 重名处理：目标路径已存在时改名（`report.txt` → `report-1.txt`），并在返回值中告知模型实际路径

**已否决**：`{file_key}-{原文件名}` 命名。虽然天然避免重名，但脚本无法预知该名称，必须靠平台注入路径映射再由模型转接，违反首要原则。

**平台如何知道该预取哪些文件**

由模型在工具调用参数中显式指定 file_key 列表。模型可从两处获知：

- 历史中的 `produced_files`（跨 run 重放，见 2.4）
- `list_files` 工具兜底

引导手段是工具描述（描述即提示词）：明确写出「若脚本需读写此前生成的文件，必须传入对应 file_key，否则该文件不存在」。

**可靠性与自愈**：多数缺失会显式失败——脚本以读模式 `open()` 抛 `FileNotFoundError`，错误进入 stdout/stderr 并回到模型上下文，模型可自行补传 file_key 重试。

**本地清单**：需在工作目录维护一份清单（如 `.manifest.json`，点前缀故被产物过滤排除），记录 `file_key → 相对路径`。用途有二：

- 差集时精确排除预取文件，避免回环
- 判断是否已在本地（按 run 销毁后此用途弱化，但同一 run 内多次调用仍有效）

不可靠的替代做法：查元数据表取原文件名后检查同名文件是否存在——同名文件可能是脚本自己刚写的，会误判。

**安全：文件名清洗**

对象存储中记录的原始文件名来自脚本，属不可信输入。若某脚本写出名为 `../../etc/passwd` 的文件并被上传，下次预取拼接路径即构成路径穿越。

因此预取时**必须**对原文件名做 sanitize：只取 basename，过滤路径分隔符与 `..`。仓库中 `resolveScript()` / `validateScriptFile` / `validateDirectoryWithinRoot` 已有同类校验，可照抄风格。

**配额**：单次预取的文件数与总大小均需上限，否则模型一次传入大量 file_key 即可撑爆磁盘。

**已否决**：向脚本注入对象存储凭证与 SDK 让其直连——等于把云凭证交给不可信代码。脚本应完全不知道对象存储的存在，云凭证一次都不进入容器。

### 4.6 产物识别与收集时机

**收集时机：run 结束后统一收集，不是每次工具调用后。**

关键洞察：**run 内部的文件传递不经过对象存储**。一次 run 内可能有多次脚本调用（ReAct 循环），它们共享同一工作目录（生命周期即 run 生命周期），因此后一次调用可直接读前一次写下的文件。

```
用户："生成报表然后转成 PDF"

run 开始 → 创建空工作目录
  ├─ 调用1: build_report.py → 写 report.txt
  ├─ 调用2: to_pdf.py → open("report.txt") 直接读到   ← 不经对象存储
  └─ 写出 report.pdf
run 结束 → 收集 {report.txt, report.pdf} → 上传 → 销毁目录
```

作用域划分：

| 作用域 | 传递机制 | 是否需上传 |
| --- | --- | --- |
| run 内部（多次工具调用之间） | 工作目录本身 | 否 |
| 跨 run | file_key + 对象存储 | run 结束时上传 |

若按工具调用收集，还会产生额外坏处：`report.txt` 在 run 内被修改 3 次即产生 3 个版本对象，其中 2 个是垃圾。

**识别方法：全量减预取清单**

因工作目录**从空开始**（按 run 创建），无需时间戳快照与 mtime 比对：

```
产物 = 目录内全部文件 − 本次 run 预取过的文件
```

只需维护预取清单即可。这比原「快照差异法」更简单，且不存在「被修改文件」这一判断分支。

**过滤规则**（平台侧，Skill 零感知）：跳过 `.` 开头的路径（覆盖 `.tmp/`、`.home/`、`.cache/`、`.config/`、`.manifest.json`）、`__pycache__/`、`*.pyc`。

**配额按 run 统计，不按工具调用**：run 内多次调用产生的中间文件会累积，故「产物数量上限 / 总大小上限」的口径是整个 run。超出时不全量上传，仅返回汇总（如「另有 N 个文件未登记」）。

**收尾入口须覆盖三种终态**：run 可能以 `completed` / `failed` / `cancelled` 结束，**三者都要收集产物**——失败或取消的 run 也可能已产出用户需要的部分结果。因此应挂在统一的 run 收尾入口，与 2026-09-15 修复失败 run 计费漏账（`StepToFailed` 携带 usage，commit `c8d669fa`）是同一位置。

### 4.7 上传流程

> 最新交付决策：4.6 的差集结果是“候选产物”，不等于最终交付物。模型结合 Skill 说明和用户上下文选择文件，并通过 `file_upload` 触发单文件上传；平台不根据文件名猜测最终报告，也不默认全量上传。

```
run 收尾（三种终态均执行）
  ↓
产物 = 目录全部文件 − 预取清单，再过滤噪音
  ↓
配额检查（数量 / 总大小 / 单文件 200MB）
  ↓
逐个上传 MinIO（复用 backend/infra/storage）
  对象 key: conversations/{会话ID}/runs/{runID}/{随机串}-{原文件名}
  ↓
写元数据表（新建，不复用 files 表）
  conversation_id / run_id / 原始文件名 / 大小 / content_type / 对象key / file_key
  ↓
失败者不阻塞 run，单独记录
  ↓
销毁工作目录
  ↓
双通道分发（见 4.9）
```

对象 key 中的随机串用于避免同名文件互相覆盖（`build_report.py` 默认写 `report.txt`，用户调用两次即有两个版本，均应保留）。

**失败处理已确认**：脚本成功即 run 成功；上传失败的文件不出现在 `produced_files`，改在返回值中单独说明，使模型能告知用户。

**幂等键已确认**：`run_id` + 文件相对路径，在元数据表上建唯一约束。上传过程中崩溃后重试靠此防重复，与计费通知表的幂等设计同一模式。

**单文件大小上限已确认**：沿用现有 200MB（与 `application/upload/consts.go` 的 `maxFileSize` 一致）。

**一个必须先解决的时序矛盾：file_key 从哪来**

本节规定「run 收尾才上传、才写元数据表」，而 4.10 / D18 规定「模型通过 `script_run` 的**返回值**拿到 `produced_files` 的 file_key」。这两条凑不到一起：

- 返回值产生于 **run 中途**（工具返回的那一刻）
- file_key 产生于 **run 结束**（上传 + 写表之后）
- 更致命的是：**工具返回值一旦写进消息就不可回填**。2.4 已确认它会被持久化并跨 run 原样重放，因此不存在「先返回占位、稍后补真值」的事后修正机会

照现在的文字直接开工，结果是「模型永远拿不到 file_key，主力通道退化为只能靠 `list_files`」——正好落进 4.10 论证要避免的局面。

**推荐解法：file_key 预分配（占位登记）**

把「发 ID」和「传内容」拆开：

```
script_run 返回前（run 中途）
  ↓ 算差集，得到本次新增文件
  ↓ 为每个文件生成 file_key，写元数据表，status = pending
  ↓ file_key 直接进工具返回值        ← 模型立刻拿到稳定 ID
run 收尾
  ↓ 按 pending 清单上传内容到对象存储
  ↓ 成功 → status = ready；失败 → status = failed
```

为什么这样两边都成立：

- **ID 是标识不是内容**。发 ID 只需一次本地写表，不要求文件已在对象存储里
- **内容仍是最终版**。上传推迟到 run 收尾，「后续调用可能修改同一文件」这一诉求（D12 的由来）完整保留
- **幂等照旧**。pending 行本身就是占位，D20 的 `run_id + 相对路径` 唯一约束不受影响
- **崩溃可回收**。进程死在中途只留下 pending 行 + 无对象的 key，正好落进 T18 的 GC 范围

代价（均可接受）：

- 元数据表需要 `pending / ready / failed` 状态机，T6 的字段清单要相应加上
- **预取与 `list_files` 只能返回 ready**，否则模型会拿到一个下载不到的 key
- 前端 File 内容块（4.9）在 ready 之后才发，与现状一致

**备选解法**（若不愿引入状态机）：

| 方案 | 代价 |
| --- | --- |
| 每次 `script_run` 结束就上传 | 放弃「最终版」语义，同一文件被改 3 次就上传 3 次；实现最简单 |
| `produced_files` 不走返回值，改为 run 末追加一条消息 | 模型要到**下一轮**才看得见 file_key，本轮内无法引用，主力通道时效性降一档 |

三者需择一，见 T26。**这是 B3 阶段开工前的前置条件。**

**已接受的代价**：run 结束才上传，意味着 run 进行中产物仅存在于宿主磁盘，**进程中途崩溃则产物全丢**。这与计费线「进程崩溃无解」是同一类问题，当时决定「场景少见且不好补偿，暂时放过」，此处一致处理。若将来需加强，可做 run 内周期性快照上传，属优化而非当前需求。

### 4.8 给模型：file_key，不给路径，不给 URL

**当前缺陷**：`node_tool_skill.go:137` 将宿主绝对路径返回给模型：

```go
fmt.Fprintf(&result, "exit_code: %d\nworking_directory: %s", response.ExitCode, response.WorkingDir)
```

两个坏处：泄露宿主目录结构；使模型与物理存储位置强耦合，一旦模型在回复中引用该路径或用户记下它，该路径即成为事实上的 API 契约，此后迁移、清理、分片全部受阻。

**修正**：改为相对路径或不再返回。

**核心原则**：模型需要跨轮引用的东西必须是**有 ID 的对象**，不能是**有路径的文件**。ID 不透明、可迁移、可授权、可回收；路径不是。预签名 URL 仅供前端下载展示，不给模型——否则模型可能将其直接吐给用户，既泄露又因时效性导致失效。

### 4.9 双通道：模型与前端各走各路

```
脚本产物
  ├──→ 给模型：工具返回值中的 file_key       （供后续调用）
  └──→ 给前端：消息中的 File 内容块 + 预签名 URL（供下载展示）
```

两条通道**互相独立**。后端上传完成后直接将文件写入消息结构（`ContentTypeFile` + `File` 字段，见 2.5），走流式事件推送，**不经过模型转述**。

理由：模型可能忘记、可能编造不存在的 key、可能吐出 URL。凡是模型可能忘记或编造的信息，都不应让它承担传递责任。

### 4.10 查询：produced_files 为主力，list_files 为兜底

**为什么返回值是主力而非 list_files**——核心原则：**能推送就不要拉取**。

1. **模型必然读到返回值，但不一定会主动调工具**。返回值直接出现在上下文中，无法忽略；调 `list_files` 完全依赖工具描述中的提示词引导，模型经常忘记或自作聪明，无法保证
2. **多一轮 ReAct 循环代价大**：多一次完整 LLM 往返（含全部历史与工具清单重发），即数秒延迟 + 数千 token + 一个额外出错点
3. **全量清单信噪比低**：会话积累十几个文件后，模型需从中挑出「刚生成的那个」，遇 `report_v1`/`report_v2` 极易选错；返回值只含本次新增，指向性强
4. **时序歧义**：`list_files` 只能告知「现在有什么」，无法告知「哪个是刚产生的」，区分需依赖时间戳加模型自行排序，仍不可靠

**为什么「run 结束才返回产物」不算晚**：工具返回值会被持久化并跨 run 原样重放（见 2.4）。第 N 轮的 `produced_files` 在第 N+1 轮仍完整位于历史中，模型直接可用。且产物信息本质上只能在产物存在之后才有——脚本可能生成 0 个、1 个或 5 个文件，文件名也可能动态拼接，执行前无从得知。

**list_files 的定位**：低频、按需、模型明确意识到自己忘记时才调。覆盖两个失效边界（历史轮数截断、落单调用被丢弃）。

**为什么 list_files 兜底必然有效**：`tools` 字段与 `messages` 平级、每轮重发，**永不被截断**；而文件清单位于历史消息中，会被截断。故模型即使完全忘记生成过什么文件，仍看得见 `list_files` 工具，可随时重新查出清单。

工具设计：

- 入参可为空（默认返回当前会话全部）
- 查询：`conversation_id` 匹配 + 状态有效 + 未删除，按创建时间倒序
- 返回扁平文本，一行一个：

```
files:
  fk_a1b2c3   report.txt    12.3KB   刚刚
  fk_d4e5f6   report.txt    11.8KB   3分钟前
  fk_g7h8i9   notes.txt     1.2KB    5分钟前
```

- 同名多版本**全部列出**，靠时间戳区分，由模型自行选择，不代替它决定「只显示最新」

**可选第三档**：若业务中文件数量少而关键（如单会话不超过数个），可将当前会话文件清单摘要注入 system prompt。system prompt 永不被截断，可靠性最高；代价是每轮多数百 token 且每次请求需查库。仅在能确定文件数量有上限时采用。

### 4.10.1 Coze 文件消息与流式结束时序

Coze 的 `conversation.stream.done` 由 AgentRun 后端生成，不是大模型生成。后端在 run 状态完成后发送 `RunEventStreamDone`，应用层再将其转换为 SSE `done`。因此交付文件必须在 `done` 之前发送，不能追加到结束事件之后。

第一版复用现有消息协议，不新增独立文件事件：

```text
conversation.message.delta
conversation.message.completed       普通文本完成
conversation.message.completed       content_type=file 的 assistant 文件消息
conversation.stream.done             保持既有结束语义
```

文件消息使用现有 `content_type=file` / `file_list` 结构，包含 `file_key`、`file_name`、`file_type`、`file_size` 和签名 `file_url`。后端应先持久化 assistant 文件消息，再推送流事件，确保刷新历史消息后仍可展示。

Coze 前端已经存在 `FileContent` / `FileCard` 渲染链路，可以显示文件名、类型和大小。现已在 `FileCard` 成功态增加可选复制/下载按钮：仅 assistant 产物文件开启，用户上传文件卡片保持原有复制/上传交互，不改变后端文件消息协议。

该时序参考 Dify 的 `message_file` → `message_end` 做法；Dify 将文件事件独立于普通文本事件推送，但共同原则是文件事件先于流结束事件。

### 4.10.2 模型选择最终交付物

`script_run` 只返回本次执行新增或修改的 workspace 文件候选列表，不自动上传全部候选文件。模型结合 Skill 说明和用户请求选择最终交付文件，再调用 `file_upload(path)`；后端负责 workspace 路径校验、对象存储上传、可信元信息生成、assistant 文件消息持久化和签名 URL 生成。

`file_upload` 的工具返回值只向模型提供文件名、类型、大小和“平台将附加文件卡片”的状态，不返回对象存储路径、签名 URL 或可被模型拼接成链接的内部 `file_key`。系统提示明确禁止输出 `file://`、对象存储 URL、宿主路径或自造 Markdown 下载链接；下载入口由 assistant 文件卡片负责。

自动文件解析只处理用户输入文件；assistant 已交付的文件消息不再次注入模型或消耗本轮解析配额。相同对象存储 URI 在一轮对话内只下载、解析一次，避免历史消息与当前消息重复计算。

这样可以避免平台误把处理中间表、调试日志或缓存文件交付给用户，也不要求 Skill 作者遵守额外的输出目录约定。模型只负责选择，平台负责授权、上传、消息和下载能力。

### 4.11 已知限制：状态型 Skill 的静默降级

工作目录按 run 销毁后，**依赖「上次写的文件还在」的 Skill 会失效**。

以 `notes.py` 为例，其核心功能是跨调用追加累积：

```python
path = pathlib.Path.cwd() / "notes.txt"
with path.open("a", encoding="utf-8") as file:    # 追加模式
    file.write(params["note"] + "\n")
notes = path.read_text(encoding="utf-8").splitlines()
```

正常情况下平台可通过预取把上一轮的 `notes.txt` 放回工作目录，脚本代码一字不改即可继续工作。**但若模型忘记传 file_key**，因该脚本使用追加模式（文件不存在时静默新建），它不会报错，而是**从空白开始，历史静默丢失**。

**这是「不可控 Skill + 无状态工作目录」组合的固有代价，平台侧无解**：

- 平台无法判断哪个文件是「状态」、哪个是「产物」、哪个是中间垃圾
- 全量预取该会话所有文件可兜住，但每次 run 的瞬时磁盘占用将变回「会话文件总量」，4.2 追求的磁盘可控性随之失效
- 要求 Skill 声明状态文件违反 4.1 首要原则

**责任划分**：平台提供能力 + 明确引导 + 让失败可见；Skill 自身是否健壮由作者负责。

**这不是妥协，而是业界现状**：ChatGPT 的 code interpreter 沙箱同样是短生命周期的，跨轮文件引用靠 file_id 显式传递，没有任何平台会自动保留「脚本碰巧写在 cwd 里的状态」。

### 4.12 Dify 的工作区抽象（长期参考）

Dify 未使用目录，而是建立了 Workspace Binding 抽象（`api/services/agent/workspace_service.py`）：

- **归属**：`WorkspaceOwnerScope` = 租户 + 应用 + owner 类型 + owner ID + scope_key，多租户天然隔离
- **后端引用**：工作区指向 `backend_binding_ref`（后端存储引用），而非宿主路径
- **会话快照**：`save_binding_session_snapshot` 阶段性持久化状态
- **显式退役与回收**：`retire_binding` → `collect_retired_binding`、`retire_workspace`、`retire_all_for_app`
- **代际校验**：`validate_binding_generation` 防止使用已失效的工作区

对应 `skills-integration-roadmap.md` 的 V4。**当前阶段不实现**，仅在多实例部署或需要跨会话工作区时引入。

**关键认知**：workspace 在服务化场景中不是一个「目录」，而是一个「资源」——需要归属、配额、生命周期与访问控制。本地 agent 无需设计这三者，是因为其信任边界即用户自己（你的文件、你的机器、你负责）；服务化场景则必须明确回答「归谁、谁能访问、何时消失」。

**注意**：4.2 采用「按 run 销毁」后，本节的 Binding 抽象已非必需——对象存储 + file_key 已承担了持久层职责。仅当需要跨会话工作区（用户级「项目空间」）时才引入。

### 4.13 分阶段实施与代码触点

3.8 给了问题 A 的四个阶段，问题 B 同样需要。按「先止血、再改地基、最后接通道」排序，每阶段可独立上线。

| 阶段 | 内容 | 主要触点 | 前置 |
| --- | --- | --- | --- |
| **B1 止血**（半天，低风险） | `HOME` → `.home/`；`.tmp/` 按 run 清理；返回值去掉宿主绝对路径 | `runner.go` 的 `dockerArgs()`；`node_tool_skill.go:137` | 无（T12、T13） |
| **B2 工作目录按 run 隔离与销毁** | runID 透传 5 层；路径改 `<根>/<会话ID>/<runID>`；收尾销毁目录 | `contract.go:39`、`single_agent.go:109`、`agent_flow_builder.go:37`、`node_tool_skill.go:32`、`script.go:35`、`runner.go:116`、`run.go:170-181` | T3、T28；**必须同步改工具描述**（T27） |
| **B3 产物登记与上传** | 新建元数据表；差集识别；pending/ready 状态机；收尾上传 + 写 File 内容块 | 新增 entity/repo；`run.go:170-181` 收尾 defer；复用 `infra/storage` | **T26**、T6；依赖 B2 提供 runID |
| **B4 预取** | `script_run` 增 file_key 入参；归属校验；下载到工作目录根；`.manifest.json` | `node_tool_skill.go`；`runner.go` 或新增 workspace 包 | T1、T2、T5；依赖 B3 有 ready 文件可查 |
| **B5 查询兜底** | 新增 `list_files` 工具 | `newSkillTools` 内多调一次 `InferTool`（2.3 的三步法） | B3 |

**依赖关系**：B2 是 B3/B4 的地基——没有 runID 就没有按 run 的目录，也就没有「全量减预取清单」这个前提。B1 与其余阶段完全解耦，现在就能做。

**B2 的回归面要提前认清**：它动的是所有 agent 执行的公共链路（`AgentRuntime` → `ExecuteRequest` → `Config`），不只是 Skill。虽然只是加字段，但这条链上挂着 chatflow、plugin、MCP、workflow 全部工具，改完需跑一遍 agent 相关单测（`node_tool_skill_test.go`、`run_process_event_test.go` 等）。

## 5. 已确认决策清单

| 编号 | 决策 | 依据章节 |
| --- | --- | --- |
| D1 | 不改动 Coze 自带的 Pyodide 沙箱 | 3.2 |
| D2 | 依赖采用「全局 requirements + 启动时独立 builder 安装 + 运行期只读挂载」，运行期保持 `--network none` | 3.3、3.5 |
| D3 | builder 与执行必须使用同一镜像 digest | 3.3 |
| D4 | builder 须独立、非 root、限资源、用完销毁，只写 staging 目录，不得挂载 Docker Socket | 3.4 |
| D5 | 仅管理员可修改全局依赖清单；默认使用清华 PyPI 源，生产可显式切换受控内网镜像并叠加包白名单，审核前置到安装之前 | 3.3、3.4 |
| D6 | 隔离加固选 gVisor（`--runtime=runsc`），配置化可回滚 | 3.1、3.6 |
| D7 | 不复用 `files` 表，新建 Skill 产物元数据表；内容存储复用 `infra/storage` | 2.6 |
| D8 | 平台适配 Skill，**任何需 Skill 作者配合的方案均不成立**；产物识别用「全量减预取清单」 | 4.1、4.6 |
| D9 | **工作目录按 run 创建与销毁，不持久**；对象存储是唯一持久层 | 4.2 |
| D10 | 工作目录路径必须按 run 隔离，避免同会话并发 run 撞目录 | 4.2 |
| D11 | run 内部多次工具调用共享工作目录，文件传递**不经**对象存储 | 4.6 |
| D12 | 产物收集在 **run 结束后**统一执行，覆盖 `completed`/`failed`/`cancelled` 三种终态 | 4.6 |
| D13 | 预取由平台在起容器**之前**执行，按需不全量，原名平铺，重名则改名 | 4.5 |
| D14 | 预取文件名必须 sanitize，防路径穿越 | 4.5 |
| D15 | 云凭证不进容器，脚本完全不知道对象存储存在 | 4.5 |
| D16 | 给模型 file_key，不给宿主路径、不给预签名 URL | 4.8 |
| D17 | 双通道：模型走工具返回值，前端走消息 File 内容块，互不依赖 | 4.9 |
| D18 | `produced_files` 为主力，`list_files` 为兜底 | 4.10 |
| D19 | 上传失败不阻塞 run，单独报告 | 4.7 |
| D20 | 幂等键 = `run_id` + 文件相对路径，元数据表建唯一约束 | 4.7 |
| D21 | 单文件上限沿用 200MB | 4.7 |
| D22 | `HOME` 改指工作目录下 `.home/`；`.tmp/` 按 run 清理 | 4.4 |
| D23 | 第一版依赖目录与 Skill 包分离，位于受控 runtime 目录或 Docker volume；不上传 MinIO、不写宿主系统 Python | 3.3 |
| D24 | 接受限制：状态型 Skill 在模型漏传 file_key 时会静默丢历史，平台无解 | 4.11 |
| D25 | 接受限制：run 中途进程崩溃则产物丢失，与计费线一致处理 | 4.7 |
| D26 | 第一版只有一个当前全局依赖环境，不支持同一包多版本按 Skill 激活 | 3.3、3.8 |
| D27 | requirements 未写版本时首次解析并记录 manifest，后续复用；需升级时修改清单或显式重建 | 3.3 |

## 6. 待拍板事项

### 6.1 阻塞实现的（必须先定）

| 编号 | 事项 | 备注 |
| --- | --- | --- |
| T1 | `script_run` 参数形态 | `file_keys` 作为独立字段，还是自动扫 params 中匹配 file_key 格式的字符串值并替换？前者可控不误替换，后者更鲁棒（模型无需记得填专门字段） |
| T2 | file_key 归属校验规则与 key 形态 | 已不可推迟——预取第一步就是它。风险：file_key 由模型填写，若不校验归属，提示注入可诱导模型传他人 key 实现越权读取。key 建议用不可遍历随机串，不用自增 ID |
| T3 | 工作目录按 run 隔离的路径格式 | 建议 `<根>/<会话ID>/<runID>`。当前为 `<根>/<会话ID>`，并发必撞。**注意这不是纯路径设计**：runID 在执行链上不可达，需穿透 5 层透传（见 2.9）；且不可用「`BuildAgent` 内生成 UUID」的捷径，resume 时 UUID 会变 |
| T4 | 产物配额 | 单次 run 的文件数上限、总大小上限、超出后的处理方式（建议仅返回汇总「另有 N 个文件未登记」） |
| T5 | 单次预取配额 | 文件数与总大小上限，防模型一次传入大量 file_key 撑爆磁盘 |
| T6 | Skill 产物元数据表字段定稿 | 需含 conversation_id、run_id、原始文件名、大小、content_type、对象 key、file_key、归属字段、状态、软删除。**状态字段取值取决于 T26**（推荐方案需 `pending/ready/failed`） |
| T26 | **file_key 的产生时机** | 2026-09-18 复审新增，**优先级最高**。4.7「run 末上传」与 4.10「file_key 走工具返回值」不可同时成立，且返回值持久化后不可回填。推荐「file_key 预分配 + pending/ready 状态机」，另有两个备选，详见 4.7。它同时影响 T1（返回值里放什么）与 T6（表要不要状态字段） |

**建议讨论顺序**：**T26** → T1 → T2 → T3 → T4/T5 → T6。理由：T26 决定 file_key 的生命周期，是 B3 的地基，并会反过来约束 T1 与 T6；T1 决定工具接口这一最外层契约；T2 依赖 T1 确定的参数位置；T3 是路径设计 + 字段透传；T4/T5/T6 是收尾细节。

**对应实施阶段见 4.13（B1-B5）**，B1 与本节全部待拍板项无依赖，可立即开工。

### 6.2 可边做边定的

| 编号 | 事项 | 状态 | 备注 |
| --- | --- | --- | --- |
| T7 | 收尾入口的具体挂载位置 | 待确认 | 需看代码确认，与 2026-09-15 修复失败 run 计费漏账（commit `c8d669fa`）是同一处 |
| T8 | 前端 File 内容块由谁写入 | 待设计 | 双通道中前端那条尚未落到实现 |
| T9 | 上传链路中间步骤失败的处理 | 待设计 | 上传成功但写表失败、写表成功但推前端失败等 |
| T10 | 销毁工作目录失败的残留清理 | 待设计 | 需兜底扫描机制 |
| T11 | 重名改名的具体规则与告知方式 | 待设计 | `report.txt` → `report-1.txt` 后如何让模型得知实际路径 |
| T12 | `HOME` 改指 `.home/`、`.tmp` 按 run 清理 | 待实施 | 独立止血项，一两行改动，随时可做 |
| T13 | `node_tool_skill.go:137` 去除宿主绝对路径 | 待实施 | 见 4.8 |
| T14 | 依赖构建的实现细节 | 已确认 | 后端启动时按 requirements + 镜像 digest fingerprint 校验；变化时启动独立 builder；未变化跳过 |
| T15 | 全局依赖目录的清理与版本保留 | 已实现 | 成功激活后 best-effort GC；默认保留 current 和最近一个完整旧版本，清理残留 staging 与不完整版本；由 `COZE_SKILL_RUNNER_DEPENDENCY_VERSIONS_TO_KEEP` 调整 |
| T16 | 内网 pip 镜像选型与搭建 | 部分确认 | 默认源已切换为清华 PyPI 公共源，解决大陆下载速度问题；生产私有镜像、包白名单和仅允许镜像访问的网络仍属后续治理 |
| T17 | 是否启用 gVisor 及其优先级 | 待拍板 | 建议置于依赖方案落地之后，一次只改一件事便于排查 |
| T18 | **对象存储 GC** | **待讨论（用户明确要求后续处理）** | 元数据软删除后 MinIO 对象仍在；会话删除后产物对象仍在。需后台回收任务：扫软删除超期记录 → 删对象 → 删记录。设计时须保留软删除与对象 key 的可反查关系 |
| T19 | 会话删除时的级联清理 | 待讨论 | 元数据与对象需一并清理，与 T18 可合并设计 |
| T20 | 前端能否主动删除产物 | 待讨论 | |
| T21 | 可观测性指标 | 待设计 | 预取命中率、上传失败率、产物数量分布、磁盘水位、构建失败率 |
| T22 | 新增配置项清单 | 已整理 | 已落地 requirements 路径、依赖根目录、pip 镜像、构建网络、版本保留数及现有 Runner 配置；builder 资源限制目前为代码内固定值 |
| T23 | agent「短期记忆 → 历史轮数」实际配置值 | 待确认 | 默认 100 轮，若实际配置很小，`produced_files` 主力通道的有效期会显著短于预期 |
| T24 | 阿里云 ECS 是否存在 `/dev/kvm` | 待确认 | 决定 Kata / Firecracker / microsandbox 是否重新进入候选 |
| T25 | Skill 上传权限收紧 | 待评估 | `authorization.go:28` 仅校验空间成员身份。若未来开放给业务方或租户上传 Skill，此处优先级高于沙箱 |
| T27 | 改写 `script_run` 工具描述 | 待实施（**须与 B2 同批上线**） | `node_tool_skill.go:99` 现文案 "the current conversation's working directory, which can be reused by later calls"，D9 之后会主动误导模型以为跨 run 持久，放大 4.11 的静默丢历史 |
| T28 | 工作目录的属主与可写性 | 待确认 | `runner.go:117-126` 仅在 backend 为 root 时 chown 给沙箱用户；非 root 时跳过，容器内 65532 用户对 0750 目录写不进去。按 run 新建目录时须保证容器用户可写 |
| T29 | 中断（interrupt）态的目录处理 | 待拍板 | D12 未覆盖中断。当作收尾则 resume 后目录为空；不当作收尾则目录挂到用户回答为止，4.2 的磁盘上界被打破。若按前者处理，须给中断态目录留兜底 TTL |

**编号说明**：T26-T29 为 2026-09-18 复审新增，编号顺延追加，不重排既有编号，以免打断已存在的交叉引用。

### 6.3 已作废

| 原编号 | 事项 | 作废原因 |
| --- | --- | --- |
| 原 T5 | 工作目录 TTL 清理 | D9 改为按 run 创建销毁，不再存在累积，无需 TTL 任务 |
| 原 T6（部分） | 工作目录累积磁盘风险 | 同上。但**单次 run 的配额仍需要**，已拆分为 T4/T5 |

**说明**：原 T6 指出的「`--storage-opt size=` 对 bind mount 无效」这一事实仍然成立，因此 T4/T5 的配额只能在平台侧实现（收集与预取时统计并拒绝），不能依赖 Docker 启动参数。

## 7. 已知遗留问题（与本设计无关，独立记录）

| 编号 | 问题 | 说明 |
| --- | --- | --- |
| L1 | `chat-app-sdk/.env.local` 含真实 PAT 且被 Git 跟踪 | 已建议轮换，未处理 |
| L2 | `message_event.go:340` interrupt 路径的 `ext.token` 显示偏大 | 该路径将 `GetUsage()` 回读写入中断消息的 `ext.token`；自 usage 改为 run 累计值后，此处展示的是累计值而非单次值。**账不会双算**（`handlerAnswer` 已不回写），仅前端展示偏大。如需处理，为其单独保留「最后一次调用」字段 |
| L3 | `backend/Dockerfile:40` deno 预热未锁版本 | 与代码中写死的 `@langchain/pyodide-sandbox@0.0.4` 可能不一致 |
| L4 | `UploadSessionKey` / `GetObjInfoBySessionKey` 为死代码 | 全仓无调用者，不可依赖其提供会话级能力 |
| L5 | `.tmp/` 目录从不清理 | `runner.go:117` 每次 run 前 `MkdirAll` 确保存在，但从不删除，会无限增长。已并入 T12 |

## 8. 变更记录

| 日期 | 变更 |
| --- | --- |
| 2026-09-17 | 初版。记录生态调研结论、依赖方案（构建期安装 + 只读挂载）、gVisor 选型、Dify 参考取舍、产物与工作区文件管理主干设计、15 项已确认决策与 12 项待拍板事项 |
| 2026-09-18 | 第 4 节大幅重写。**推翻原 D9**：工作目录由「会话级持久 + TTL 清理」改为「按 run 创建与销毁」，理由是磁盘占用可严格界定，且宿主目录持久性在多实例下必然失效（4.2）。产物识别由「快照差异法」简化为「全量减预取清单」，因目录从空开始无需 mtime 比对（4.6）。产物收集时机由「容器退出后」修正为「run 结束后」，覆盖三种终态（4.6）。新增 4.3 容器内路径语义（含 `sys.path[0]` 为脚本目录这一影响 `.deps` 的事实）、4.4 污染源治理（`HOME` 指向工作目录根的隐患）、4.5 预取设计（原名平铺、重名改名、文件名 sanitize、本地清单）、4.11 状态型 Skill 静默降级的已知限制。否决 `.inputs/` 子目录与 SKILL.md 状态声明两个方案，强化 4.1「任何需 Skill 作者配合的方案均不成立」。新增 D10-D25 决策；待办重组为「阻塞实现」（T1-T6）与「可边做边定」（T7-T25），原 T5 作废、原 T6 拆分；新增遗留问题 L5 |
| 2026-09-18（复审） | 通读复审并回代码逐条核对。**发现一处设计矛盾**：4.7「run 末上传」与 4.10「file_key 走工具返回值」不可同时成立，且工具返回值持久化后不可回填；补写推荐解法「file_key 预分配 + pending/ready 状态机」及两个备选（4.7 末、新增 T26）。新增 2.9：核实 runID 在执行链 5 层结构体上均不可达，据此修正 T3 的定位（是跨 5 层字段透传而非纯路径设计），并否决「`BuildAgent` 内生成 UUID」的捷径（resume 会变）；同时指出「收尾端有 runID、执行端没有」这一不对称。补 4.2 中断态缺口（D12 只覆盖三种终态，中断两头不讨好）。新增 4.13 问题 B 的分阶段实施与代码触点（B1-B5），补齐此前只有问题 A 有阶段计划的空缺。2.2 补 chown 与可写性事实、2.3 补 `script_run` 现描述与本设计冲突。修正 4.2 对路径格式的错误交叉引用（原写 T13，应为 T3）、删除 T2 备注中失效的「原 T3」。新增 T27-T29 |
| 2026-09-18（依赖方案定稿） | 依赖方案从「按 Skill 发布时构建 `.deps`」改为「管理员维护全局 requirements，后端启动按 fingerprint 校验，变化时由独立 builder 容器构建到受控目录，执行容器只读挂载单一 current 环境」。确认第一版不上传 MinIO、不做按 Skill 多版本激活、不根据 import 自动推导依赖；补充 D26-D27，T14 改为已确认。 |
| 2026-09-20（Phase 1 实现与验证） | 将全局清单固定到仓库 `backend/conf/skill-runtime/python-requirements.txt`，明确忽略 Skill 包内 `requirements.txt`；实现后端启动时 builder、`versions/current` 原子切换、manifest、默认保留 2 个版本和执行期只读挂载。默认 pip 源固定为清华源，仅显式 `COZE_SKILL_RUNNER_PIP_MIRROR` 可覆盖，避免继承宿主 `PIP_INDEX_URL`。完成针对性 Go 测试、Docker 集成测试、`go vet` 与 `8888` 前端真实 Skill 调用验证；补充公共镜像与生产私有镜像治理的边界。 |
| 2026-09-20（Phase 1 生产路径修正） | 将全局清单从会被生产整体挂载覆盖的 `backend/conf/skill-runtime` 移到独立的 `backend/skill-runtime`，镜像路径改为 `/app/resources/skill-runtime`；同步更新 Debug 复制脚本和生产环境变量。`/data/hz-agent/coze/runtime` 仍作为 builder 与执行容器共享的运行时目录。 |
| 2026-09-20（Phase 1 builder 输入修正） | builder 不再把后端容器内的 requirements 路径作为宿主 Docker bind mount；后端读取镜像内清单后通过 stdin 传给 builder，builder 使用 `/dev/stdin` 交给 pip。这样生产只需共享 runtime 输出目录，不需要在宿主机额外放置 requirements 文件。 |
| 2026-09-22（产物交付协议确认） | 确认由模型从 `script_run` 返回的候选文件中选择最终产物，通过 `file_upload` 上传；后端持久化 assistant 文件消息，并在既有 `conversation.stream.done` 之前发送 `content_type=file` 消息。`done` 继续由后端生成，不追加结束后数据；Coze 前端复用现有 FileCard，assistant 文件卡片增加下载按钮。 |
| 2026-09-22（交付卡片与链接防误用） | 交付文件消息以签名 `file_url` 作为完成信号，不要求 `upload_status`；助手文件卡片显示复制与下载按钮。`file_upload` 不再向模型返回 `file_key` 或 URL，系统提示和工具结果明确禁止输出 `file://`、对象存储 URL、本地路径或自造 Markdown 下载链接，下载入口统一由文件卡片提供。 |
| 2026-09-22（下载附件唯一定位） | `file_download` 入参拆为首选 `file_key` 和兼容 `file_name`。后端先对对象存储 URI 做精确匹配，再允许唯一文件名匹配；同名多文件返回候选 key，由模型重试，不按上传顺序或文件名任意选择。 |
| 2026-09-23（历史附件按需读取） | Resolver 只自动解析当前输入附件；历史用户附件仅保留文件名和 `file_key` 元信息，不再参与本轮自动解析、文件数量上限或总字符上限。历史内容由模型按需调用 `file_search`，Skill 需要原始二进制时调用 `file_download`。 |
