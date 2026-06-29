---
name: hr-shared
version: 0.1.0
description: "hr-cli 通用规则:认证机制、命令结构、输出格式、错误处理。所有领域 skill 开始前 MUST 先读本文件。"
---

# hr-cli 通用规则

**CRITICAL — 开始任何操作前 MUST 先用 Read 工具读本文件。** 本文件包含 hr-cli 的认证、命令结构、错误处理规则,所有领域 skill(如 hr-invite)都假定你已经读过了。

## hr-cli 是什么

hr-cli 是听评课(listen)系统的命令行封装,把 listen 的 REST API 暴露给 AI Agent 调用。基于 YApi 元数据自动生成命令,保证文档与 CLI 同步。

## 领域术语速查(用户说的话 → service.method)

**听到这些词时,对应调这个接口。参数细节用 `schema` 命令查。**

| 用户说的词 | service.method | 类型 | 说明 |
|----------|---------------|------|------|
| 邀课、创建评课邀请 | `course +create`(优先) / `course add` | write | 有 shortcut,flag 拆解参数 |
| 查教师、找老师 | `groupmanage +search-teacher` | read | shortcut,按姓名查 |
| 查课型 | `course coursetag_get_lists` | read | |
| 查评课量表 | `course coursepaper_get_suit_lists` | read | |
| 查授课节次 | `course course_node_get_lists` | read | |
| 我讲的课、我开的课、我被评的课 | `course course_get_lists` | read | 我作为授课老师的课(查 course_info 表) |
| 待评课、评课待办、我的待办 | `listen pending_get_lists` | read | 查我的待评课清单 |
| 我听的课、我评过的课 | `listen listen_get_lists` | read | 我作为评课人的记录,支持多维筛选 |

**边界提示**:`listen listen_get_lists` 和 `listen pending_get_lists` 都强制按当前用户过滤,不能查"某老师评的课"。查他人评课记录的通用接口(如 `getTeacherListenRecordPage`)暂未接入 CLI。

**查不到的术语**:用 `hr-cli schema` 列所有 service,或 `hr-cli schema <service>` 列 method,靠 method 名 + description 猜,然后 `schema <service>.<method>` 看参数。

### service 边界速记(按数据实体归属)

- **`course`**:查 `course_info` 表的接口(创建邀课 + 课型/量表/节次字典 + 我讲的课)
- **`listen`**:查 `listen_record` / `teach_pending` 表的接口(我听的课 + 待评课清单)
- **`groupmanage`**:教研组成员

## 三种命令

```bash
hr-cli <service> <method>           # service 命令(原始 API,1:1 映射)
hr-cli <service> +<verb>            # shortcut 命令(智能封装,优先用)
hr-cli schema <service[.method]>    # schema 查询(看参数结构)
```

**选择规则**:
- **有 shortcut 的操作,优先用 shortcut**(更易用、输出友好)
- 没有 shortcut 时,用 service 命令(`--data` 传 JSON body)
- 不确定参数时,先 `schema` 查

## 登录(前置门槛,BLOCKING)

**CRITICAL — 系统是登录后才能使用的。任何 listen API 都需要 hrToken,未登录会 401。Claude 处理任何业务请求前,MUST 先确认登录态。**

### 检查登录态(每次会话开始时)

```
方法 1: 看配置文件 ~/.hr-cli/config.json 是否存在且 hrToken 非空
方法 2: 直接调任意 read 接口,401 = 未登录
```

**反模式(严禁)**:配置文件不存在或字段不全时,**直接引导用户 `saas +login`,不要尝试用用户提供的任何 token 绕过登录流程去反查 schoolId/campusId**。这些 ID 是 saas 应用层信息,不在 JWT payload 里(只有 uc accountId),绕路只会浪费对话轮次。即使手上有 hrToken,没登录也只能走 `+login`。

### 未登录时的处理(BLOCKING REQUIREMENT)

**未登录时 MUST 先引导用户完成登录,不处理任何业务请求:**

```
你需要先登录才能使用本系统。请执行:
1. export SAAS_AUTH="<从 saas 系统 UI 拿的 Authorization header>"
2. hr-cli saas +login
登录成功后我再帮你处理 [用户原请求]。
```

详见 [`hr-auth/SKILL.md`](hr-auth/SKILL.md)。

### 登录后

`saas +login` 成功会把 hrToken 写入 `~/.hr-cli/config.json`,后续所有命令自动复用,Claude 无需再管 token。

---

## 认证细节

**所有接口(read 和 write)都需要 hrToken(JWT),不传 token 会 401。**

### token 来源(优先级从高到低)

1. `--token "xxx"` flag
2. `LISTEN_TOKEN` 环境变量
3. `~/.hr-cli/config.json`(由 `hr-cli saas +login` 自动写入)

### token 怎么拿(推荐:自动登录)

**CRITICAL — 用户需要 token 时,优先推荐自动登录,不要让用户手动复制。**

```bash
# 1. 用户先 export saas token(从 saas 系统 UI 拿,一次性)
export SAAS_AUTH="xxx"

# 2. 一条命令自动换 hrToken 并持久化到 ~/.hr-cli/config.json
hr-cli saas +login
# 之后所有命令自动读 config.json,无需再传 --token
```

详见 [`hr-auth/SKILL.md`](hr-auth/SKILL.md)。只有自动登录不可用时(如 saas-auth 服务故障),才退回手动方式:从听评课 Web 系统登录后,浏览器 devtools → Network → 任意请求的 hrToken header 复制。

### token 安全

- **禁止**在对话里向用户展示完整 token(hrToken 或 SAAS_AUTH 都不能展示)
- **禁止**把 token 写进日志或返回给用户
- 引导用户自己设置:`export LISTEN_TOKEN="xxx"` 或 `--token "xxx"`
- `saas +login` 成功后**只展示账号名/学校名**,不展示 hrToken 本身(已持久化)

## 输出格式

| 命令类型 | 输出 |
|---------|------|
| service 命令 | 原始 JSON(listen ResultJson.data 部分) |
| shortcut 命令(读) | 友好区 + `[internal]` 区(见下) |
| shortcut 命令(写) | 友好卡片(只含用户能理解的字段,如"邀请码",不含技术 ID) |
| 错误 | stderr + 非零 exit code |

### shortcut 输出的 `[internal]` 区(读类 shortcut 才有)

**这是给 Claude 用、不给用户看的数据**。结构:

```
找到 1 位匹配 "王佳" 的教师:

教研组: 高中历史
  - 王佳

[internal]
{"teachGroupList":[{"staffId":"...","staffName":"王佳","teachGroupId":"...","teachGroupName":"高中历史"}]}
```

**规则**:
- 友好区(上半):人类可读,**转述给用户时用这部分**
- `[internal]` 区(下半):含 ID,供 Claude 后续调用 write 操作(如 `+create` 需 staffId/teachGroupId)
- **CRITICAL — 转述给用户时 MUST 省略 `[internal]` 区全部内容**

**为什么这样设计**:CLI 是一次性进程,没有跨调用状态。Claude 取 ID 的唯一通道是 stdout。分区让"人看的话术"和"程序用的 ID"共存于一次输出,Claude 自己区分对待。

### 列表查询与 `--jq`(`*_get_lists` 接口通用 SOP)

列表接口(`listen_get_lists` / `course_get_lists` / `pending_get_lists` 等)响应大、嵌套深,几乎一定要用 `--jq` 投影。按这个 SOP 走,不靠记忆、不靠猜:

```
Step 1  schema <service>.<method>
        → 看必填字段(requestSchema)、可选筛选字段、responseSchema

Step 2  从 responseSchema 拿 listKey(响应里放列表的顶层字段名)
        ├─ responseSchema 有(标了 listKey=xxx)→ 直接用
        └─ responseSchema 没有              → 第一次调用加 --jq 'keys' 探顶层 key

Step 3  按需选择调用方式
        ├─ 只关心总数   → pageSize:1 + --jq '.pagination.allCount'
        ├─ 看完整列表   → 套默认投影模板(下方)
        └─ 排查问题     → 不加 --jq,看原始响应

Step 4  筛选字段从 schema 的 requestSchema.properties 读
        不要凭记忆写字段名,后端改了字段 CLI 不会知道
```

**默认投影模板**(Step 3 的"看完整列表"套这个,字段映射按场景调整):
```bash
hr-cli <service> <method> --data '{...}' \
  --jq '{total: .pagination.allCount, items: [.<listKey>[] | {<字段映射>}]}'

# 实例(我听的课):嵌套字段走 .courseInfo.xxx
hr-cli listen listen_get_lists --data '{...}' \
  --jq '{total: .pagination.allCount, items: [.listenList[] | {teacher: .courseInfo.teacherName, course: .courseInfo.name, score: .commentScore}]}'
```
- `total` 留分页总数,让用户知道"还有多少页"
- `items` 投影成精简对象数组,字段做**语义化映射**(如 `fromType=="invite"` → `"邀课"`),让 AI 不用再查映射表
- 嵌套字段注意层级:`listen_get_lists` 的 teacher 在 `.courseInfo.teacherName`,`course_get_lists` 在 `.teacherName`(扁平)—— 从 responseSchema 看嵌套,别猜。**responseSchema 的填写规范**(listKey 标注、字段层级等)见 [`docs/add-skill-workflow.md`](../../docs/add-skill-workflow.md) Step 3,本文档只讲"怎么用"

**何时必须用 `--jq`**(满足任一):
- 响应 list 长度 > 5,或单条字段数 > 15
- 嵌套深 > 3 层(`.a.b.c.d` 才能拿到目标值)
- 只关心 1-2 个字段

**反例(不该用 jq)**:
- 创建/写操作(响应通常就一个 ID/状态)
- 单值查询(如 `schema` 命令、查字典的 `coursetag_get_lists` 返回少)
- 调试/排查问题时

**`--jq` 行为**:
- 多结果 → NDJSON(每行一个 JSON),便于 AI 逐行解析
- 单结果 → 缩进美化输出
- 空结果 → 静默 exit 0(脚本管道友好)
- 语法错误 → stderr + 非零 exit code

**为什么不直接全量返回**:LLM 推理成本 ≈ O(token²)(attention 是二次方),响应从 4KB → 200B 不仅省时间,还**显著降低模型幻觉**(无关字段干扰少)。这是 hr-cli 对 AI 友好的核心设计。

**反模式**:跳过 Step 1 直接凭记忆写 `--data`,字段名拼错 → 后端 400,白浪费一次调用。

## 错误处理

**CRITICAL — hr-cli 出错时,exit code 非零,stderr 有 Error 消息。根据消息类型决定处理方式:**

| 错误模式 | 含义 | 处理建议 |
|---------|------|---------|
| `HTTP 5xx` / `服务器错误` | listen 服务异常 | 告诉用户"系统暂时不可用,稍后重试" |
| `业务失败(status=401)` | token 无效或过期 | **提示用户重新获取 token**:从听评课 Web 系统重新登录拷贝 hrToken |
| `业务失败(status=400): <字段> 不能为空` | 缺必填字段 | 根据 message 补字段后重试 |
| `业务失败(status=400): 当前课程已存在` | 业务去重 | 改 `--time` 或 `--node` 避开冲突 |
| `<字段> 格式非法` | shortcut 输入校验 | 按提示修正(如 --node 要数字) |
| 网络错误 / dial tcp | 网络不通或 --base-url 错 | 检查 `--base-url`,确认能访问 |

**禁止**:看到错误后盲目重试相同命令(尤其是 400/业务错误,重试不会成功)。

## 全局 flag

所有命令都支持这些 flag(可放在任意位置):

- `--token` — 认证 token
- `--base-url` — listen 服务地址(**可选**,见下方解析链)
- `--format` — 输出格式(默认 `json`)
- `--jq` — jq 表达式过滤(对 service 命令的 JSON 输出过滤,见下方「`--jq` 过滤」章节)

### `--base-url` 解析链(从高到低)

1. `--base-url` flag(本次调用显式传)
2. `HR_CLI_BASE_URL` 环境变量
3. **`<repo>/.hr-cli.json`**(项目配置,跟登录态分离;模板见 `.hr-cli.json.example`)
4. 默认值 `http://localhost:8080`(本地开发)

**推荐做法**:clone 项目后 `cp .hr-cli.json.example .hr-cli.json`,把测试环境地址写进去,以后所有命令自动复用,不必每次传 `--base-url`。

`.hr-cli.json` 已加入 `.gitignore`(每个环境的地址不同,不 commit)。

## 服务地址速查

| 环境 | base-url |
|------|----------|
| 本地开发 | `http://localhost:8080` |
| 测试环境 | `https://hrjy-test.hailiangedu.com/hr` |
| 生产环境 | (待补充) |

**注意**:`/hr` 是网关前缀,不要省略。
