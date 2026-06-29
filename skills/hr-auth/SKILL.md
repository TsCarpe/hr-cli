---
name: hr-auth
version: 0.1.0
description: "saas 登录与 hrToken 持久化。系统前置门槛:未登录任何接口都 401。export SAAS_AUTH 后一条命令自动登录,后续所有命令免传 --token。"
---

# hr-auth(saas 登录)

**CRITICAL — BLOCKING:本 skill 是所有其他 skill 的前置条件。** 系统是登录后才能使用的,未登录状态下任何 listen/course/groupmanage 接口都会返回 401,Claude 无法处理任何业务请求。

**开始前先读 [`../hr-shared/SKILL.md`](../hr-shared/SKILL.md)** 的「登录(前置门槛,BLOCKING)」章节。

## 什么时候用本 skill(优先级最高)

以下场景 **MUST 先完成登录,再处理其他请求**:
- 会话开始,首次处理任何业务请求前
- 用户说 "我需要 token"/"怎么登录"/"hrToken 怎么拿"
- 用户说 "token 过期了"/"401 了"/"未授权"
- 任何命令返回 401 或认证失败错误
- 用户表达任何听评课/邀课/查询意图,但当前未登录

**规则**:未登录时 **MUST 暂停所有业务处理,先引导用户完成登录**。不要尝试"先试一下看看",401 是确定性的。

## 什么时候用本 skill

用户表达这些意图时,使用本 skill:
- "我需要 token"/"怎么登录"/"hrToken 怎么拿"
- "token 过期了"/"401 了"
- "帮我登录一下"
- 任何需要 hrToken 但当前没设置的场景

## 核心命令:`saas +login`

```bash
# 前置:用户先 export saas token(从 saas 系统 UI 拿)
export SAAS_AUTH="xxx"

# 一条命令全自动登录
hr-cli saas +login
```

**自动流程**(用户不感知):
1. 用 SAAS_AUTH 调 saas-auth 服务换 userId
2. 用 userId 查用户可访问的学校/校区
3. 单学校单校区自动用;多学校/多校区列出让用户选
4. 用选中的身份调 lesson login 换 hrToken
5. 持久化到 `~/.hr-cli/config.json`

成功后输出:
```
✓ 身份验证通过: 沈超杰 (userId: 7044...)
✅ 登录成功
账号: 沈超杰
学校: 海亮外国语学校 / 主校区
hrToken 已保存到 ~/.hr-cli/config.json,后续命令自动使用
```

**之后所有命令无需 `--token`**:hr-cli 自动从 config.json 读 hrToken。

## 前置条件:SAAS_AUTH 怎么拿

`SAAS_AUTH` 是底层 saas 服务签发的 token(不是 hrToken)。用户从 saas 系统 UI 获取:
- 登录 saas 系统的 web 界面
- 浏览器 devtools → Network → 任意请求的 `Authorization` header 复制
- `export SAAS_AUTH="复制的值"`

**CRITICAL — 不要向用户展示完整 SAAS_AUTH**。引导用户自己 export。

## 选学校/校区的交互

用户跨多个学校时,`+login` 会列出选择:

```
请选择要登录的学校:

1. 海亮外国语学校(校区 3 个)
2. 海亮实验中学(校区 1 个)

[internal]
1. 海亮外国语学校(校区 3 个)
2. 海亮实验中学(校区 1 个)

回复序号:
```

**规则**:
- 友好区(上半):只展示学校名/校区名,**转述给用户用这部分**
- `[internal]` 区(下半):给 Claude 看的,含 ID
- **CRITICAL — 转述给用户时 MUST 省略 `[internal]` 区**

跳过选择:用户传 `--school-id` / `--campus-id` flag 直接指定。

## 边界

- ✅ **支持**:SAAS_AUTH env 全自动登录 + 多学校选择 + 持久化复用
- ✅ **支持**:测试环境开箱即用(内置 saas-auth URL)
- ⚠️ **限制**:生产环境需用户传 `--saas-url`(本期不内置生产 URL)
- ❌ **不支持**:账号密码登录(lesson 有 `account_login` 但本期不做)
- ❌ **不支持**:钉钉/飞书免登(`free_login` 场景不同)

## 失败排查

| 错误 | 原因 | 解决 |
|------|------|------|
| `未设置 SAAS_AUTH 环境变量` | 没 export | 引导用户 `export SAAS_AUTH=xxx` |
| `tokenAuth 失败` | SAAS_AUTH 过期或无效 | 重新从 saas 系统拿 |
| `未找到任何学校` | 用户无学校权限 | 联系管理员分配学校 |
| `多学校选择逻辑未实现` | 用了旧版二进制 | 重新 `make build` |
| 网络错误 | saas-auth 服务不通 | 确认 `--saas-url`,测试环境 `http://10.30.5.53:31759` |

## 参数细节

完整参数表和内部链路见 [`references/hr-auth-login.md`](references/hr-auth-login.md)。

## Claude 行为指引(重要)

**Claude 处理任何业务请求前,必须确认登录态**:

```
用户:帮我创建一个听评课邀请

Claude(内部检查):~/.hr-cli/config.json 存在且 hrToken 非空?

  ├─ 是 → 继续,读 hr-invite/SKILL.md 处理
  └─ 否 → MUST 暂停,引导登录:
       你需要先登录才能创建邀课。请执行:
       1. export SAAS_AUTH="<从 saas 系统 UI 拿的 Authorization>"
       2. hr-cli saas +login
       登录成功后告诉我,我继续帮你创建。
```

**禁止**:
- ❌ 未登录时尝试调任何业务接口(必然 401,浪费调用)
- ❌ 让用户从浏览器 devtools 手动复制 hrToken(有自动登录,不要走弯路)
- ❌ 在对话里展示完整 SAAS_AUTH 或 hrToken

**鼓励**:
- ✅ 主动检查 ~/.hr-cli/config.json 判断登录态
- ✅ 遇到 401 立即切换到登录引导
- ✅ 登录成功后,简洁告知用户并继续原业务
