# 接口建模前的能力调研 Checklist

> 每次给 listen 后端接口建模(往 `meta_data.json` 加 method)前必读。
>
> 核心原则:**字段是否生效要看 Business 层源码,不是看 Java 注释或 validator profile**。

## 为什么需要这个 checklist

后端接口的真实能力,跟"看 Controller 注释/reqo 类字段"得到的印象,**经常不一致**。原因:

1. **Java 注释不完备**:50+ 字段,很多没注释;有注释的也可能含糊(比如"授课老师"不说是被评人还是评课人)
2. **一个 reqo 类复用给多个接口**:`ListenRecordPageReq` 复用给 `listen_get_lists` / `course_get_lists` / `getTeacherListenRecordPage` 等十几个接口。**validator 的 profile 只管必填校验,不管字段是否被使用**
3. **Business 层有字段处理逻辑**(尤其 Convert 类的 `toQuery` 方法):字段在 `toQuery` 里被 `if (!isEmpty)` 处理才算真生效,profile 不匹配的字段也可能被处理
4. **Business 层有强制覆盖逻辑**:`setUserId(staffId)`、`setCampusId(null)`、硬编码排序——这些才是接口的真实语义

**只读 Controller/reqo 会必然误判接口能力**。

---

## 字段生效的"三层失效"真相

字段从请求到 SQL,要过 3 道关,**任何一道都能让字段失效**:

```
第 1 关:validator(profile)
   ↓ 只管必填校验,profile 不匹配的字段也会被 Spring 接收进 reqo
第 2 关:Convert.toQuery / 类似的字段构造方法
   ↓ 不区分 profile!只要 reqo 字段非空,就 set 到 query 上
第 3 关:Business 层覆盖
   ↓ 如 listenGetLists 里 setUserId(staffId) 会覆盖用户传的 userId
```

**结论**:字段是否最终生效,**不只看 profile**,还要看 toQuery 是否处理它、Business 是否覆盖它。

**实证案例**(来自 `ListenRecordConvert.java:65-117` 的 `toQuery(ListenRecordPageReq)`):
- `recordStartTime` / `recordEndTime` 不在任何 validator profile 里
- 但 `toQuery` L87-91 处理了它们
- 所以这俩字段**实际生效**(传了会进 SQL),只是不强制必填

---

## 调研必做项

| 项 | 做什么 | 为什么 |
|---|-------|-------|
| 读 Business 层实现 | 看 Service/Convert 类,找字段处理逻辑和强制覆盖 | Business 层是接口真实语义和字段生效判断的 source of truth |
| 区分"validator profile"和"实际生效" | profile 只决定是否必填校验;字段在 toQuery 里被处理才算真生效 | 不在 profile 里 = 不强制,但传了仍可能被使用 |
| 用真实调用验证 | dry-run + 真实 token 跑一次 | 验证假设的接口定位和字段筛选是否成立 |
| 能力清单要完整 | 列出所有进 toQuery 的字段及含义 | 不要挑几个就建,漏字段 = 漏能力 |
| 描述要包含边界 | 在 schema description 标注限制(如"只能查自己"、"必填但被 Business 清空") | AI 读了就知道边界,不会瞎试 |

---

## 反模式

### ❌ 反模式 1:被质疑后不验证就反向改结论(滑跪)

**错误做法**:用户说"这个接口应该是通用的",你不验证就直接改了结论,结果把本来对的定位改错了。

**正确做法**:先问"你指的是哪个接口"——后端可能有多个相似接口(`listen_get_lists` vs `getTeacherListenRecordPage`),确认目标后再决定要不要改。**被质疑 ≠ 自己错了**。

### ❌ 反模式 2:用残缺理解给接口下边界结论

**错误做法**:看了几个字段就说"按评课人查不支持,是后端缺口"——其实是你没看完所有相关接口,别的接口可能支持。

**正确做法**:说"不支持"前先看完所有相关接口(`Convert.toQuery` + 同类查询接口)。CLI 该诚实透传能力边界,不该编造。

### ❌ 反模式 3:把 reqo validator profile 当字段是否生效的判据

**错误做法**:看 `@NotNull(profiles={"xxx"})` 的 profiles 数组,不在目标 profile 里就认为"字段不生效"。

**正确做法**:profile 只管必填校验,字段生效要看 `Convert.toQuery`(或同类构造方法)是否处理。读 Business 层源码是唯一可靠依据。

---

## 调研产出模板

每次接口建模完成时,应该能回答下面 5 个问题。答不上来就别建。

```
接口:<service.method>
路径:POST /v1/...

1. 这个接口查的是什么数据?(一句话定位)
   答:

2. 强制过滤条件是什么?(Business 层覆盖了哪些入参)
   答:例如 setUserId(staffId) 强制按当前用户过滤

3. 在 Convert.toQuery 中被处理的字段有哪些?(这些才是真生效)
   答:

4. 不能做什么?(明确边界)
   答:例如"只能查自己,不能查他人"

5. 真实调用验证结果?
   答:用真实 token 调一次,数据范围是否符合定位
```

把这份填好的模板留在 commit message 或 PR 描述里,便于后续审查。

---

## service 归属规则(建模时一并决定)

接口归哪个 service,**按数据实体归属,不按"我听的/我讲的"视角成对**:

| 查询的表 | service |
|---------|---------|
| `course_info`(课程实体) | `course` |
| `listen_record`(评课记录实体) | `listen` |
| `teach_pending`(待办实体) | `listen` |
| 教研组相关表 | `groupmanage` |

**为什么按实体不按视角**:
- 视角会变(同一个用户既是评课人又是授课老师),按视角分会让 service 职责模糊
- 实体稳定,按实体分边界清晰、耐变化
- 例:`course_get_lists`(我讲的课)查 `course_info` 表,归 `course`;`listen_get_lists`(我听的课)查 `listen_record` 表,归 `listen`——虽然视角成对,但实体不同,各自归属
