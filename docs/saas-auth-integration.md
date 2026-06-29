# saas 登录接入 hr-cli 设计方案

> 状态:已与用户确认设计,待实施
> 日期:2026-06-26
> 关联:lesson 项目(`/Users/hljy/IdeaProjects/lesson`)、hr-cli saas-auth 服务集成

## Context

**现状**:hr-cli 调 listen API 全靠用户手动从浏览器 devtools 复制 `hrToken`,体验对 AI Agent 不友好。

**目标**:在 hr-cli 里新增 saas 登录能力,让 AI Agent 用一个 `SAAS_AUTH` env 变量,全自动完成「换 userId → 查学校校区 → login 拿 hrToken → 持久化复用」全流程。

---

## 核心突破:userId 问题已解决(深度探索结论)

### 完整事实链(全部已验证)

| 事实 | 来源 |
|------|------|
| saas-auth 服务测试环境 URL:`http://10.30.5.53:31759` | `lesson/web/.../config-cache/LESSON+hr-test+application.properties` 的 `open.url` |
| tokenAuth 完整 URL:`${open.url}/saas-auth/sso/channel/token/auth` | `LoginApi.java` 的 `@RequestMapping("/saas-auth")` + `Constant.TOKEN_AUTH_URL = "/sso/channel/token/auth"` |
| 请求体:`{"token": "<Authorization>"}` | `TokenAuthRequest.java` 只有 `token` 字段(`@NotBlank`) |
| 返回:`LoginAuthVO.authenticationResource.ucUserInfoVO.id` = userId | `UserInfoConvert.java:21` `userInfo.setUserId(String.valueOf(authenticationResource.getUcUserInfoVO().getId()))` |
| `getSaasInfo` 内部 `doLogin` **只用 authorization**,其他字段全忽略 | `SaasServiceImpl.java:153-162` 只调 `saasClient.doLogin(requestLoginDto.getAuthorization())` |

### 关键洞察

**lesson 的 `/v1/saas/login` 要求 schoolId 是 lesson 的设计,但底层 saas-auth 服务的 tokenAuth 根本不需要 schoolId**——它只用 Authorization 就能解出 userId。

**真正可走的全自动链路**(hr-cli 直接调 saas-auth,不经过 lesson 的 login):
```
SAAS_AUTH env
    ↓ POST http://10.30.5.53:31759/saas-auth/sso/channel/token/auth
    ↓ body: {"token": "<SAAS_AUTH>"}
拿到 userId + accountName
    ↓ POST lesson /v1/saas/app_school_campus_get_lists
    ↓ body: {"userId": "<userId>", "saasAppId": "<appId>"}
拿到 schools[] + campus[] 列表(含 schoolId/staffId/tenantId)
    ↓ 用户选择(或自动选 recentLogin=true 的)
    ↓ POST lesson /v1/saas/login
    ↓ body: {staffId, tenantId, schoolId, campusId, authorization: SAAS_AUTH}
拿到 hrToken
    ↓ 写入 ~/.hr-cli/config.json
后续所有 listen 命令自动复用 hrToken
```

### lesson 接口字段(已验证)

**`POST /v1/saas/login`(SaasReq)**

| 字段 | 必填 | 说明 |
|------|------|------|
| authorization | ✅ | saas token(从 SAAS_AUTH env) |
| staffId | ✅ | 从 app_school_campus_get_lists 返回 |
| tenantId | ✅ | 同上 |
| schoolId | ✅ | 同上 |
| campusId | ❌ | 同上(可选) |

返回 `LoginRespDto`:`{hrToken, hrUserInfo:{accountName, schoolName, campusName, ...}}`

**`POST /v1/saas/app_school_campus_get_lists`(SaasSchoolCampusReq)**

| 字段 | 必填 | 说明 |
|------|------|------|
| userId | ✅ | 从 tokenAuth 拿 |
| saasAppId | ❌ | 默认用后端 appId |

返回:`{schools:[{schoolId, schoolName, staffId, tenantId, campus:[...]}], schoolCampus:[...]}`

**`POST ${SAAS_URL}/saas-auth/sso/channel/token/auth`(saas-auth 服务,非 lesson)**

| 字段 | 必填 | 说明 |
|------|------|------|
| token | ✅ | Authorization(saas token) |

返回:`{authenticationResource:{ucUserInfoVO:{id=userId, name, avatar, mobile}, accountName, accessToken, ...}}`

---

## 设计方案

### 用户决策(已确认)

| 议题 | 决策 |
|------|------|
| Authorization 来源 | `SAAS_AUTH` env 变量 |
| 命令形态 | 元数据驱动加 `saas` service(basePath `/`) |
| hrToken 处理 | 写入 `~/.hr-cli/config.json`,后续命令自动复用 |
| client 改动 | 扩展现有 `Do` 方法(支持 Authorization header) |
| skill 文档 | 新增 `skills/hr-auth/SKILL.md` |
| 多学校/校区 | 有多个就列出让用户选(友好区 + [internal] 区格式),不自动选 recentLogin |
| 生产环境 | 本期只接测试环境,生产 URL 留 TODO,用户可用 `--saas-url` flag 手传 |

### 新增的 2 个接口到 meta_data.json(token_auth 不进)

**核心简化**:token_auth 不进 meta_data,直接在 +login shortcut 里 `net/http` 裸调 saas-auth 服务。

**理由(3 条,任一足以支持不进)**:
1. **返回结构不兼容**:saas-auth 返回 `CommonResult`(saas 服务统一格式),不是 lesson 的 `ResultJson`。现有 `client.go:60-67` 写死了 `var result ResultJson` 解析,token_auth 走现有 client 会直接挂掉。
2. **URL 模型不一致**:lesson API 是 `${BASE_URL}${basePath}${path}`,token_auth 是独立的 saas-auth 服务域名,塞进元数据要加 `externalBaseURL` 字段打破统一模型。
3. **避免给通用框架开特例**:为 1 个异构接口给 types.go/client.go/cmd/service.go 三处加分支判断,得不偿失。

### 最终决策:混合方案(同构走元数据,异构走裸调)

**按接口同构性分层**:

| 接口 | 同构性 | 实现方式 |
|------|--------|---------|
| `token_auth`(saas-auth 服务) | ❌ 异构(CommonResult + 不同 URL + Authorization) | shortcut 内 `net/http` 裸调 |
| `app_school_campus_get_lists`(lesson) | ✅ 同构(ResultJson) | 进 meta_data,走 runMethod |
| `login`(lesson) | ✅ 同构(ResultJson) | 进 meta_data,走 runMethod |

**lesson 2 个 saas 接口进 meta_data 的特殊处理**:它们需要 `Authorization` header(不是 hrToken)。给 Method struct 加 `AuthHeader string` 字段(默认 "hrToken"),client.go 按此字段决定注入哪个 header。token 来源:saas service 用 `SAAS_AUTH` env,其他 service 用 hrToken 解析链。

**修订后的 +login 链路**:
```
saas +login shortcut (shortcuts/saas/login.go)
  ├─ 步骤1: net/http 裸调 saas-auth tokenAuth (绕开元数据,解析 CommonResult)
  ├─ 步骤2: runMethod("saas", "app_school_campus_get_lists", {userId})
  │         返回 {schools:[...], schoolCampus:[...]},嵌套深、字段多
  │         ↑ 在 Go 代码里只遍历 schools[] 取 schoolName + campus[],不暴露完整响应
  ├─ 步骤3: 用户选择学校/校区(多个时列出,友好区+[internal]区格式)
  ├─ 步骤4: runMethod("saas", "login", {staffId,tenantId,schoolId,campusId,authorization})
  └─ 步骤5: config.SaveToken(hrToken)  ← 持久化
```

### 列表响应处理(重要)

**用户可能跨多个学校**——`app_school_campus_get_lists` 返回结构大且嵌套深(`{schools:[{...,campus:[...]}], schoolCampus:[...]}`),必须按场景处理:

**场景 1:+login shortcut 内部调用**(走 runMethod)
- runMethod 返回完整 `result.Data map[string]any`,**不能用 --jq**
- 在 Go 代码里直接遍历 `result.Data["schools"].([]any)`,每个 school 取 `schoolName` + `campus` 列表
- 友好区只展示人类可读字段(学校名/校区名),`[internal]` 区附带 ID 供步骤4 使用

**场景 2:用户直接调 service 命令**(`hr-cli saas app_school_campus_get_lists --data '...'`)
- **必须用 --jq 投影**,避免完整响应炸 token
- 推荐投影模板(写进 meta_data.json 的 responseSchema):
  ```bash
  hr-cli saas app_school_campus_get_lists --data '{"userId":"..."}' \
    --jq '{total: (.schools|length), items: [.schools[] | {school: .schoolName, staffId, tenantId, campuses: [.campus[] | .campusName]}]}'
  ```
- meta_data.json 的 responseSchema 要标 `listKey: schools` + 投影示例,让 AI 知道用 --jq

### +login shortcut 编排(核心)

```bash
hr-cli saas +login
# 自动流程(全部在 shortcut 里,net/http 裸调,不走元数据引擎):
# 1. 读 SAAS_AUTH env
# 2. 裸调 saas-auth:
#    POST ${SAAS_URL}/saas-auth/sso/channel/token/auth
#    body: {"token": "<SAAS_AUTH>"}
#    解析 CommonResult → 拿 userId + accountName
# 3. 裸调 lesson app_school_campus_get_lists:
#    POST ${BASE_URL}/v1/saas/app_school_campus_get_lists
#    header: Authorization: <SAAS_AUTH>
#    body: {"userId": "<userId>"}
#    解析 ResultJson → 拿 schools[]
# 4. 学校/校区选择:
#    - 只有 1 个学校 1 个校区 → 自动用
#    - 有多个 → 列出让用户选(友好区 + [internal] 区格式),等用户回复序号
# 5. 裸调 lesson login:
#    POST ${BASE_URL}/v1/saas/login
#    header: Authorization: <SAAS_AUTH>
#    body: {staffId, tenantId, schoolId, campusId, authorization: SAAS_AUTH}
#    解析 ResultJson → 拿 hrToken
# 6. 写入 ~/.hr-cli/config.json
# 输出:✅ 登录成功 / 账号: 沈超杰 / 学校: XX / 校区: YY / hrToken 已保存
```

**flag**:
- `--saas-url`(可选,默认测试环境 `http://10.30.5.53:31759`;生产环境用户需手传,本期不内置)
- `--school-id` / `--campus-id`(可选,跳过选择直接用,适合用户已知 ID 的场景)

### config 持久化(扩展 config.go)

- `ResolveToken(flagToken)`:解析链改为 `flag > env > config file`
- `SaveToken(hrToken)`:写入 `~/.hr-cli/config.json`(权限 0600)
- `LoadConfig()`:读取
- `ResolveSaasAuth()`:读 `SAAS_AUTH` env
- 文件格式:`{"hrToken":"...","savedAt":"...","userInfo":{accountName,schoolName,...}}`

### client.go 改动

`Do` 方法扩展:增加一个 header 注入逻辑。当 method 元数据标记 `authHeader: "Authorization"` 时,注入 `Authorization: <SAAS_AUTH env>`;否则维持现状注入 `hrToken: <ResolveToken>`。

具体形式(二选一,实施时看哪种更顺):
- **选项 1**:`Do` 加 `authHeader string` 参数,由 runner 层根据元数据传入
- **选项 2**:在 client 里加 `DoWithAuth` 方法,saas 走新方法

倾向选项 1,改动集中、调用点统一。

---

## 待修改/新增文件

| 文件 | 改动 |
|------|------|
| `internal/registry/meta_data.json` | 新增 `saas` service(basePath `/`)+ 2 个 method(login / app_school_campus_get_lists),都标记 `authHeader: "Authorization"` |
| `internal/registry/types.go` | Method struct 加 `AuthHeader string` 字段(默认 "hrToken") |
| `internal/client/client.go` | `Do` 方法接收额外 header 参数,或读 method 元数据的 authHeader 决定注入 hrToken 还是 Authorization |
| `internal/runner/runner.go` | `RunMethod` 按 method 的 authHeader 决定 token 来源:Authorization → 读 `SAAS_AUTH` env;hrToken → 走现有 ResolveToken |
| `internal/config/config.go` | 扩展 token 解析链(flag > env > config file)+ 新增 SaveToken/LoadConfig/ResolveSaasAuth |
| `cmd/service.go` | schema 命令展示 authHeader 字段(可选,提升可读性) |
| `shortcuts/saas/saas_login.go` | **新增** +login shortcut,步骤1 裸调 tokenAuth,步骤2/4 走 runMethod,步骤5 SaveToken |
| `shortcuts/register.go` | 注册 saas shortcut |
| `skills/hr-shared/SKILL.md` | 认证章节加指针引用 hr-auth |
| **新增** `skills/hr-auth/SKILL.md` | 完整 workflow:SAAS_AUTH env、+login 全自动流程、边界说明 |
| **新增** `skills/hr-auth/references/hr-auth-login.md` | +login 参数表 + 内部三步链路说明 + 错误码 |

**不动**:现有 course/groupmanage/listen service 的行为、邀课 skill、其他 shortcut、cmd/build.go 命令树组装。

---

## 验证方式

1. **元数据验证**:
   ```bash
   make build
   hr-cli schema saas              # 列 3 个 method
   hr-cli schema saas.token_auth   # 看入参(只有 token)
   ```

2. **单接口验证**(token_auth 直接调 saas-auth):
   ```bash
   export SAAS_AUTH="<真实 saas token>"
   hr-cli saas token_auth --saas-url "http://10.30.5.53:31759"
   # 预期:返回 userId + accountName
   ```

3. **全自动登录验证**:
   ```bash
   hr-cli saas +login
   # 预期:自动走完三步,输出账号+学校,~/.hr-cli/config.json 写入
   ```

4. **持久化复用验证**:
   ```bash
   hr-cli course coursetag_get_lists --data '{"schoolId":"...","campusId":"..."}'
   # 预期:成功(从 config.json 自动读 hrToken)
   ```

5. **多学校分叉验证**:若有多个学校/校区,+login 应列出选择(友好区展示学校名,[internal] 区含 schoolId/campusId),用户回复序号后继续

6. **错误场景**:
   - SAAS_AUTH 未 export → 提示 export
   - Authorization 过期 → token_auth 失败,提示重新拿
   - 配置文件无写权限 → 降级输出 hrToken 提示手动 export
   - `--saas-url` 未传且非测试环境 → 提示用户传生产 URL(本期不内置生产值)

---

## 诚实标注的边界

- ✅ **支持**:SAAS_AUTH env 全自动登录(tokenAuth → 查学校 → login → 持久化)
- ✅ **支持**:多学校/多校区场景列出让用户选
- ✅ **支持**:测试环境开箱即用(内置 saas-auth URL)
- ❌ **不支持**:账号密码登录(lesson 有 account_login 但本期不做)
- ❌ **不支持**:钉钉/飞书免登(free_login 场景不同)
- ⚠️ **限制**:生产环境需用户手传 `--saas-url`(本期不内置生产 URL,留 TODO)

---

## 探索过程的关键教训

1. **别接受"不可能"的结论太快**:第一轮探索后误以为"5 件套硬约束"是后端约束,CLI 无法绕过。用户追问"还是没解决 userId"才重新挖,发现底层 saas-auth 服务的 tokenAuth 只用 Authorization 就能换 userId。
2. **约束是分层的**:表层 API(lesson)的约束,在底层服务(saas-auth)可能根本不存在。CLI 作为 AI Agent 的手脚,有特权直连底层服务。
3. **字段消费层决定收集策略**:`getSaasInfo` 的 RequestLoginDto 有 5 个字段,但 `doLogin` 只用 authorization 一个——这是判断字段是否真正必需的关键证据。
