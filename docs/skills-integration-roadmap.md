# Coze Skills 分阶段实现路线

> 状态：设计基线，可根据实现过程中的代码事实和测试结果微调
>
> 记录日期：2026-08-31
>
> 目标：在 Coze Studio 中逐步增加类似 Dify 的 Skills 能力，并从可信的本机执行方案开始，逐步演进到受控执行和多实例部署。
>
> 当前进度：V1 本机执行已完成；V1-S Docker 基线沙箱已实现，等待接入后的人工验证。

## 1. 总体原则

本功能按“先跑通，再增强隔离”的顺序实现。每个版本只解决一个主要问题，完成后先验证和 Review，再进入下一版。

第一版本机执行模式明确接受以下前提：

- Skill 脚本来自受信任来源。
- Skill 在 Coze 服务所在机器上执行。
- 新增独立的 `ScriptRunner`，不修改现有 CodeRunner。
- Skill 根目录和 conversation 工作区根目录通过配置指定，默认使用服务工作目录下的运行目录。
- 同一个 conversation 使用同一个懒创建的本机工作目录。
- 暂时不提供完整的多租户安全隔离。

因此，本机执行模式只能用于本地开发、内部测试和受信任 Skill，不作为公网多租户生产方案。Docker 模式只提供基础容器边界，仍需经过生产安全评估。

## 2. 版本路线

### V1：可信环境下直接执行 Skill

**目的：先证明 Skill 能参与一次完整的智能体对话。**

最小链路：

```text
扫描本地 Skill 目录
    -> 将 Skill 清单提供给智能体运行时
    -> 模型决定是否调用通用 script_run 工具
    -> ScriptRunner 在 conversation 工作目录中执行 Python
    -> 返回脚本结果给模型
```

实现范围：

- 扫描本地 Skill 根目录，向模型提供每个 Skill 的名称和简短描述。
- 被选中或被提到的 Skill 再加载完整 `SKILL.md`。
- 增加通用 `script_run` 工具，让模型传入 Skill 名称、相对脚本路径和 JSON 参数。
- 使用独立 `ScriptRunner` 执行本机 Python，不要求脚本包含固定的 `main()` 函数。
- 为每个 conversation 懒创建并复用一个本机工作目录。
- 将执行结果接回现有 Agent/ReAct 工具循环。
- 增加最小的成功、参数传递和脚本失败测试。

本机执行模式不做：

- 不增加 Skill 上传页面。
- 不增加 Skill 数据库表或资源库类型。
- 不增加 `WorkingDir`、`AllowRead`、`AllowWrite` 等 CodeRunner 执行参数。
- 不接入生产级 Sandbox Runner 或独立 Worker。
- 不处理多实例文件同步。

风险边界：ScriptRunner 会直接在宿主机启动 Python，Skill 理论上可以访问宿主机文件、环境变量、网络和外部命令。因此第一版必须限制 Skill 来源和使用环境。

### V1-S：Docker 基线沙箱

**目的：在不修改现有 CodeRunner 的前提下，为 Skill 脚本增加基础容器隔离。**

实现范围：

- 使用固定的 `python:3.12-slim-bookworm` 镜像，镜像必须在启动阶段预先拉取。
- 每次脚本调用创建一个临时容器，结束、失败或超时后清理。
- 只挂载当前 Skill 目录（只读）和 `runtime/conversations/<conversationID>`（读写）。
- 使用只读根文件系统、禁用网络、非 root 用户、能力删除和 CPU/内存/进程数限制。
- 支持多 Python 文件互相导入，以及在 conversation 工作目录中持久化文件。

本版本仍不做依赖安装、镜像构建、分布式执行和生产级恶意代码防护。

### V2：Skill 资源管理和智能体配置

**目的：让用户可以在产品界面管理 Skill，并在智能体中选择启用的 Skill。**

实现范围：

- 资源库支持上传 Skill ZIP 或符合约定的 Skill 文件。
- 校验目录结构、入口文件、名称和版本。
- 保存 Skill 元数据和文件内容。
- 智能体草稿支持选择 Skill。
- 发布版本保存已启用的 Skill 引用。
- 删除和基础版本管理。

这一版仍然不默认解决不可信脚本的安全执行问题，上传和执行权限需要先限制在可信用户或内部环境。

### V3：更强的受控执行隔离

**目的：降低不可信 Skill 对宿主机造成影响的风险。**

实现范围：

- 在 Docker 基线之上引入独立 Worker、专用 Sandbox 或更强的运行时隔离。
- 限制 CPU、内存、执行时长和进程数量。
- 限制文件读取和写入范围。
- 限制网络访问和外部命令执行。
- 将 Skill 执行器与业务权限校验分开。

需要根据 Skill 类型选择运行环境：纯计算型 Skill 可以使用 Pyodide 等受限运行时，需要真实文件或系统命令的 Skill 更适合独立 Worker 或容器。

### V4：多实例和分布式工作区

**目的：让 API 和执行 Worker 可以横向扩展。**

实现范围：

- Skill 包放入对象存储或共享文件存储。
- conversation 工作区使用共享存储或独立 Runtime 管理。
- 在数据库中保存工作区引用或 Binding 引用，而不是依赖某台机器的本地路径。
- 任意 API 实例都能根据 conversation 找到对应执行环境。
- 处理 Worker 故障、重连、并发和清理。

### V5：生产安全和治理

**目的：满足公网、多租户和长期运行场景。**

实现范围：

- Skill 包签名和恶意脚本扫描。
- 依赖包白名单和版本锁定。
- 租户、工作空间、智能体和 conversation 权限校验。
- 执行配额、并发限制和审计日志。
- 网络域名白名单和 SSRF 防护。
- 文件大小限制、生命周期管理和自动清理。
- 监控、告警和失败重试。

## 3. V1 的详细实现拆分

第一版不需要完整的前后端链路，建议只拆成以下几个内部步骤：

### V1-1：定义运行目录和 ScriptRunner 契约

**目的：先让脚本可以在本机被可靠执行，并确定本地开发和生产环境共用的路径配置方式。**

最小运行参数：

```text
script_path     相对 Skill 根目录的脚本路径
conversation_id 用于定位 conversation 工作目录
params          传给脚本的 JSON 参数
```

目录约定：

```text
COZE_SKILLS_ROOT             Skill 根目录，可被多个环境分别配置；默认 resources/conf/skills
COZE_SKILLS_WORKSPACE_ROOT   conversation 工作区根目录；默认 runtime/conversations
{workspace_root}/{conversation_id}  单个 conversation 的工作目录
```

### V1-2：增加 Skill 清单和说明加载

**目的：先让模型知道有哪些 Skill，再按需读取对应的完整说明。**

Skill 放在本地根目录中，目录名和 `SKILL.md` 中的描述用于生成清单。

### V1-3：增加 Skill 执行适配器

**目的：把模型的通用 `script_run` 调用转换为 ScriptRunner 执行。**

适配器负责：

- 校验 Skill 名称和相对脚本路径。
- 校验输入参数。
- 调用 ScriptRunner 并传入 conversation ID。
- 将标准输出、错误输出和退出状态转换为工具结果。

### V1-4：接入 Agent 运行时

**目的：让模型看到 Skill，并能在工具循环中调用它。**

优先复用现有 Agent 工具注册和执行路径，只增加 Skill 类型的适配逻辑，不新增 HTTP 接口。Skill 的执行结果需要按照现有工具结果格式返回给模型，使模型可以继续生成最终回答。

### V1-5：增加最小测试

**目的：确认链路可重复验证，并固定第一版行为。**

至少覆盖：

- Skill 能被注册到智能体运行时。
- 模型工具调用参数可以传给 Python 脚本。
- Python 返回的 JSON 可以回到工具循环。
- Python 执行失败时，错误可以被模型和调用方感知。
- 同一个 conversation 的多次调用使用同一个工作目录。
- 不同 conversation 使用不同工作目录。

### V1-6：Docker 基线沙箱

**目的：把 Skill 执行从宿主机 Python 切换到受限的临时容器。**

实现范围：

- 新增 Docker ScriptRunner，并通过 `COZE_SKILL_RUNNER_TYPE` 选择 `docker` 或 `local`。
- Docker 模式默认使用 `python:3.12-slim-bookworm`，不在运行时安装第三方依赖。
- 当前 Skill 目录只读挂载，conversation 工作目录读写挂载。
- 容器命令使用 `--pull=never`，避免脚本超时窗口内隐式拉取镜像。
- 增加 Docker 集成测试，覆盖多文件导入、文件持久化、跨 conversation 隔离、只读挂载和网络限制。

Docker 执行模式要求部署或启动阶段预先拉取配置的 Python 镜像（例如 `docker pull python:3.12-slim-bookworm`）；Runner 使用 `--pull=never`，不会在脚本执行期间隐式拉取镜像。

## 4. V1 的预计改动规模

本阶段相对于完整 Skills 功能仍然较小，但不会只改一个文件。当前实际包含：

```text
手写代码：Skill 清单、ScriptRunner 工厂、Local Runner、Docker Runner、Agent 工具接入及测试
示例资源：Calculator、Report Builder、Conversation Notes
IDL：不需要
数据库迁移：不需要
前端：不需要
资源库接口：不需要
CodeRunner 核心接口：原则上不需要修改
```

第一步只预计新增 3 个文件：运行契约、本机实现和本机实现测试。

## 5. V1 验收标准

满足以下条件才进入 V2：

```text
1. 本地 Skill 可以被智能体识别。
2. 模型可以生成通用 Skill 调用参数。
3. Coze 可以使用独立 ScriptRunner 执行 Python。
4. Python 结果可以返回模型并完成最终回答。
5. Python 失败时不会导致进程崩溃，调用方能看到明确错误。
6. 没有新增数据库表、HTTP 接口和前端配置页面。
7. 同一 conversation 的脚本可以共享工作目录，不同 conversation 不使用同一个目录。
8. Docker 模式下多文件导入、conversation 文件持久化和基础容器限制测试通过。
9. 文档中明确标注该版本不具备生产级安全隔离能力。
```

## 6. 允许调整的地方

下面内容可以在实现过程中根据源码和测试结果微调，但调整前需要先说明原因：

- Skill 是直接作为 Eino Tool 注册，还是先适配到 Coze 现有的工具收集结构。
- V1 的 Skill 根目录和工作区根目录；后续版本可替换为资源库或对象存储加载。
- Skill 脚本的输入输出约定；V1 允许多文件目录和任意相对脚本路径。
- Skill 结果是直接返回 JSON，还是包装成统一的工具结果结构。
- V2 是先做资源库上传，还是先做智能体绑定配置。

除非重新讨论确认，不应在本阶段提前实现依赖安装、多实例和生产治理能力。
