---
name: hr-invite
version: 0.2.0
description: "听评课邀课管理。核心场景:创建听评课邀请(invite +create)。M8 已支持查教师/课型/量表/节次,可对话式创建邀课。"
---

# invite(课程邀请)

**CRITICAL — 开始前 MUST 先用 Read 工具读 [`../hr-shared/SKILL.md`](../hr-shared/SKILL.md)**,其中包含认证、命令结构、错误处理的通用规则。

## 什么时候用本 skill

用户表达这些意图时,使用本 skill:
- "创建一个听评课邀请" / "建邀课" / "发起评课"
- "给 X 老师下周五第三节课建听评课"
- "生成一个评课邀请码"

**术语映射**:用户口语"评课"、"听课"、"听评课",在本 skill 都对应"课程邀请"操作。底层 service 为 `course`(邀课相关接口挂在 course 下,不是 invite),shortcut 命令仍叫 `invite +create`。

## ⚡ 创建邀课的两种模式

### 模式 1:对话式创建(推荐,用户表达不完整时)

**CRITICAL — 用户没给齐所有 ID 时 MUST 走这个模式。**

读 [`references/hr-invite-create-workflow.md`](references/hr-invite-create-workflow.md),按 Round 1→(2)→3 流程:
- **Round 1**:智能补齐——解析用户已说的,**只问缺失项**(学校/校区/时间/课型/班级/地址/邀请人),用户一次回复
- **Round 2**(可选):仅在出现分叉时触发(同名教师/课型多选/节次歧义),所有分叉**合并成一条消息**
- **Round 3**:最终确认 + 问课程名称,用户"确认"后创建

**目标:把沟通轮次压到 ≤ 3 轮(无分叉)或 ≤ 4 轮(有分叉)**。禁止把 API 依赖链(查教师依赖校区 ID)硬塞给用户逐轮问。

### 模式 2:直接创建(用户给齐 ID 时)

用户已经通过其他方式(如 URL、curl)拿到所有 ID 时,直接调:
[`references/hr-invite-create.md`](references/hr-invite-create.md)

## 核心概念

- **课程邀请(invite)**:一次听评课活动的邀请记录,生成后产生 `courseInviteCode`(邀请码)
- **听课方式 listenType**:`select`(指定人)、`appointment`(按范围预约)、`other`(其他)。**展示给用户时 MUST 翻译成中文,不要直接给 select/appointment/other 枚举值**
- **节次 node**:第几节课(1-10 正课 / 11 早读 / 12-15 晚自习)
- **教研组(teachGroup)**:教师的组织单位,影响量表推荐和权限

## 资源关系

```
学校(school)
└── 校区(campus)
    └── 课程邀请(invite)
        ├── 授课老师(teacher)  ← 属于教研组(teachGroup)
        ├── 评课量表(commentPaper)  ← 由课型 + 教研组决定推荐
        └── 邀请成员(member)  ← 被邀请评课的人
```

## Shortcuts

| Shortcut | 说明 |
|----------|------|
| [`+create`](references/hr-invite-create.md) | 创建听评课邀请(智能默认 + 友好输出) |
| [`+search-teacher`](#) `*` | 按姓名模糊查教师(`groupmanage` service) |

`*` 标记的 shortcut 属于 `groupmanage` service,但常用于邀课流程。

## 业务规则

**CRITICAL — 创建邀课前必须了解:**

1. **去重约束**:同一 `teacherId` + `time(日期)` + `node(节次)` 不能重复创建。后端返回 `当前课程已存在,请检查授课教师、日期和节次`。解决:改 `--time`(换日期)或 `--node`(换节次)

2. **校外教师**:默认 `isOutside=0`(非校外)。校外教师场景用 service 命令 `course add --data '{...,"isOutside":1,...}'`,`+create` shortcut 不暴露此参数

3. **评课量表推荐**:由 `courseTagId`(课型)+ `teachGroupId`(教研组)共同决定。必须调 `course coursepaper_get_suit_lists` 查推荐列表,不要瞎填 commentPaperId

4. **节次取值范围**:1-10 正课 / 11 早读 / 12-15 晚自习。不在列表的值会被后端拒绝

5. **⚠️ 创建前最终确认(强制)**:`+create` 是 write 操作,所有参数齐后,Claude MUST 把完整信息列给用户确认,得到明确同意后才创建。详见 [workflow Step 8.5](references/hr-invite-create-workflow.md)

## 查询接口(创建前用)

| 接口 | 用途 | 必传参数 |
|------|------|---------|
| `groupmanage +search-teacher` | 按姓名查教师 | school-id, campus-id, name |
| `course coursetag_get_lists` | 查课型列表 | schoolId, campusId |
| `course coursepaper_get_suit_lists` | 查推荐量表 | schoolId, campusId, courseTagId |
| `course course_node_get_lists` | 查节次列表 | schoolId, campusId |

**典型流程**:查教师 → 查课型 → 查量表 → 校验节次 → 创建。详见 [workflow](references/hr-invite-create-workflow.md)。

## API Resources

```bash
hr-cli schema course             # 列出 course 所有 method
hr-cli schema course.add         # 查看 add 完整参数
hr-cli schema groupmanage        # 列出 groupmanage 所有 method
```

| service.method | risk | 说明 |
|----------------|------|------|
| `course.add` | write | 添加听评课邀请(+create 的底层) |
| `course.coursetag_get_lists` | read | 查课型列表 |
| `course.coursepaper_get_suit_lists` | read | 查推荐评课量表 |
| `course.course_node_get_lists` | read | 查节次列表 |
| `groupmanage.member_get_lists` | read | 查教研组成员(+search-teacher 的底层) |

## 决策树:用户说"创建邀课"

```
用户提供齐全 ID 了吗?(teacher-id/school-id/course-tag-id/comment-paper-id 等)
├─ 是 → 调 hr-cli course +create(参考 references/hr-invite-create.md)
└─ 否 → MUST 走 workflow(参考 references/hr-invite-create-workflow.md)
         Round 1 智能补齐(只问缺失项)
         → 内部并行查教师/课型/量表/节次
         → Round 2 合并分叉(若有)
         → Round 3 最终确认 + 问课程名称 → 创建
```

## MVP 能力边界(邀课领域专属)

通用边界(按学校名查 schoolId 等)见 [`hr-shared/SKILL.md`](../hr-shared/SKILL.md),此处只列邀课领域特有:

- ✅ **支持**:创建听评课邀请(invite +create)
- ❌ **不支持**:邀课列表查询、编辑、删除

用户请求未支持能力时,诚实告知边界,不要瞎编。
