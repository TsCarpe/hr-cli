# hr-cli:听评课业务 Skill 化实施方案

> **文档说明**:本文件是项目雏形与决策记录,可能与最终实施有差异。OpenSpec change `listen-cli-mvp` 是权威的变更追踪,本文件作为背景资料保留。
>
> **已确认的修订**(2026-06-16):
> - 项目位置改为现有 hr-cli 仓库(`/Users/hljy/IdeaProjects/hr-cli`,module `hr-cli`),不新建 listen-cli 独立项目
> - cmd 包位置:`cmd/`(标准 cobra 习惯),不是 `internal/cmd/`
> - `get_lists` 接口**后端实际不存在**,从 MVP 砍掉,所有列表查询移到后续扩展
> - ResultJson 路径已确认:`api/src/main/java/com/hailiang/hr/listen/dto/ResultJson.java`
> - 认证 header 是 `hrToken`(小写 h,大写 T),从 `LoginHandlerInterceptor.java` L55 确认
> - `/listen` 是 Spring `server.servlet.context-path` 配置,listen 服务本身只认 `/v1/...`,但通过反向代理访问时 URL 是 `/listen/v1/...`
> - Shortcut struct 简化为 `BuildBody` + `After` 两回调,DryRun 与 schema required 校验由 service 层兜底
> - 入参策略:**MVP 方式 2**——`+create` 接受终值 ID,前置查询能力推后,skill 文档诚实标注依赖
> - workflow 文档归属:领域 skill(方案 A),MVP 不写 workflow,扩展阶段补
>
> **M5 实战发现**(2026-06-18):
> - 网关前缀 `/hr`:测试环境完整 URL 是 `https://hrjy-test.hailiangedu.com/hr/listen/v1/...`,由 `--base-url` 传 `https://hrjy-test.hailiangedu.com/hr` 处理(不要塞进 basePath,生产环境可能无 /hr)
> - YApi schema 漏字段:邀课 add 实际需要 26 字段(YApi 只标 21),漏了 `teachPlanId`(空)、`nodeDisplayName`(显示用)、`commentPaperName`(显示用)、`saasSchoolId`(=schoolId)、`saasCampusId`(=campusId),已在 meta_data.json 补齐
> - 业务校验带后端消息:listen 在 status=400 时返回字段级错误(`campusId 不能为空,teacherName 不能为空...`),client.go 已改为带 message 的 error
> - 业务去重规则:同 teacherId + date + node 不能重复创建,后端返回 `当前课程已存在,请检查授课教师、日期和节次`(skill 文档应捕获此规则)
> - 额外 header(tenantId/schoolId/campusId/staffId)非必填,JWT 已含这些信息,client 只发 hrToken 即可
> - cobra `SilenceUsage: true` + `SilenceErrors: true`:让 main 统一控制错误输出,避免重复
>
> **M6 范围调整**(2026-06-18,路径 3):
> - **关键洞察**:`+create` 的 flag 版对"人"几乎无用(所有 ID 用户都不知道),对"AI"才有价值(简化 JSON 构造 + 输出友好)。MVP 阶段 +create 的真实用户是 AI(Claude Code),不是人。
> - **"无参数/对话式"创建邀课的真正机制**:不是 hr-cli 解析自然语言,而是 Claude 读 skill 文档后编排多步(查教师→查课程→+create)。hr-cli 只提供"原子能力"(查询 + 创建),智能在编排层。
> - **路径 3 决策**:M6 按计划做 +create(承认 AI 友好定位),M8 改做**最有价值的查询接口**(teacher search、course-tag list、comment-paper list 等),不做"批量拉所有邀课接口"。MVP 结束时真正能端到端对话式创建邀课。
> - **M8 范围调整**:从"批量拉邀课领域其余 8 个接口"改为"拉查询领域关键接口"。原 M8 推到 MVP 之后。

## Context

**问题**:listen 是一个 Java/Spring 听评课系统,业务能力只通过 Web UI 暴露,无法被 AI Agent 直接调用。内部运营人员和教师都需要通过 AI 完成听评课相关操作(创建邀课、查听课记录、管理评课等)。

**目标**:在现有 hr-cli 仓库内,参考 `/Users/hljy/IdeaProjects/cli`(lark-cli)的架构,把听评课业务封装成:
- **hr-cli**(Go CLI 工具):元数据驱动,自动封装 listen REST API
- **skills/**(Markdown 文档):教 AI 什么时候用什么命令

**为什么是元数据驱动**:YApi(项目 59 "鸿儒教研迭代api")已存储完整的 JSON Schema(参数、类型、required、枚举、嵌套结构),质量已验证(邀课 add 接口 20+ 字段完整描述)。YApi 是接口文档的 single source of truth,基于它生成 CLI 命令,能保证文档和 CLI 天然同步。文档缺失时用 YApi MCP 补充。

**双场景支持**:
- 内部运营:本地 Claude Code + hr-cli,直连 listen API
- 教师端:服务端 AI Agent 调 hr-cli(同一二进制跑在服务器)

**认证外部化**:JWT token 由另一个 skill/项目注入,hr-cli 只读取。

---

## 已确认的决策清单

| 决策项 | 结论 |
|---|---|
| 项目形态 | 在现有 hr-cli 仓库内实施(module `hr-cli`),不新建独立项目 |
| 位置 | `/Users/hljy/IdeaProjects/hr-cli` |
| 语言 | Go(同 lark-cli,最大化复用脚手架) |
| 仓库结构 | 单仓库(CLI + skills 一起) |
| 架构 | 三层:shortcut(service(client)),**禁止 shortcut 直调 client** |
| cmd 包位置 | `cmd/`(标准 cobra 习惯) |
| service 层实现 | **元数据驱动**(基于 YApi JSON Schema) |
| 元数据源 | YApi MCP 手动拉取(不写 fetch_meta 脚本) |
| MVP 领域 | 邀课(invite),只做 PC 端 `/v1/course/invite/*` |
| MVP service 方法 | 只 `course.add` 一个,验证元数据驱动 |
| MVP shortcut | 只做"+create 创建邀课"1 个,验证模式 |
| 入参策略 | **方式 2**——shortcut 接受终值 ID,前置查询推后,skill 诚实标注 |
| Shortcut struct | 简化为 `BuildBody` + `After` 两回调 |
| workflow 归属 | 领域 skill(`skills/listen-invite/references/`),MVP 不写,扩展阶段补 |
| 认证 header | `hrToken`(小写 h,大写 T) |
| 认证解析链 | `--token` flag > `LISTEN_TOKEN` env > 配置文件 |
| MVP token 策略 | 手动从浏览器拷贝,不实现 JWT 签发 |
| ResultJson 结构 | `{status int, message string, data Object, success Boolean}` |
| `/listen` 前缀 | Spring context-path,反代场景下 URL 是 `/listen/v1/...` |
| mobile 接口 | MVP 不做,后续扩展 |

---

## 架构总览

```
┌──────────────────────────────────────────────────────────────┐
│  Skill 层 (skills/*.md)                                       │
│  教 AI 什么时候用什么命令                                      │
│  listen-shared/SKILL.md     通用规则(认证、错误处理)          │
│  listen-invite/SKILL.md     邀课领域 + references/            │
└────────────────────────┬─────────────────────────────────────┘
                         │ AI 根据意图选择调用
                         ▼
┌──────────────────────────────────────────────────────────────┐
│  Shortcut 层 (shortcuts/invite/+create)                       │
│  高频操作的高级封装,手写                                      │
│  BuildBody:组装 flag → body map                              │
│  After:对返回结果做格式化                                     │
│  调用 service 层 RunMethod("course", "add", body)            │
└────────────────────────┬─────────────────────────────────────┘
                         │ shortcut 调用 service 层(不直调 client)
                         ▼
┌──────────────────────────────────────────────────────────────┐
│  Service 层 (元数据驱动,自动生成)                              │
│  1:1 映射 listen REST API                                     │
│  meta_data.json ← YApi MCP 手动拉取转换                       │
│  cmd/service/service.go 读元数据自动生成 cobra 命令           │
│  统一兜底:--dry-run 预览、schema required 校验                │
└────────────────────────┬─────────────────────────────────────┘
                         │ service 层调 client
                         ▼
┌──────────────────────────────────────────────────────────────┐
│  Client 层 (internal/client/)                                 │
│  net/http + hrToken header 注入                               │
│  解析 ResultJson,提取 data                                   │
└──────────────────────────────────────────────────────────────┘
```

---

## 目录结构(hr-cli 仓库内)

```
hr-cli/
├── cmd/                             # 命令树(标准 cobra 习惯)
│   ├── main.go                      # 入口 → 调 internal/cmd.NewRoot()  [已存在]
│   ├── root.go                      # 根命令 + 全局 flag
│   ├── build.go                     # 命令树组装
│   ├── global_flags.go              # --token, --base-url, --format, --jq
│   ├── version.go                   # version 命令  [已存在,需迁移]
│   ├── service/
│   │   └── service.go               # [核心] 元数据驱动命令生成
│   └── schema/
│       └── schema.go                # schema 查询命令
│
├── internal/
│   ├── registry/                    # [核心] 元数据
│   │   ├── loader.go                # 元数据加载 (go:embed)
│   │   ├── meta_data.json           # [核心] 从 YApi 生成的 API 元数据(进 git)
│   │   └── types.go                 # 元数据 Go 类型定义
│   ├── client/
│   │   ├── client.go                # ListenClient: net/http + hrToken 注入
│   │   └── response.go              # 解析 ResultJson, 提取 data
│   ├── output/                      # 输出格式化(从 lark-cli 精简)
│   │   ├── format.go                # JSON/Table
│   │   └── jq.go                    # jq 过滤
│   ├── cmdutil/
│   │   ├── iostreams.go             # IO 流
│   │   └── json.go                  # JSON 解析(--data)
│   └── config/
│       └── config.go                # token 解析链
│
├── shortcuts/
│   ├── register.go                  # 注册所有 shortcut
│   ├── common/                      # Shortcut 框架(从 lark-cli 适配)
│   │   ├── types.go                 # Shortcut struct(简化版)
│   │   └── runner.go                # RuntimeContext
│   └── invite/
│       ├── shortcuts.go             # Shortcuts() 注册
│       └── invite_create.go         # +create
│
├── skills/
│   ├── listen-shared/
│   │   └── SKILL.md                 # 通用规则
│   └── listen-invite/
│       ├── SKILL.md                 # 邀课领域 skill
│       └── references/
│           └── listen-invite-create.md   # MVP 只交付这个
│                                          # listen-invite-create-workflow.md
│                                          # 留扩展阶段(前置查询就绪后)
│
└── docs/
    └── meta-format.md               # 元数据格式说明
```

**迁移说明**:hr-cli 现有的 `internal/cmd/root.go` `internal/cmd/version.go` 在 M1 阶段迁移到 `cmd/`。`internal/handler/model/repository/service` 这些空目录(Spring MVC 习惯命名)在 M1 阶段清理。

---

## 实施步骤

### Step 1: 项目脚手架改造

**前提**:hr-cli 已有 `cmd/main.go`(调用 `internal/cmd.NewRoot()`)+ `internal/cmd/{root,version}.go`,cobra 已装。

1. 迁移 `internal/cmd/{root,version}.go` → `cmd/{root,version}.go`,更新 import 路径
2. 清理 `internal/{handler,model,repository,service}` 空目录
3. 改造 `cmd/root.go`:注册全局 flag(`--token`、`--base-url`、`--format`、`--jq`)
4. 从 lark-cli 复制并适配脚手架:
   - `internal/output/`(格式化,原样复制,移除飞书错误码)
   - `internal/cmdutil/iostreams.go`、`json.go`(原样复制)
   - `Makefile`(改 `BINARY := hr-cli`)
5. **验证点**:`go build` 成功,`./hr-cli --help` 输出帮助,`./hr-cli version` 正常

**参考文件**:
- `/Users/hljy/IdeaProjects/cli/cmd/root.go`(根命令模式)
- `/Users/hljy/IdeaProjects/cli/Makefile`(构建脚本)

### Step 2: 元数据生成(手动从 YApi 拉)

1. 用 YApi MCP 拉取邀课 PC 端接口的定义(分类 1019 "pc端听评课重构" 里 path 含 `/course/invite/` 的接口):
   - `mcp__yapi-auto-mcp__yapi_get_api_desc` 逐个拉取
   - **MVP 阶段只拉 `add` 一个接口**,验证全链路;批量拉取留 M8
   - **注意:`get_lists` 接口不存在**(邀课列表查询不在 `CourseInviteController`),不要去找
2. 邀课领域实际可用的 10 个接口(来自 `CourseInviteApi.java`):

| method 名 | path | 需登录 |
|---|---|---|
| add | `/v1/course/invite/add` | ✅ |
| edit | `/v1/course/invite/edit` | ✅ |
| paper_edit | `/v1/course/invite/paper_edit` | ✅ |
| edit_get | `/v1/course/invite/edit/get` | ✅ |
| image_edit | `/v1/course/invite/image/edit` | ✅ |
| appointment_add | `/v1/course/invite/appointment/add` | ✅ |
| appointment_del | `/v1/course/invite/appointment/del` | ✅ |
| school_get_row | `/v1/invite_num/get_row` | ❌ |
| detail_get_row | `/v1/course/invite/detail/invite_num/get_row` | ✅ |

3. 手动转换为 `internal/registry/meta_data.json`,格式如下:

```json
{
  "version": "2026-06-16",
  "services": [
    {
      "name": "course",
      "title": "课程邀请",
      "basePath": "/listen/v1/course/invite",
      "methods": {
        "add": {
          "path": "/add",
          "httpMethod": "POST",
          "description": "添加听评课邀请",
          "risk": "write",
          "requestSchema": { /* YApi 的 Json 参数原样保留 */ },
          "responseSchema": { /* YApi 的 响应内容原样保留 */ }
        }
      }
    }
  ]
}
```

4. 命名规则:`basePath` 最后一段作 service 名(`/course/invite` → `course`),剩余路径用 `_` 连接作 method 名
5. 枚举处理:从 description 里解析 `select|选人 appointment|预约` 格式,提取为 `enum` + `enumLabels`
6. **MVP `meta_data.json` 只含 course.add 一条真实数据**,但 schema 设计要能容纳上述 9 个接口

### Step 3: 元数据引擎实现

1. `internal/registry/types.go`:定义 Go 结构体(Service/Method/Schema)
2. `internal/registry/loader.go`:用 `//go:embed meta_data.json` 加载元数据
3. `cmd/service/service.go`:核心命令生成引擎
   - 遍历 `services`,为每个 service 创建 cobra 子命令(`course`)
   - 遍历 `methods`,为每个 method 创建子命令(`course add`)
   - 统一注册 flag:`--data`(请求体 JSON)、`--format`、`--jq`、`--dry-run`
   - RunE:解析 `--data` → 校验 required → 调 HTTP → 解析 ResultJson → 输出
4. `cmd/schema/schema.go`:`hr-cli schema course.add` 展示参数结构(递归打印嵌套 schema、required 标注、枚举值)

**参考文件**(lark-cli 的对应实现):
- `/Users/hljy/IdeaProjects/cli/cmd/service/service.go`(`RegisterServiceCommands`、`buildMethodCommand`、`serviceMethodRun`)
- `/Users/hljy/IdeaProjects/cli/internal/registry/loader.go`(embed 机制)

### Step 4: HTTP Client 与认证

1. `internal/client/client.go`:`ListenClient` 结构
   - `Do(ctx, method, path, body)`:标准 `net/http`,自动注入 `hrToken` header(注意大小写)和 `Content-Type: application/json`
2. `internal/client/response.go`:解析 listen 统一返回结构

```go
type ResultJson struct {
    Status  int             `json:"status"`   // 200=成功, 400=异常, 401=未登录
    Message string          `json:"message"`
    Data    json.RawMessage `json:"data"`
    Success bool            `json:"success"`
}
```

   - HTTP 错误 → 网络错误;`status==401` → 认证错误;`!success` → 业务错误
   - 成功时提取 `data` 部分,按 `--format`/`--jq` 输出
3. `internal/config/config.go`:token 解析链 `--token` > `LISTEN_TOKEN` env > `~/.hr-cli/config.yaml`

**参考文件**(均已确认):
- listen 的 ResultJson:`/Users/hljy/IdeaProjects/listen/api/src/main/java/com/hailiang/hr/listen/dto/ResultJson.java`
- listen 的认证拦截器:`/Users/hljy/IdeaProjects/listen/web/src/main/java/com/hailiang/hr/listen/handler/LoginHandlerInterceptor.java`(L55: `request.getHeader("hrToken")`)
- lark-cli client:`/Users/hljy/IdeaProjects/cli/internal/client/client.go`(参考结构,但去掉飞书 SDK)

### Step 5: Shortcut 框架适配(简化版)

1. `shortcuts/common/types.go`:Shortcut struct(从 lark-cli 简化)

```go
type Shortcut struct {
    Service     string
    Command     string   // "+create"
    Description string
    Risk        string   // "read" | "write"
    Flags       []Flag
    BuildBody   func(ctx, *RuntimeContext) (map[string]any, error)  // 组装 body
    After       func(ctx, *RuntimeContext, result map[string]any) error  // 输出加工
}
```

**说明**:相比 lark-cli 删掉 `DryRun` / `Validate` / `Scopes` / `AuthTypes`。
- DryRun:由 service 层统一兜底
- Validate:schema required 校验由 service 层兜底;跨字段业务校验按需在 BuildBody 里做
- Scopes/AuthTypes:listen 不用飞书 scope 体系

2. `shortcuts/common/runner.go`:RuntimeContext
   - `RunMethod(service, method, body)`:调 service 层,返回 data(map)
   - `Str(name)`/`Int(name)`:读取 flag 值
3. `shortcuts/invite/invite_create.go`:实现 `+create`
   - Flags:school-id、campus-id、teacher-name、course-name、time、node、listen-type、comment-paper-id 等
   - BuildBody:组装 body
   - After:格式化输出(回显 courseInviteId + courseInviteCode)
   - **入参策略**:不做交互式补全,缺失参数返回带 hint 的错误,交给 AI 通过 skill 文档引导(方式 2)

**参考文件**:
- `/Users/hljy/IdeaProjects/cli/shortcuts/common/types.go`(精简参考)
- `/Users/hljy/IdeaProjects/cli/shortcuts/common/runner.go`
- `/Users/hljy/IdeaProjects/cli/shortcuts/calendar/calendar_create.go`(BuildBody/After 模式参考)

### Step 6: Skill 文档

1. `skills/listen-shared/SKILL.md`:认证方式、通用 flag、错误码、命令结构说明
2. `skills/listen-invite/SKILL.md`:
   - 核心概念(课程邀请、听课方式、邀请码、评课量表)
   - Shortcuts 表(目前只有 `+create`)
   - API Resources 列表(所有 service method,标注 read/write)
   - 决策树:什么时候用 `+create`,什么时候用 `course add --data`
3. `skills/listen-invite/references/listen-invite-create.md`:`+create` 的参数表、使用示例、**诚实标注 MVP 阶段的依赖边界**(school-id 等参数需要用户提供,查询能力后续补齐)
4. **MVP 不写** `listen-invite-create-workflow.md`,留扩展阶段(前置查询接口就绪后)

**参考文件**:
- `/Users/hljy/IdeaProjects/cli/skills/lark-calendar/SKILL.md`(结构、BLOCKING REQUIREMENT 写法)
- `/Users/hljy/IdeaProjects/cli/skills/lark-shared/`(共享文档模式)

---

## 关键设计要点

### 元数据同步机制

```
YApi 接口变更
    ↓
开发者用 YApi MCP 重新拉取接口定义
    ↓
手动更新 internal/registry/meta_data.json (git diff 可审查)
    ↓
make build 重新编译, embed 新元数据
```

- `meta_data.json` **进 git**,是编译时 embed 的数据源
- 顶层 `version` 字段记录同步时间
- `hr-cli version` 输出元数据版本便于排查

### shortcut vs service 的选择标准(四问法)

四个问题里 2 个以上答"是"才做 shortcut,否则只做 service:
1. 高频吗?
2. 需要多个 API 组合吗?
3. 参数复杂、需要智能默认吗?
4. 输出需要加工吗?

MVP 的 `+create` 符合 1/3/4(高频、参数复杂、输出要回显邀请码),所以做成 shortcut。其余邀课能力只做 service。

### 入参策略(方式 2)

`+create` 的 flag 都是终值(school-id、course-id 等),不做交互式查询。
- 缺失参数 → 返回带 hint 的错误
- AI 根据 skill 文档,询问用户提供 ID,或在上下文中已知 ID 时直接调用
- 前置查询能力(school list / course list 等)推到扩展阶段,届时补充 workflow 文档

参考 lark-cli `calendar +create` 的 `--attendee-ids`:接受 `ou_xxx` 终值,查询能力由 contact skill 负责。

### 枚举值处理

从 YApi description 里的 `select|选人 appointment|预约 other|其他` 格式正则解析,转成:
```json
{
  "type": "string",
  "enum": ["select", "appointment", "other"],
  "enumLabels": ["选人", "预约", "其他"]
}
```

schema 查询时展示成:
```
listenType string [required] — 听课方式
  - select: 选人
  - appointment: 预约
  - other: 其他
```

---

## 后续扩展路径

每加一个领域(如听课记录)的标准步骤:

1. 用 YApi MCP 拉取该领域接口定义
2. 手动转换,追加到 `meta_data.json` 的 `services` 数组
3. (可选)为高频操作写 shortcut
4. 写 `skills/listen-<domain>/SKILL.md`
5. `make build && 验证`
6. (可选)为跨领域流程写 workflow 文档,放在主领域 skill 的 references/

扩展路线图:
- MVP:邀课(course),只 course.add + +create
- 第二期:邀课领域补齐(9 个接口)、前置查询领域(school/course/paper/teacher 等)、邀课 workflow 文档
- 第三期:听课记录(record)、评课(comment)
- 第四期:课程(course)、教研活动(activity)

---

## 验证方案

### 编译与元数据验证
```bash
make build
./hr-cli schema                    # 列出服务
./hr-cli schema course             # 列出邀课方法
./hr-cli schema course.add         # 查看 add 参数结构(含 required、枚举)
```

### Dry-run(不发真实请求)
```bash
./hr-cli course add --token "test" --data '{"schoolId":"1"}' --dry-run
# 预期:打印 POST /listen/v1/course/invite/add + 请求体 + headers
```

### 真实 API 调用(需有效 token)
```bash
export LISTEN_TOKEN="eyJ..."
./hr-cli course add --data '{...完整 add 参数...}'
# 预期:返回 ResultJson.data 里的 courseInviteId + courseInviteCode

./hr-cli course +create \
  --school-id "1001" --campus-id "2001" \
  --teacher-name "测试" --course-name "测试课程" \
  --time "2026-06-20" --node 3 \
  --listen-type select --comment-paper-id "p1" \
  --course-tag-id "t1" --class-name "高一班" \
  --addr "A301" --teach-group-id "g1" \
  --member-ids "u1,u2"
# 预期:返回 courseInviteId + courseInviteCode
```

### 错误场景
```bash
./hr-cli course add --data '{}'              # 预期:报告缺失 required 字段
./hr-cli course add --token "invalid" --data '{...}'  # 预期:401 认证失败
```

### AI 集成验证
- 把 `skills/listen-invite/` 配置进 Claude Code
- 对 AI 说"帮我创建一个邀课",验证 AI 能正确调用 `+create`(参数齐时)或 `course add --data`(需精细控制时)
