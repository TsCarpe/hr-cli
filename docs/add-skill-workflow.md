# 新增 Skill 的标准流程

> 当你要把一个听评课场景做成 skill(让 AI 能通过 hr-cli 完成它)时,按本流程操作。
>
> 核心原则:**文档先于代码,接口按需拉不预建,shortcut 用四问法卡住过度封装,纯查询止步于 service。**

## 决策流(精简版)

```
Step 0 场景澄清
  ├─ 筛子 A:需要独立 skill 吗?
  │   ├─ 否(纯原子查询) → Step 3 加 service(+ hr-shared 术语表一行),流程结束
  │   └─ 是(多步编排/参数复杂/输出加工) ↓
  └─ 筛子 B:澄清触发词 + 编排
                    ↓
       Step 1 摸接口 ──→ Step 2 写 skill 文档 ──→ Step 3 拉接口进 meta_data.json
                                                      ↓                  ↓
                                                      ↓      列表接口必补 responseSchema(标 listKey + 常用字段)
                                                      ↓
                                                Step 4 dry-run 验证
                                                      ↓
                                           Step 5 四问法判定要不要 shortcut
                                                      ↓ yes
                                                 Step 6 写 shortcut
                                                      ↓
                                                 Step 7 端到端验证
                                                      ↓
                                           (编排型)Step 8 补 workflow 文档
```

---

## Step 0:场景澄清(必做,跳过会返工)

动任何代码前,先过两道筛子。

### 筛子 A:这个场景真的需要独立 skill 吗?(先问,大部分"新 skill"死在这里)

**核心判断:skill 的边界是编排边界,不是接口边界。** 一个接口对应一个 skill 文件是过度封装。判断标准 —— 出现以下任一信号才做独立 skill,否则只加 service:

| 信号 | 含义 | 例子 |
|------|------|------|
| 多步编排 | 「查 A → 用 A 的结果调 B」,AI 需要文档教怎么串 | 创建邀课(先查教师/量表/节次,再组装 body) |
| 参数复杂到需要智能默认 | 字段多 + 有合理默认值 + flag 拆解体验显著好于裸 `--data` | `course +create` |
| 输出需要 After 回调加工 | 原始返回对用户不友好,需要格式化成卡片 | 写操作回显邀请码 |

**都不满足 → 只加 service,不做 skill 文件。** 纯原子查询(单次 API 调用、参数简单、输出能用 `--jq` 解决)永远止步于 service + hr-shared 术语表一行映射。

判定流程:

```
出现上面任一信号?
├─ 否 → 只做 Step 3(加 service),在 hr-shared 术语表加一行映射,流程结束
└─ 是 → 继续筛子 B
```

**反模式**:见一个新接口就想建 skill 文件。每多一个 skill 文件 = 多一份要维护的真相源,接口字段变了要同步两处。hr-shared 术语表(触发词 → service.method)是**最轻的 skill 形态**,纯原子查询止步于此。

**产物**:明确"只加 service"或"做独立 skill"。选前者,流程到 Step 3 结束。

### 筛子 B:澄清触发词 + 编排

独立 skill 需要问清楚三件事:

1. **场景的触发词是什么**——用户会怎么描述这个需求?(例:「帮我查某老师的听课记录」「创建一个评课」)
2. **AI 完成这个场景需要哪些信息**——哪些是用户提供(终值 ID/名称),哪些是 CLI 能查到的?
3. **有没有跨步骤编排**——是单次 API 调用,还是要「查 A → 用 A 的结果调 B」?

**产物**:一句话场景定义 + 输入输出清单。这一步决定后面整个走向。

**反模式**:跳过这步直接拉接口,大概率拉了一堆用不上的,或者拉完了发现编排逻辑不对要重做。

---

## Step 1:摸底后端能力(只读,不改代码)

用 YApi MCP 看这个场景涉及哪些接口:

```
mcp__yapi-auto-mcp__yapi_search_apis   // 按关键词找
mcp__yapi-auto-mcp__yapi_get_api_desc  // 拉详情
```

判断每个接口属于哪类:

| 接口类型 | 处理方式 | 例子 |
|---------|---------|------|
| 原子查询 | 进 `meta_data.json`,元数据驱动 service 层就够 | 查教师、查量表、查节次 |
| 写操作 + 参数复杂 | 进 `meta_data.json` + 候选 shortcut | 创建邀课 |
| 不需要的接口 | **果断不拉**(YAGNI) | — |

**产物**:一份「这个场景需要的接口清单」+ 每个接口的归属(纯 service / shortcut 候选)。

---

## Step 2:写 skill 文档(在写 Go 代码之前)

**反直觉但关键:文档先于代码。** 原因——skill 文档逼你把「AI 该怎么编排」想清楚,会倒逼出真正的接口需求,经常能砍掉 Step 1 里多拉的接口。

参考 `skills/hr-invite/SKILL.md` 的结构:

- 核心概念(领域术语解释)
- Shortcuts 表
- API Resources(标注 read/write)
- **决策树**:什么时候用 shortcut,什么时候用原始 service
- **MVP 边界**:诚实标注哪些前置能力还没有(如「按学校名查 schoolId 不支持,需用户提供」)

新建文件:`skills/hr-<domain>/SKILL.md`。

**产物**:skill 文档草稿,含依赖接口清单 + MVP 边界标注。

---

## Step 3:拉接口进 meta_data.json

**只拉 Step 1 + Step 2 确认要用的。** 格式参照 `listen-cli-plan.md` Step 2 的规则:

- `basePath` 最后一段做 service 名(`/course/invite` → `course`)
- 剩余路径 `_` 连接做 method 名
- 枚举从 description 解析成 `enum` + `enumLabels`(格式 `select|选人 appointment|预约`)

**⚠️ 接口能力调研**:建模前必读 [`docs/interface-modeling-checklist.md`](interface-modeling-checklist.md)。字段是否生效要看 Business 层源码(Convert.toQuery + Service 实现),不是看 Java 注释或 validator profile。漏看 Business 层会误判接口定位和字段能力。

示例结构:

```json
{
  "name": "<service>",
  "title": "<中文标题>",
  "basePath": "/listen/v1/<path>",
  "methods": {
    "<method>": {
      "path": "/<sub-path>",
      "httpMethod": "POST",
      "description": "<命令描述>",
      "risk": "read|write",
      "requiresAuth": true,
      "requestSchema": { /* YApi JSON Schema 原样保留 */ },
      "responseSchema": { /* YApi 响应原样保留 */ }
    }
  }
}
```

**⚠️ 列表接口必须补 responseSchema(加 service 时同步想 jq)**:

判断方法是 `*_get_lists` / `*_lists` / 分页返回(`{pagination, <listKey>}`)的接口。这类接口 AI 几乎一定要用 `--jq` 投影,而投影的第一步是 `.<listKey>[]` —— 不知道 listKey 整个投影卡住。

要求:

1. **必填 `responseSchema`**,至少标出 `listKey` 和 `pagination`
2. **顶层 `description` 携带 listKey + 投影示例**(被 `schema` 命令打印成 response 区块第一行,AI 第一眼就拿到):
   ```json
   "responseSchema": {
     "type": "object",
     "description": "listKey=listenList;授课信息嵌在 courseInfo 下,投影示例 --jq '.listenList[] | {teacher: .courseInfo.teacherName, score: .commentScore}'",
     "properties": { ... }
   }
   ```
3. **listKey 字段的元素标常用投影字段**(4-8 个,AI 高频用的),不必全量。嵌套对象(如 `.courseInfo.teacherName`)必须标出嵌套层级,否则 AI 会写错 jq 路径
4. **`pagination.allCount`** 标注"命中总数,不受 pageSize 影响",让 AI 知道能用 `pageSize:1` 取巧

**反模式**:只加 requestSchema 不加 responseSchema。AI 第一次调用必须发真实请求 + `--jq 'keys'` 探查 listKey,浪费一次调用 + 一次 token 额度。元数据驱动的好处(编译期已知)被抵消。

**产物**:`internal/registry/meta_data.json` 新增 service/method(含 requestSchema;列表接口含 responseSchema)。

---

## Step 4:验证元数据(零代码)

```bash
make build
./bin/hr-cli schema <service>                  # 看方法列表
./bin/hr-cli schema <service>.<method>         # 看参数结构
./bin/hr-cli <service> <method> --data '{...}' --dry-run  # 预览请求
```

**这步只用元数据驱动的通用 service 命令,不动 Go 代码。** 如果 schema 显示不对,回 Step 3 修 JSON。

**产物**:确认所有原子接口 dry-run 通过,参数结构正确。

---

## Step 5:判断要不要做 shortcut(四问法)

**2 个以上答「是」才做 shortcut,否则只做 service。**

| 问题 | 考量 |
|------|------|
| 高频吗? | 用户/AI 会反复调,做 shortcut 省事 |
| 需要多个 API 组合吗? | 单 API 调用一般不需要 |
| 参数复杂、需要智能默认吗? | 字段多、有合理默认值,做 shortcut 体验好 |
| 输出需要加工吗? | 原始返回不友好,需要 After 回调格式化 |

参照已有决策:
- `course +create`:符合 1/3/4(高频、参数复杂、输出要回显邀请码)→ 做 shortcut
- `coursetag_get_lists` 等查询:只符合 0-1 个 → 只做 service

**产物**:明确「只做 service」或「做 service + shortcut」。

---

## Step 6:写 shortcut(Step 5 判定「需要」才做)

1. 新建 `shortcuts/<domain>/<action>.go`
2. 实现 `Shortcut` struct:
   - `Flags`:接受终值 ID(方式 2,不做交互式查询)
   - `BuildBody`:flag → body map(可加智能默认)
   - `After`:对返回结果格式化输出
3. 在 `shortcuts/register.go` 的 `All()` 里 append

参考 `shortcuts/course/course_create.go`。

**关键约束**:shortcut 必须通过 `RuntimeContext.runMethod` 闭包调 service 层,**禁止直调 client**(由 `shortcuts/common/runner.go` 的注入机制强制保证)。

**产物**:新增 shortcut 文件 + 注册。

---

## Step 7:端到端验证

```bash
# 真实 API 调用(需有效 token)
export LISTEN_TOKEN="eyJ..."
./bin/hr-cli <service> +<action> --school-id ... --teacher-id ...

# 或对话式(把 skill 配进 Claude Code,对 AI 说场景触发词)
```

**产物**:真实跑通 + 输出符合 skill 文档承诺。

---

## Step 8:补 workflow 文档(编排型场景才需要)

**判定**:Step 0 判定是「查 A → 用结果调 B」的多步场景才写。

新建 `skills/hr-<domain>/references/hr-<scenario>-workflow.md`,教 AI 怎么编排多步。参照已有的 `skills/hr-invite/references/hr-invite-create-workflow.md`。

单步场景不需要这步。

**产物**:workflow 文档(可选)。

---

## 校验分层原则

在整个流程中,记住错误处理的两层划分(详见 `CLAUDE.md`「校验分层」):

| 错误类型 | 校验位置 | 例子 |
|---------|---------|------|
| 协议/格式错误 | **CLI 本地拦** | `--data` 非 JSON、`--node` 非整数、必填 flag 缺失 |
| 业务规则错误 | **后端拦,CLI 透传** | node 范围、去重约束、权限、量表适用性 |

**反模式**:把业务规则(如 node 必须在 1-15)硬编码进 CLI 校验。后端改规则时 CLI 会错误拦截合法请求。后端消息不清晰时,**治本是让后端修消息**,不是 CLI 替后端擦屁股。

---

## 反模式速查

| 反模式 | 正确做法 |
|--------|---------|
| 见新接口就想建 skill 文件 | 纯原子查询只加 service + hr-shared 术语表一行;skill 边界 = 编排边界,不是接口边界 |
| 跳过 Step 0 直接拉接口 | 先过筛子 A/B,再决定拉哪些接口 |
| 先写代码再写 skill 文档 | 文档先于代码,倒逼接口需求 |
| 把整个领域的接口都拉了 | 只拉当前 skill 需要的(YAGNI) |
| 每个写操作都做 shortcut | 四问法卡住,2 个以上「是」才做 |
| shortcut 里直调 client | 通过 `runMethod` 闭包走 service 层 |
| CLI 里硬编码业务规则 | 业务规则留给后端,CLI 透传错误 |
| 给纯查询接口写 shortcut 代码 | 查询接口参数简单,`--jq` 已能解决输出;shortcut 代码层只在 Step 5 四问法过线才写(区别于上方的「skill 文件」层) |
| 列表接口只加 requestSchema | 列表接口必补 responseSchema:标 listKey + 常用投影字段 + 嵌套层级,否则 AI 要发真实调用探查 listKey |
