# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

hr-cli 是听评课(listen)系统的 CLI 封装,基于 Go + cobra + 元数据驱动。把 listen 的 REST API 封装成可被 AI Agent 直接调用的命令行工具。参考了 `/Users/hljy/IdeaProjects/cli`(lark-cli)的架构。

## 常用命令

```bash
# 构建(产物在 bin/hr-cli)
make build

# 直接 go build
go build -o bin/hr-cli .

# 运行全部测试
go test ./...

# 运行单个包的测试
go test ./internal/client/...

# 运行特定测试
go test ./internal/client/ -run TestDo_Success

# 清理构建产物
make clean
```

## AI Agent 使用 hr-cli 前必读

**CRITICAL — 系统必须登录后才能使用,任何 listen API 调用都要 hrToken,不传会 401。**

### Step 1: 检查登录态(每次会话开始时 MUST 先做)

```bash
hr-cli doctor                       # 推荐:一次自检配置/token/连通性/schoolId
                                    # exit 0 = 全就绪;exit 1 = 有 FAIL,按提示修
hr-cli --agent doctor               # AI 消费场景:输出结构化 JSON
```

- 若已登录(配置文件存在且非空)→ 继续处理用户请求
- 若未登录或 401 → **MUST 先引导用户完成登录**(详见 [`skills/hr-auth/SKILL.md`](skills/hr-auth/SKILL.md)):
  ```
  你需要先登录。请执行:
  1. 确认 `~/.haiclaw/saas-config.json` 存在(由 haiclaw 工具生成);若无,降级:`export SAAS_AUTH="<从 saas 系统 UI 拿的 Authorization>"`
  2. hr-cli saas +login
  ```
  **登录是前置门槛,未登录前不处理任何业务请求。**

### Step 2: 读取领域 skill

根据用户请求场景,**MUST 先用 Read 工具读取**对应 skill:

1. [`skills/hr-shared/SKILL.md`](skills/hr-shared/SKILL.md) — 通用规则 + **领域术语速查表**(触发词 → service.method 映射,是场景路由的第一入口)
2. 对应领域 skill(如 [`skills/hr-invite/SKILL.md`](skills/hr-invite/SKILL.md) 处理邀课场景)

读完后,根据用户意图选择命令调用。

## 快速参考

```bash
# 查询
hr-cli schema                      # 列所有 service
hr-cli schema course               # 列 course 的 method
hr-cli schema course.add           # 看 add 参数结构
hr-cli which 邀课                  # 按意图关键词找命令(同义词友好,如"查教师"/"建课")
hr-cli doctor                      # 环境自检:配置/token/连通性/schoolId(FAIL → exit 1)

# service 命令(原始 API,1:1 映射 REST)
hr-cli course add --data '{...}'

# shortcut 命令(智能封装,优先用)
hr-cli course +create --school-id ... --teacher-id ...

# 预览请求(不发真实请求)
hr-cli course add --data '{...}' --dry-run

# 列表查询 + --jq 投影(列表接口几乎必用,详见 hr-shared 的「列表查询 SOP」)
hr-cli listen listen_get_lists --data '{...}' \
  --jq '{total: .pagination.allCount, items: [.listenList[] | {teacher: .courseInfo.teacherName, score: .commentScore}]}'

# --agent: AI agent 模式信号,强制 compact JSON(无缩进);doctor/which 在 agent 模式下输出结构化 JSON
hr-cli --agent schema course
hr-cli --agent doctor
hr-cli --agent which 邀课

# 测试环境
hr-cli --base-url "https://hrjy-test.hailiangedu.com/hr" --token "eyJ..." course add ...
# 或把地址写入 .hr-cli.json(推荐,以后不必每次传 --base-url):
#   {"baseURL":"https://hrjy-test.hailiangedu.com/hr"}
# 解析链:--base-url flag > HR_CLI_BASE_URL env > .hr-cli.json > http://localhost:8080
```

## 架构(三层分层,禁止跨层)

```
cmd/                 cobra 命令树组装 + 全局 flag 解析
   ↓
shortcuts/           高频操作的高级封装(手写,+create/+search-teacher)
   ↓ 调 runner.RunMethod
internal/runner/     元数据查找 + 调 client + 返回 Result
   ↓
internal/client/     net/http + hrToken 注入 + ResultJson 解析
```

**关键约束**:shortcut 必须通过 `RuntimeContext.runMethod` 闭包调 service 层,**禁止直调 client**(由 `shortcuts/common/runner.go` 的注入机制强制保证)。

### 元数据驱动(核心机制)

- `internal/registry/meta_data.json`:API 元数据,通过 `//go:embed` 编译进二进制
- `cmd/service.go` 启动时读元数据,为每个 service.method 自动生成 cobra 子命令(`course add`)
- `cmd/schema.go` 暴露 `requestSchema` 和 `responseSchema`,`schema <service>.<method>` 命令直接打印
- **加新 API 只改 JSON,不改 Go 代码**;`make build` 重新编译即生效
- **列表接口(`*_get_lists`)必须补 `responseSchema`**:标 listKey + 常用投影字段,AI 写 `--jq` 才不用发真实调用探查。详见 [`docs/add-skill-workflow.md`](docs/add-skill-workflow.md) Step 3
- 元数据源:YApi(项目 59 "鸿儒教研迭代api"),手动用 YApi MCP 拉取后转换

### 认证

- Header 名:`hrToken`(小写 h,大写 T)
- 解析链(`internal/config/config.go`):`--token` flag > `LISTEN_TOKEN` env
- 测试环境完整 URL 示例:`https://hrjy-test.hailiangedu.com/hr/listen/v1/course/...`(网关前缀 `/hr` 由 `--base-url` 或 `.hr-cli.json` 提供,basePath 由 meta_data.json 按 service 决定:`/listen`(course/listen 业务)或 `/lesson`(groupmanage/saas))
- **base-url 解析链**:`--base-url` flag > `HR_CLI_BASE_URL` env > `<repo>/.hr-cli.json`(项目配置,与登录态 `~/.hr-cli/config.json` 分离)> 默认 `http://localhost:8080`
- **schoolId/campusId 自动注入**:`saas +login` 写入 `~/.hr-cli/config.json` 后,所有 shortcut/service 命令自动复用,不必传 `--school-id`/`--campus-id` flag(由 `internal/runner/runner.go` 的 body 层兜底实现)

### 命令树组装

`cmd/build.go` 是命令树的唯一入口:
1. `NewCmdVersion()` → version 命令
2. `shortcuts.NewCmds(optsFactory)` → 按 service 分组的 shortcut 命令
3. `NewCmdService(shortcutCmds)` → 元数据驱动生成 service 命令,并把对应 shortcut 挂到该 service 下
4. `NewCmdSchema()` → schema 查询命令

新增 shortcut:`shortcuts/register.go` 的 `All()` 里 append,并在对应领域包实现 `Shortcut` struct(`BuildBody` + `After` 两回调)。

### 错误处理约定

- cobra 根命令 `SilenceUsage: true` + `SilenceErrors: true`,业务错误不重复打 Usage
- `main.go` 统一把 error 输出到 stderr,exit code 统一为 1
- listen 业务错误(status≠200)带后端 message(`internal/client/client.go` L65),便于诊断字段级错误

### 校验分层:协议错误 vs 业务错误

**原则:业务规则单一来源,CLI 不复制后端规则。**

| 错误类型 | 校验位置 | 例子 |
|---------|---------|------|
| 协议/格式错误 | **CLI 本地拦** | `--data` 非 JSON、`--node` 非整数、必填 flag 缺失 |
| 业务规则错误 | **后端拦,CLI 透传** | node 范围(1-15)、去重约束、权限、量表适用性 |

**反模式**:把业务规则(如 node 必须在 1-15)硬编码进 CLI 校验。后端改规则时 CLI 会错误拦截合法请求,且 skill 文档也要同步,增加维护成本。后端消息不清晰时,**治本是让后端修消息**,不是 CLI 替后端擦屁股。

## MVP 边界

**接口清单的单一真相源是 [`skills/hr-shared/SKILL.md`](skills/hr-shared/SKILL.md) 的「领域术语速查表」**(触发词 → service.method 映射,新接入接口只改那里)。

通用边界:
- 所有 list 接口(`*_get_lists`)强制按当前用户过滤,不能查"某老师评的课/讲的课"
- 按学校名查 schoolId 不支持,需用户从 Web URL 复制

用户请求未支持能力时,诚实告知边界,不要瞎编。领域特定边界见对应 skill 的 MVP 段。

## 新增 Skill 的标准流程

当要把一个新场景做成 skill 时,**按 [`docs/add-skill-workflow.md`](docs/add-skill-workflow.md) 操作**:8 个步骤从场景澄清到端到端验证,含四问法判定要不要做 shortcut、校验分层原则、反模式速查。

关键原则:文档先于代码、接口按需拉不预建、shortcut 用四问法卡住过度封装。

## 参考文档

- [`docs/add-skill-workflow.md`](docs/add-skill-workflow.md):新增 skill 的标准流程(操作手册)
- [`docs/interface-modeling-checklist.md`](docs/interface-modeling-checklist.md):接口建模前的能力调研 checklist(每次往 meta_data.json 加接口前必读)
- [`listen-cli-plan.md`](listen-cli-plan.md):项目雏形与决策记录(背景资料,OpenSpec change `listen-cli-mvp` 是权威)