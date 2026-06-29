# course +create

创建听评课邀请,生成邀请码(`courseInviteCode`),供教师扫码或凭码参与评课。

**开始前 MUST 先读 [`../../hr-shared/SKILL.md`](../../hr-shared/SKILL.md)** 获取认证与错误处理规则。

## 使用场景

- 用户:"创建一个听评课邀请"
- 用户:"给 X 老师建一节评课"
- 用户:"生成一个评课邀请码"

## 必填参数

**CRITICAL — 以下 13 个 flag 必须全部提供,缺一不可:**

| flag | 说明 | 示例值 |
|------|------|--------|
| `--school-id` | 学校 id | `6970662285670088704` |
| `--campus-id` | 校区 id | `6970663625087516672` |
| `--teacher-id` | 授课老师 id | `6968852941467111424` |
| `--teacher-name` | 授课老师姓名 | `张三` |
| `--name` | 课程名称 | `高一语文《荷塘月色》` |
| `--time` | 授课时间 | `2026-06-30 14:00:00` |
| `--node` | 授课节次(整数) | `3` |
| `--course-tag-id` | 课程标签 id | `7196754881811050496` |
| `--class-name` | 授课班级 | `高一(3)班` |
| `--addr` | 授课地址 | `A301` |
| `--teach-group-id` | 授课老师教研组 id | `7457650924890157056` |
| `--current-user-teach-group-id` | 当前用户(操作者)教研组 id | `7457650924890157056` |
| `--member-ids` | 邀请人 id 列表(逗号分隔) | `7166276249457274880` |

## 可选参数(带默认值)

| flag | 默认值 | 说明 |
|------|--------|------|
| `--listen-type` | `select` | 听课方式(`select`/`appointment`/`other`) |
| `--comment-paper-id` | `1` | 评课量表 id |
| `--comment-paper-name` | `通用教师评课表` | 评课量表名称 |
| `--node-display` | (空) | 节次显示名,如 `第三节`(建议提供,提升 Web 端显示效果) |

## 智能默认(用户无需关心)

shortcut 自动补齐以下字段,**不要**让用户传:

- `isOutside`: `0`(非校外教师)
- `appointmentRange`: `all`(全校预约范围)
- `outsideSchoolId` / `outsideSchoolName`: 空
- `teachPlanId`: 空
- `saasSchoolId`: 复制自 `--school-id`
- `saasCampusId`: 复制自 `--campus-id`
- `courseInviteGroupAddReqList`: 空数组
- `resourceList`: 空数组

## 调用示例

### 最小调用

```bash
hr-cli course +create \
  --base-url "https://hrjy-test.hailiangedu.com/hr" \
  --token "$LISTEN_TOKEN" \
  --school-id 6970662285670088704 \
  --campus-id 6970663625087516672 \
  --teacher-id 6968852941467111424 \
  --teacher-name "测试小老弟1" \
  --name "M7示例" \
  --time "2026-06-30 00:00:00" \
  --node 3 \
  --course-tag-id 7196754881811050496 \
  --class-name "高一(3)班" \
  --addr "A301" \
  --teach-group-id 7457650924890157056 \
  --current-user-teach-group-id 7457650924890157056 \
  --member-ids 7166276249457274880
```

### DryRun(预览请求,不发)

```bash
hr-cli course +create --dry-run ...  # 加 --dry-run 即可
```

输出会打印 `POST <URL>` + 脱敏 hrToken + Body,不实际调用。

## 输出

成功时输出友好卡片(非 JSON,**只含用户能理解的字段**):

```
✅ 邀课创建成功

邀请码: CQZGQ5
(课程邀请已创建,授课老师和受邀评课人可凭此邀请码扫码评课)
```

**告诉用户的只有"邀请码"**(courseInviteCode 的值)。不要把 courseInviteId 等技术 ID 展示给用户——他们看不懂,也没用。

**Claude 告诉用户时**:"已创建,邀请码是 CQZGQ5"——简短即可,不要复述 JSON。

## 错误处理

| Error 消息 | 原因 | 解决 |
|-----------|------|------|
| `node 格式非法: "x"` | `--node` 不是整数 | 改成数字,如 `--node 3` |
| `业务失败(status=400): 当前课程已存在...` | 同老师+日期+节次已存在 | 改 `--time`(换日期)或 `--node`(换节次) |
| `业务失败(status=400): <字段> 不能为空` | 缺必填 flag | 按 message 补 |
| `业务失败(status=401)` | token 过期/无效 | 提示用户重新从 Web 系统拷 hrToken |
| 网络错误 | `--base-url` 错或服务不通 | 确认 `--base-url`,测试环境用 `https://hrjy-test.hailiangedu.com/hr` |

## 相关命令

- **精细控制**:用 service 命令 `hr-cli course add --data '{...json...}'`,可覆盖所有 26 字段
- **查参数**:`hr-cli schema course.add` 列出完整字段表
- **校内 vs 校外教师**:`+create` 默认校内(isOutside=0)。校外教师场景用 `course add --data` 显式传 `isOutside:1` + outsideSchoolId/Name
