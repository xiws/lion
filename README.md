# lion

lion 是一个用于发送 gRPC / HTTP 请求的命令行工具，面向微服务调试与接口联调场景。通过对接服务发现（Consul）、接口规范（Swagger / gRPC 反射），实现接口自动发现、请求自动生成与一键发送。

## 核心特性

- **接口自动生成** — 从 Swagger / OpenAPI 文档自动解析并生成接口定义和请求结构
- **Collection 分组管理** — 按服务来源组织接口，支持列表、详情查看、导出和删除
- **HTTP & gRPC 发送** — 支持 GET / POST / PUT / DELETE 等 HTTP 方法及 gRPC 动态调用（基于 Server Reflection）
- **Consul 服务发现** — 集成 Consul 自动寻址，也支持直连模式
- **多环境配置** — 独立管理 dev / staging / prod 等多套环境，一键切换
- **脚本钩子** — 支持前置 / 后置脚本（Python / Node.js / Shell），用于签名、Token 注入、断言等
- **Markdown 驱动** — 接口定义以 Markdown 文件存储，可直接编辑请求参数，可读可编辑

## 项目结构

```
lion/
├── cmd/cli/          # CLI 入口
├── internal/
│   ├── generate/     # 接口解析与生成（Swagger / gRPC / Proto）
│   ├── collection/   # Collection 管理
│   ├── sender/       # 报文发送（HTTP / gRPC）
│   ├── config/       # 配置加载
│   ├── markdown/     # Collection Markdown 解析与生成
│   ├── script/       # 脚本执行引擎（stdin/stdout 协议）
│   └── consul/       # Consul 服务发现客户端
├── docs/             # 设计文档
├── Makefile          # 构建脚本（Linux/macOS）
└── build.bat         # 构建脚本（Windows）
```

## 安装

### 从源码构建

**前置要求：** Go 1.26+

```bash
# 克隆项目
git clone <repo-url>
cd lion

# 构建当前平台
go build -o lion ./cmd/cli

# 或使用 Makefile（Linux/macOS）
make build

# 或使用 build.bat（Windows）
build.bat
```

### 交叉构建所有平台

```bash
# Linux / macOS
make build-all

# Windows
build.bat
```

支持平台：`windows/amd64`、`windows/arm64`、`darwin/amd64`、`darwin/arm64`、`linux/amd64`、`linux/arm64`

## 快速开始

### 1. 初始化配置目录

```bash
lion init
```

这会在 `~/.lion/` 下创建默认目录结构和配置文件：

```
~/.lion/
├── global.yaml      # 全局配置（环境、超时、脚本钩子等）
├── api/             # Collection 接口定义目录
└── scripts/         # 脚本文件目录
```

### 2. 配置环境

编辑 `~/.lion/global.yaml`：

```yaml
global:
  timeout: 30s
  output: pretty

environments:
  dev:
    resolve: direct
    host: "127.0.0.1:8080"
  staging:
    consul_addr: "https://consul.staging.example.com"
    consul_token: "your-token"
    resolve: consul
```

### 3. 生成接口集合

```bash
# 从 Swagger 文档生成
lion generate --name petstore --type swagger "https://petstore.swagger.io/v2/swagger.json"

# 从 gRPC 反射生成（并指定 Consul 服务名）
lion generate --name wms --type grpc "127.0.0.1:9090" --service wms-service
```

### 4. 查看接口

```bash
# 列出所有 Collection
lion collections --list

# 列出某个 Collection 下的接口
lion collections --name petstore --list

# 查看接口详情
lion collections details <api-id>
```

### 5. 发送请求

```bash
# 直连发送
lion send <api-id> --addr "127.0.0.1:8080"

# 指定环境发送
lion send <api-id> --env dev

# 覆盖参数
lion send <api-id> --env dev --param "orderId=12345"
```

## CLI 命令参考

### `lion init`

初始化配置目录和默认配置文件。

### `lion generate`

根据 Swagger / gRPC 反射 / Proto 文件生成接口集合。

```bash
lion generate [source] --name <name> --type <type> [flags]
```

| 参数 | 必填 | 说明 |
|------|------|------|
| `--name` | 是 | Collection 名称 |
| `--type` | 是 | 来源类型：`swagger` / `grpc` / `proto` |
| `--proto_root` | 否 | proto import 根目录（可多次指定） |
| `--force` | 否 | 强制重新生成，保留用户编辑的默认值 |
| `--service` | 否 | Consul 服务名称（用于 consul 寻址模式） |

**示例：**

```bash
# Swagger
lion generate --name tms --type swagger "http://tms-service:8080/swagger.json"

# gRPC 反射
lion generate --name wms --type grpc "127.0.0.1:9090" --service wms-service

# 重新生成（保留用户编辑）
lion generate --name tms --type swagger "http://tms-service:8080/swagger.json" --force
```

### `lion collections`

管理接口分组。

```bash
# 列出所有 Collection
lion collections --list

# 列出指定 Collection 的接口
lion collections --name <name> --list

# 查看接口详情
lion collections details <api-id>

# 删除 Collection
lion collections --name <name> --delete

# 导出 Collection 为 JSON
lion collections --name <name> --export
```

### `lion send`

发送请求。

```bash
lion send <api-id> [flags]
```

| 参数 | 必填 | 说明 |
|------|------|------|
| `--env` | 否 | 目标环境（不指定则使用第一个环境） |
| `--addr` | 否 | 直连地址（覆盖配置中的 host） |
| `--param` | 否 | 覆盖参数，格式 `key=value`（可多次使用） |
| `--output` | 否 | 输出格式：`pretty` / `json` / `raw` |
| `--verbose` | 否 | 显示完整请求/响应报文 |

**示例：**

```bash
lion send createOrder --env dev
lion send createOrder --addr "127.0.0.1:8080" --param "orderId=12345"
lion send getOrder --env dev --output json
```

## 配置说明

### global.yaml

全局配置文件，位于 `~/.lion/global.yaml`：

```yaml
# 全局设置
global:
  timeout: 30s            # 默认请求超时
  output: pretty          # 输出格式：pretty / json / raw

# 环境配置（可配置多个）
environments:
  dev:
    resolve: direct
    host: "127.0.0.1:8080"
  staging:
    consul_addr: "https://consul.staging.example.com"
    consul_token: "your-token"
    resolve: consul

# 全局默认值规则
defaults:
  types:
    basic.PageRequest: '{"index":1,"size":10}'
  fields:
    "*.tenantId": "T001"

# 脚本钩子
hooks:
  pre_request: "~/.lion/scripts/pre_sign.py"
  post_request: "~/.lion/scripts/post_check.py"
```

**环境寻址方式：**

| 模式 | 说明 |
|------|------|
| `direct` | 直连模式，使用 `host` 字段或 `--addr` 参数 |
| `consul` | 通过 Consul 服务发现自动寻址 |

### Collection Markdown

由 `generate` 自动创建，每个分组一个文件，位于 `~/.lion/api/<collection>/<group>.md`。

文件结构示例：

```markdown
# order

> source: swagger
> collection: tms
> service: tms-service

### POST /order/create `createOrder`

> script: ~/.lion/scripts/order_create.py

```json
{
  "orderId": "",
  "warehouseId": "W001",
  "items": [
    {
      "skuId": "",
      "quantity": 0
    }
  ]
}
```
```

- 接口 ID 在反引号内（如 `createOrder`）
- JSON 代码块可直接编辑，修改请求参数
- `> script:` 可配置接口级脚本（优先级高于全局 hooks）
- `> service:` 指定 Consul 服务名称

## 脚本协议

脚本通过 stdin/stdout 以 JSON 格式传递数据，支持 Python、Node.js、Shell 等语言。

脚本通过 `--action` 参数区分执行阶段：

```bash
python <script_path> --action pre_request    # 前置脚本
python <script_path> --action post_request   # 后置脚本
```

### 前置脚本（pre_request）

请求发送前执行，用于动态修改请求参数（签名、注入 Token 等）。

**输入字段（stdin）：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `api_id` | string | 接口唯一标识 |
| `method` | string | HTTP Method（如 `POST`）或 `gRPC` |
| `url` | string | 请求完整 URL |
| `headers` | map[string]string | 请求头 |
| `params` | map[string]string | Query / Path 参数 |
| `body` | object | 请求体（JSON） |

**输出字段（stdout）：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `headers` | map[string]string | 否 | 修改后的请求头（覆盖原值） |
| `params` | map[string]string | 否 | 修改后的参数（覆盖原值） |
| `body` | object | 否 | 修改后的请求体（覆盖原值） |

**示例：**

```json
// 输入
{
  "api_id": "createOrder",
  "method": "POST",
  "url": "http://127.0.0.1:8080/order/create",
  "headers": { "Content-Type": "application/json" },
  "params": {},
  "body": { "orderId": "12345" }
}

// 输出
{
  "headers": {
    "Content-Type": "application/json",
    "X-Signature": "abc123",
    "X-Timestamp": "1726639200"
  },
  "body": { "orderId": "12345" }
}
```

### 后置脚本（post_request）

请求完成后执行，用于断言、数据提取、日志记录等。

**输入字段（stdin）：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `api_id` | string | 接口唯一标识 |
| `method` | string | HTTP Method 或 `gRPC` |
| `url` | string | 请求完整 URL |
| `headers` | map[string]string | 请求头 |
| `params` | map[string]string | Query / Path 参数 |
| `body` | object | 请求体 |
| `status_code` | int | HTTP 响应状态码 |
| `resp_body` | object/string | 响应体（JSON 对象或原始字符串） |
| `elapsed_ms` | int64 | 请求耗时（毫秒） |

**输出字段（stdout）：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `assertions` | array | 否 | 断言结果列表 |
| `assertions[].name` | string | 是 | 断言名称 |
| `assertions[].pass` | bool | 是 | 是否通过 |
| `assertions[].message` | string | 是 | 断言描述信息 |
| `extracted` | map[string]string | 否 | 提取的变量（可供后续使用） |
| `output` | string | 否 | 输出信息（显示在终端） |

**示例：**

```json
// 输入
{
  "api_id": "createOrder",
  "method": "POST",
  "url": "http://127.0.0.1:8080/order/create",
  "headers": { "Content-Type": "application/json" },
  "params": {},
  "body": { "orderId": "12345" },
  "status_code": 200,
  "resp_body": { "code": 0, "data": { "orderId": "12345", "status": "CREATED" } },
  "elapsed_ms": 120
}

// 输出
{
  "assertions": [
    { "name": "code_check", "pass": true, "message": "code == 0" },
    { "name": "status_check", "pass": false, "message": "status should be PAID" }
  ],
  "extracted": { "orderId": "12345" },
  "output": "订单创建成功: 12345"
}
```

### 脚本执行流程

```
lion 构造上下文 (ScriptContext)
    │
    ▼
序列化为 JSON → 写入脚本 stdin
    │
    ▼
执行: python <script> --action pre_request|post_request
    │
    ▼
读取脚本 stdout → 反序列化为 ScriptResult
    │
    ├─ 前置脚本：用输出的 headers/params/body 覆盖原请求，然后发送
    └─ 后置脚本：展示 assertions 结果和 output 信息
```

### 脚本优先级

| 层级 | 配置位置 | 优先级 |
|------|----------|--------|
| 接口级 | Markdown 中 `> script:` | 最高 |
| 全局 | `global.yaml` 的 `hooks` | 较低 |

## 服务寻址

lion 支持两种方式确定目标服务地址：

1. **Consul 寻址** — 环境配置 `resolve: consul`，从 Markdown 元数据 `> service:` 读取服务名，通过 Consul 查询实例
2. **直连模式** — 环境配置 `resolve: direct`，使用 `host` 字段或命令行 `--addr` 参数

地址解析优先级：`--addr` > Consul > 环境 `host`

## License

MIT
