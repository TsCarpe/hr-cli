# hr-cli

听评课(listen)系统的命令行封装,把 listen 的 REST API 包装成可被人类或 AI Agent 直接调用的 CLI 工具。基于 Go + cobra,元数据驱动,加新 API 只改 JSON 不改代码。

## 环境要求

- Go **1.24+**(更低版本编译会失败)
- 可访问 listen 网关的网络环境
- 一个有效的 saas 系统 Authorization(用于登录换取 hrToken)

## 安装

三种方式任选其一。

### 方式 1:源码编译(推荐)

```bash
git clone https://github.com/TsCarpe/hr-cli.git
cd hr-cli
make build          # 产物在 bin/hr-cli
```

或直接用 go build:

```bash
go build -o bin/hr-cli .
```

### 方式 2:go install

```bash
go install github.com/TsCarpe/hr-cli@latest
```

二进制会被装到 `$GOPATH/bin`(确保 `$GOPATH/bin` 已加入 `PATH`)。

## 首次使用

### 1. 配置后端地址(可选,默认 `http://localhost:8080`)

测试环境或自建环境,把地址写到项目根目录的 `.hr-cli.json`:

```json
{ "baseURL": "https://hrjy-test.hailiangedu.com/hr" }
```

或每次调用时传 `--base-url`,或导出 `HR_CLI_BASE_URL` 环境变量。解析优先级:`--base-url` flag > `HR_CLI_BASE_URL` > `.hr-cli.json` > 默认值。

### 2. 登录(必须,未登录调用业务接口会 401)

```bash
export SAAS_AUTH="<从 saas 系统 UI 抓到的 Authorization>"
hr-cli saas +login
```

登录成功后 token / schoolId / campusId 会写入 `~/.hr-cli/config.json`,后续调用自动复用。

### 3. 自检

```bash
hr-cli doctor       # 检查配置 / token / 连通性 / schoolId,全绿即可用
```

## 基本用法

```bash
# 查询元数据(列所有 service / 单 service 的 method / 某 method 的参数结构)
hr-cli schema
hr-cli schema course
hr-cli schema course.add

# 按意图关键词找命令(支持同义词,如"查教师""建课""邀课")
hr-cli which 邀课

# 调原始 API(1:1 映射 REST)
hr-cli course add --data '{...}'

# 调 shortcut(高频操作的智能封装,优先用)
hr-cli course +create --teacher-id 123 --course-name "语文"

# 预览请求(不发真实请求,适合调试)
hr-cli course add --data '{...}' --dry-run

# AI Agent 模式(强制 compact JSON 输出)
hr-cli --agent schema course
hr-cli --agent doctor
```

完整命令树、领域 skill、新增 API 流程等开发文档见 `CLAUDE.md` 和 `docs/`。

## License

内部使用,无对外许可证。
