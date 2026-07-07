# saas +login

用 haiclaw saas token 自动换 hrToken,持久化到 `~/.hr-cli/config.json`,后续命令免传 `--token`。

**开始前 MUST 读 [`../SKILL.md`](../SKILL.md) 和 [`../../hr-shared/SKILL.md`](../../hr-shared/SKILL.md)。**

## 使用场景

- 用户需要 hrToken 但没设置
- token 过期(调 listen API 返回 401)
- 想要免 token 体验(登录一次,后续命令自动复用)

## 前置条件

saas token 的唯一来源:

**haiclaw 配置文件**
- 已用 haiclaw CLI 登录,token 自动持久化到 `~/.haiclaw/saas-config.json`(`saasToken` 字段)
- `hr-cli saas +login` 自动读取

## Flags

| flag | 默认值 | 说明 |
|------|--------|------|
| `--saas-url` | `http://10.30.5.53:31759` | saas-auth 服务地址(测试环境;生产需手传) |
| `--school-id` | (空) | 跳过学校选择,直接用指定学校 |
| `--campus-id` | (空) | 跳过校区选择,直接用指定校区 |

## 调用示例

### 最简(全自动)

```bash
hr-cli saas +login
```

### 指定学校校区(跳过交互)

```bash
hr-cli saas +login --school-id 6970662285670088704 --campus-id 7037712993508421632
```

### 生产环境

```bash
hr-cli saas +login --saas-url "https://生产环境-saas-auth地址"
```

## 内部链路(三步)

```
~/.haiclaw/saas-config.json 的 saasToken
    ↓ 步骤1: net/http 裸调 saas-auth(不走元数据,返回 CommonResult)
    POST ${SAAS_URL}/saas-auth/sso/channel/token/auth
    body: {"token": "<saasToken>"}
    → 拿 userId + accountName
    ↓ 步骤2: 走元数据调 lesson
    POST ${BASE_URL}/lesson/v1/saas/app_school_campus_get_lists
    header: Authorization: <saasToken>
    body: {"userId": "<userId>", "saasAppId": "6874934855898312704"}
    → 拿 schools[] + campus[]
    ↓ 用户选择(多学校时)
    ↓ 步骤3: 走元数据调 lesson
    POST ${BASE_URL}/lesson/v1/saas/login
    header: Authorization: <saasToken>
    body: {staffId, tenantId, schoolId, campusId, authorization: <saasToken>}
    → 拿 hrToken
    ↓ 持久化
    写入 ~/.hr-cli/config.json (权限 0600)
```

**为什么步骤1不走元数据**:saas-auth 服务返回的是 saas 的 `CommonResult`(不是 lesson 的 `ResultJson`),且 URL 不同源、认证模型不同。强行塞进元数据引擎要给框架开特例,得不偿失。

## 输出

### 成功

```
✓ 身份验证通过: 沈超杰 (userId: 7044624002919739392)
✅ 登录成功
账号: 沈超杰
学校: 海亮外国语学校 / 主校区
hrToken 已保存到 ~/.hr-cli/config.json,后续命令自动使用
```

### 多学校选择

```
✓ 身份验证通过: 沈超杰 (userId: 7044...)

请选择要登录的学校:

1. 海亮外国语学校(校区 3 个)
2. 海亮实验中学(校区 1 个)

[internal]
1. 海亮外国语学校(校区 3 个)
2. 海亮实验中学(校区 1 个)

回复序号:
```

**给用户转述时**:只说友好区(上半),不提 `[internal]` 区。

### 持久化失败(降级)

```
✅ 登录成功
账号: 沈超杰
学校: XX / XX
⚠ 持久化失败(权限不足),请手动 export:
export LISTEN_TOKEN="eyJ..."
```

## 配置文件格式

`~/.hr-cli/config.json`(权限 0600):

```json
{
  "hrToken": "eyJhbGci...",
  "savedAt": "2026-06-26T10:30:00Z",
  "userInfo": {
    "accountName": "沈超杰",
    "schoolName": "海亮外国语学校",
    "campusName": "主校区"
  }
}
```

**安全**:
- 权限 0600(只用户可读)
- `+login` 输出**不含 hrToken 本身**(只说已保存)
- 后续命令读 config.json 时不打 log

## 错误处理

| Error 消息 | 原因 | 解决 |
|-----------|------|------|
| `未找到 saas token` | haiclaw 配置不存在 | 用 haiclaw 登录,生成 `~/.haiclaw/saas-config.json` |
| `tokenAuth 请求失败` | saas-auth 服务不通 | 确认 `--saas-url`,测试环境默认值 |
| `tokenAuth 失败(status=xxx)` | saas token 过期/无效 | 重新用 haiclaw 登录刷新 |
| `解析 tokenAuth 响应失败` | saas-auth 返回结构异常 | 看原始响应排查 |
| `未找到任何学校` | 用户无学校权限 | 联系管理员 |
| `校区 X 不在学校 Y 内` | `--campus-id` 和 `--school-id` 不匹配 | 检查 ID |
| `login 返回的 hrToken 为空` | lesson login 异常 | 看 result 详情 |

## 相关命令

```bash
# 直接调 lesson 的 saas 接口(高级用法,通常不需要)
hr-cli schema saas                              # 看 saas service 的方法
hr-cli saas app_school_campus_get_lists --data '{"userId":"..."}'  # 单独查学校
hr-cli saas login --data '{...}'                # 单独调 login(需手动组装参数)
```

**通常无需直接调这些**——`+login` 已经封装了完整流程。
