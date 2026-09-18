# lion 整体设计

lion 是一个用于发送 gRPC / HTTP 请求的命令行工具，面向微服务调试与接口联调场景。通过对接服务发现（Consul）、接口规范（Swagger / OpenAPI / gRPC 反射 / Proto 文件），实现接口自动发现、请求自动生成与一键发送。

## 核心能力

1. **接口自动生成**：根据 Swagger / OpenAPI / gRPC 反射 / Proto 文件，自动解析并生成请求信息结构体（含路径、方法、参数、Header 等）。
2. **接口分组管理**：不同的 Swagger 源、gRPC 服务建立不同的 Collection 分组，便于组织与检索。
3. **报文发送**：支持 HTTP 和 gRPC 报文发送，可结合 Consul 服务发现动态寻址，支持前置/后置脚本、默认值填充等高级能力。

---

## 项目结构

```
lion/
├── cmd/cli/          # CLI 入口
│   └── main.go
├── internal/         # 内部模块（不对外暴露）
│   ├── generate/     # 接口解析与生成（swagger / openapi / grpc / proto）
│   ├── collection/   # 接口分组（Collection）管理
│   ├── sender/       # 报文发送（HTTP / gRPC）
│   ├── config/       # 配置加载（global.yaml + Collection Markdown 双层合并）
│   ├── markdown/     # Collection Markdown 解析与生成（编排规则引擎）
│   ├── script/       # 脚本执行引擎（stdin/stdout 协议）
│   └── consul/       # Consul 服务发现客户端
├── docs/             # 设计文档
└── go.mod

~/.lion/                        # 用户配置目录
├── global.yaml                 # 全局配置（环境、脚本、全局默认值）
├── api/                        # Collection API 文档
│   └── ${collection}/          # 集合目录（如 tms、wms）
│       ├── order.md            # 按分组一个文件（HTTP: 一级路径 / gRPC: 服务名）
│       ├── inventory.md
│       └── ...
└── scripts/                    # 脚本文件目录
    ├── pre_sign.py
    └── post_check.py
```

---

## 模块设计

### 1. Generate — 接口解析与生成

负责从不同来源解析接口定义，生成统一的请求描述结构，并自动创建 Collection Markdown 文件。

| 来源类型 | 说明 |
|----------|------|
| `swagger` | 解析 Swagger 2.0 / OpenAPI 3.x JSON 或 URL |
| `grpc` | 通过 gRPC Server Reflection 获取服务与方法列表 |
| `proto` | 解析本地 `.proto` 文件，支持多目录、多 import path |

生成产物：
- 统一请求描述结构（服务名、方法、路径、参数等）
- Collection Markdown 文件（`~/.lion/api/${collection}/${group}.md`），按一级路径/gRPC服务名拆分

### 2. Collection — 接口分组管理

将生成的接口按来源组织为 Collection，每个 Collection 对应 `~/.lion/api/${collection}/` 目录，按分组拆分为多个 Markdown 文件：
- 分组名：HTTP 取一级路径（如 `/order/create` → `order`），gRPC 取服务名（如 `rbe.entity.petrel.ScheduleTask`）
- 每个文件包含该组所有接口的**完整请求参数结构**和可选的**接口级脚本配置**

Markdown 文件在 `generate` 时自动创建，用户可直接编辑修改请求参数和脚本路径。详见下方「Collection Markdown 规范」章节。

### 3. Sender — 报文发送

根据 Collection 中的接口定义，构造并发送请求。

- **HTTP 发送**：支持 GET / POST / PUT / DELETE 等方法，支持自定义 Header、Query 参数、JSON Body。
- **gRPC 发送**：通过反射或 proto 定义构造请求消息，支持 metadata 设置。
- **服务寻址**：
  - 通过 Consul 服务发现动态获取目标 IP + Port，服务名称从 Markdown 元数据 `> service:` 读取
  - 支持直连模式：优先使用 `--addr` 参数，未传时回退到配置文件中的 `host` 字段

---

## CLI 命令

### generate — 生成接口集合

```bash
# 根据 Swagger 生成名为 tms 的 API 集合
lion generate --name "tms" --type swagger "http://tms-service:8080/swagger.json"

# 根据 gRPC 反射生成名为 wms 的 API 集合，并指定 Consul 服务名
lion generate --name "wms" --type grpc "127.0.0.1:9090" --service "wms-service"

# 根据本地 proto 文件生成名为 wms 的 API 集合
#   --proto_root 指定 proto import 根目录（可多次指定）
lion generate --name "wms" --type proto "./protos" \
  --proto_root "./protos" \
  --proto_root "./third_party/protos"

# 重新生成（覆盖已有 Markdown 中的接口列表，保留用户编辑的默认值）
lion generate --name "tms" --type swagger "http://tms-service:8080/swagger.json" --force
```

**参数说明：**

| 参数 | 必填 | 说明 |
|------|------|------|
| `--name` | 是 | Collection 名称，同时决定目录路径 `api/${name}/`。分组文件名：HTTP 取一级路径（`/order/create` → `order`），gRPC 取服务名（`rbe.entity.petrel.ScheduleTask`） |
| `--type` | 是 | 来源类型：`swagger` / `grpc` / `proto` |
| `--proto_root` | 否 | proto import 根目录，仅 `--type proto` 时有效，可多次指定 |
| `--force` | 否 | 强制重新生成接口列表，但保留用户编辑的默认值配置 |
| `--service` | 否 | Consul 服务名称，写入 Markdown 元数据，用于 consul 寻址模式 |

### collections — 管理接口分组

```bash
# 列出所有分组
lion collections --list

# 列出指定分组下的所有接口
lion collections --name "tms" --list

# 查看接口详情（按接口 ID）
lion collections details <api-id>

# 删除分组
lion collections --name "tms" --delete

# 导出分组为 JSON
lion collections --name "tms" --export > tms_backup.json
```

### send — 发送请求

```bash
# 发送指定接口（使用默认值 / 交互填写参数）
lion send <api-id>

# 指定环境发送
lion send <api-id> --env dev

# 覆盖单个参数
lion send <api-id> --param "orderId=12345"

# 指定直连地址（覆盖配置中的 host）
lion send <api-id> --addr "127.0.0.1:8080"

# 不传 --addr 时，自动使用配置文件中 resolve: direct 对应的 host 值
lion send <api-id> --env local
```

---

## 配置

配置分为两层，存放在 `~/.lion/` 目录下：

| 文件 | 作用域 | 说明 |
|------|--------|------|
| `global.yaml` | 全局 | 环境、脚本路径、全局默认值规则、全局 hooks |
| `api/${collection}/${group}.md` | 分组级 | 每组一个文件，包含完整请求结构 + 接口级脚本，generate 时自动创建 |

### global.yaml — 全局配置

```yaml
# 全局设置
global:
  timeout: 30s            # 默认请求超时
  output: pretty          # 输出格式：pretty / json / raw

# 环境配置（可配置多个）
environments:
  dev:
    consul_addr: "https://consul.dev.shijizhongyun.com/"
    consul_token: "3f84201a-31a2-c843-bbc0-0a45983aa7b7"
    resolve: consul       # 寻址方式：consul / direct
  local:
    resolve: direct       # 直连模式
    host: "127.0.0.1:8080"  # 直连地址，--addr 未传时默认使用此值

# 全局默认值规则（所有 Collection 共享）
defaults:
  types:
    basic.PageRequest: '{"index":1,"size":10}'
  fields:
    "*.tenantId": "T001"
    "*.timestamp": "{{now_unix}}"

# 脚本（指定脚本文件路径）
hooks:
  pre_request: "~/.lion/scripts/pre_sign.py"    # 请求前执行
  post_request: "~/.lion/scripts/post_check.py"  # 请求后执行
```

### api/${collection}/${group}.md — 分组 Markdown

由 `generate` 自动创建，每个分组一个文件。包含该组所有接口的完整请求参数结构和接口级脚本配置。详见下方「Collection Markdown 规范」章节。

### 配置加载规则

1. 环境、脚本等基础配置从 `global.yaml` 加载
2. 默认值规则从 `global.yaml` 的 `defaults` 加载
3. 请求结构：从 `api/${collection}/${group}.md` 中对应接口的 JSON 代码块读取（用户可编辑）
4. 脚本优先级：接口级 `> script:` > 全局 `hooks`（接口级覆盖全局）
5. 若命令行传入 `--addr`，覆盖环境配置中的 `host`

---

## 发送规则

1. **环境感知**：通过 `--env` 指定目标环境，环境配置在配置文件中定义，包含 Consul 地址、Token、寻址方式等。未指定时默认使用第一个环境。
2. **参数填充优先级**：接口参数按以下优先级填充（高 → 低）：
   - 命令行 `--param` 显式传入
   - Collection Markdown 中该接口的 JSON 代码块（用户编辑后的完整请求结构）
   - `global.yaml` 中的 `defaults` 规则
   - 接口定义中的默认值（Swagger default / Proto default）
   - 空值（由用户交互补充或留空）
3. **服务寻址**：
   - `consul` 模式：通过 Consul 查询服务实例，随机或轮询选取一个节点发起请求。服务名称从 Collection Markdown 元数据的 `> service:` 字段读取（可通过 `generate --service` 设置，或手动编辑 Markdown 文件添加）。
   - `direct` 模式：直连模式。优先使用命令行 `--addr` 参数；未传时，使用配置文件中 `host` 字段的值。
4. **前置 / 后置脚本**：分全局和接口级两层。全局脚本在 `global.yaml` 的 `hooks` 中配置；接口级脚本在 Markdown 中通过 `> script:` 配置，优先级高于全局。脚本通过 stdin/stdout 传递数据，详见「脚本对接协议」章节。
5. **请求体格式**：请求体以 JSON 代码块形式存储在分组 Markdown 中，展示完整请求参数结构，用户直接编辑即可。发送时遵循最小添加数据原则，仅发送有值的字段。
6. **输出与日志**：请求结果默认以 Pretty Print 格式输出，支持 `--output json` 输出原始 JSON，支持 `--verbose` 查看完整请求/响应报文。

---

## Collection Markdown 规范

每个 Collection 对应 `~/.lion/api/${collection}/` 目录，按分组拆分为多个 Markdown 文件。由 `generate` 自动创建。

### 文件组织

```
~/.lion/api/
├── tms/                          # Collection: tms
│   ├── order.md                  # 分组: /order（HTTP 一级路径）
│   ├── inventory.md              # 分组: /inventory
│   └── ...
└── wms/                          # Collection: wms
    ├── rbe.entity.petrel.ScheduleTask.md   # 分组: gRPC 服务名
    └── ...
```

**分组规则：**
- HTTP：取请求路径第一段，如 `/order/create` → 分组 `order` → 文件 `order.md`
- gRPC：取服务名，如 `rbe.entity.petrel.ScheduleTask/List` → 分组 `rbe.entity.petrel.ScheduleTask` → 文件 `rbe.entity.petrel.ScheduleTask.md`

### 文件格式

每个 Markdown 文件包含：
- **元数据**：`> key: value` 引用块
- **接口列表**：每个接口包含完整请求参数结构 + 可选的接口级脚本

```markdown
# order

> source: swagger
> collection: tms
> service: tms-service

### POST /order/create `createOrder`

> script: ~/.lion/scripts/order_create.py

```json
{
  "orderId": "",                 // 必填
  "warehouseId": "W001",         // 默认: *.warehouseId
  "orderType": 0,                // 可选: 1=普通 2=加急
  "items": [                     // 必填
    {
      "skuId": "",               // 必填
      "quantity": 0,             // 必填
      "price": 0.00              // 可选
    }
  ],
  "page": {                      // 默认: basic.PageRequest
    "index": 1,
    "size": 10
  },
  "remark": ""                   // 可选
}
```

### GET /order/detail `getOrder`

```json
{
  "orderId": ""                  // 必填
}
```
```

gRPC 示例：

```markdown
# rbe.entity.petrel.ScheduleTask

> source: proto
> collection: wms

### rbe.entity.petrel.ScheduleTask/List `ScheduleTask/List`

> script: ~/.lion/scripts/schedule_list.py

```json
{
  "filter": "",                  // 可选
  "page": {                      // 默认: basic.PageRequest
    "index": 1,
    "size": 10
  }
}
```
```

### 编排规则

| 规则 | 说明 |
|------|------|
| **元数据** | `# {group_name}` 一级标题 + `> key: value` 引用块（`source`、`collection`、可选 `service`） |
| **接口定义** | `### {METHOD} {path} \`{api_id}\`` 三级标题，反引号内为接口 ID |
| **接口脚本** | `> script: {path}` 引用块，紧跟接口标题，指定该接口的专属脚本（可选） |
| **完整请求结构** | ` ```json ` 代码块，展示接口所有字段，用注释标注必填/默认值/可选 |
| **字段注释** | `// 必填` `// 可选: 说明` `// 默认: 规则名` 三种标注 |

### 完整请求结构

生成的 JSON 代码块展示接口的**完整请求参数结构**，包含所有字段：

| 字段情况 | 生成规则 | 注释标注 |
|----------|----------|----------|
| 有默认值 | 填入默认值 | `// 默认: 规则名`（如 `// 默认: basic.PageRequest`、`// 默认: *.warehouseId`） |
| 必填无默认值 | 填入类型零值（`""` / `0` / `false`） | `// 必填` |
| 可选无默认值 | 填入类型零值 | `// 可选: 说明` |
| 嵌套对象 | 完整展开所有子字段 | 继承父级注释 |

用户可直接编辑 JSON 代码块：修改值、添加字段、删除不需要的字段。

### 接口级脚本

每个接口可通过 `> script: {path}` 配置专属脚本，优先级高于 `global.yaml` 中的 `hooks`。

| 配置位置 | 作用范围 | 优先级 |
|----------|----------|--------|
| `> script:` 在接口标题下方 | 仅该接口 | 最高 |
| `global.yaml` 的 `hooks` | 所有接口 | 较低 |

脚本对接协议与全局 hooks 相同（stdin/stdout JSON），详见下方「脚本对接协议」章节。

---

## 脚本对接协议

脚本以**文件路径**方式配置，分全局和接口级两层。lion 在执行时调用脚本解释器运行脚本文件，通过 stdin/stdout 传递数据。

- **全局脚本**：`global.yaml` 的 `hooks.pre_request` / `hooks.post_request`，对所有接口生效
- **接口级脚本**：Markdown 中 `> script: {path}`，仅对该接口生效，**优先级高于全局**

### 对接接口

#### 前置脚本（pre_request）

请求发送前执行，用于动态修改请求参数（签名、注入 Token、时间戳等）。

**调用方式：**
```bash
python <script_path> --action pre_request
```

**输入（stdin）：** JSON 格式的请求上下文
```json
{
  "api_id": "tms.getOrder",
  "method": "POST",
  "url": "http://192.168.1.100:8080/api/order/get",
  "headers": {
    "Content-Type": "application/json"
  },
  "params": {
    "orderId": "12345"
  },
  "body": {
    "warehouseId": "W001"
  }
}
```

**输出（stdout）：** JSON 格式的修改后请求上下文
```json
{
  "headers": {
    "Content-Type": "application/json",
    "X-Signature": "abc123...",
    "X-Timestamp": "1726639200"
  },
  "params": {
    "orderId": "12345"
  },
  "body": {
    "warehouseId": "W001"
  }
}
```

#### 后置脚本（post_request）

请求完成后执行，用于响应断言、数据提取、日志记录等。

**调用方式：**
```bash
python <script_path> --action post_request
```

**输入（stdin）：** JSON 格式的响应上下文
```json
{
  "api_id": "tms.getOrder",
  "status_code": 200,
  "headers": {
    "Content-Type": "application/json"
  },
  "body": {
    "code": 0,
    "data": { "orderId": "12345", "status": "CREATED" }
  },
  "elapsed_ms": 120
}
```

**输出（stdout）：** JSON 格式的处理结果
```json
{
  "assertions": [
    { "name": "code_check", "pass": true, "message": "code == 0" }
  ],
  "extracted": {
    "orderId": "12345"
  },
  "output": "订单创建成功: 12345"
}
```

### 脚本接口定义

```go
// ScriptContext 脚本上下文，传递给脚本的完整信息
type ScriptContext struct {
    APIID      string            `json:"api_id"`
    Method     string            `json:"method"`      // HTTP Method
    URL        string            `json:"url"`         // 请求 URL
    Headers    map[string]string `json:"headers"`     // 请求头
    Params     map[string]string `json:"params"`      // Query / Path 参数
    Body       interface{}       `json:"body"`        // 请求体
    // 后置脚本额外字段
    StatusCode int               `json:"status_code,omitempty"`
    RespBody   interface{}       `json:"resp_body,omitempty"`
    ElapsedMs  int64             `json:"elapsed_ms,omitempty"`
}

// ScriptResult 脚本执行结果
type ScriptResult struct {
    Headers    map[string]string `json:"headers,omitempty"`    // 修改后的请求头（前置）
    Params     map[string]string `json:"params,omitempty"`     // 修改后的参数（前置）
    Body       interface{}       `json:"body,omitempty"`        // 修改后的请求体（前置）
    Assertions []Assertion       `json:"assertions,omitempty"` // 断言结果（后置）
    Extracted  map[string]string `json:"extracted,omitempty"`  // 提取的变量（后置）
    Output     string            `json:"output,omitempty"`     // 输出信息
}

// Assertion 断言结果
type Assertion struct {
    Name    string `json:"name"`
    Pass    bool   `json:"pass"`
    Message string `json:"message"`
}
```

### 脚本执行流程

```
lion 构造 ScriptContext
    │
    ▼
序列化为 JSON → 写入脚本 stdin
    │
    ▼
执行: python <script_path> --action pre_request|post_request
    │
    ▼
读取脚本 stdout → 反序列化为 ScriptResult
    │
    ├─ 前置脚本：用 ScriptResult 中的 headers/params/body 覆盖原请求
    └─ 后置脚本：展示 assertions、保存 extracted 变量供后续使用
```

---

## 数据模型

### Collection（接口集合，对应 `api/${collection}/` 目录）

```go
type Collection struct {
    Name      string   `json:"name"`       // 集合名称（目录名）
    Source    string   `json:"source"`     // 来源类型：swagger / grpc / proto
    SourceURL string   `json:"source_url"` // 来源地址
    Groups    []*Group `json:"groups"`     // 分组列表，每组对应一个 .md 文件
}
```

### Group（分组，对应一个 Markdown 文件）

```go
type Group struct {
    Name string  `json:"name"` // 分组名（文件名，如 "order"、"rbe.entity.petrel.ScheduleTask"）
    Path string  `json:"path"` // Markdown 文件完整路径
    APIs []*API `json:"apis"`  // 该分组下的接口列表
}
```

### API（单个接口）

```go
type API struct {
    ID          string `json:"id"`           // 唯一标识（反引号内内容）
    Method      string `json:"method"`       // HTTP Method 或 gRPC 方法名
    Path        string `json:"path"`         // 请求路径或全限定方法名
    BodyJSON    string `json:"body_json"`    // 完整请求结构 JSON（代码块内容，用户可编辑）
    Script      string `json:"script"`       // 接口级脚本路径（空则使用全局 hooks）
    Description string `json:"description"` // 接口描述
}
```

### DefaultRule（默认值规则，在 global.yaml 中配置）

```go
type DefaultRule struct {
    Types  map[string]string `json:"types,omitempty"`  // 按类型名匹配，如 "basic.PageRequest": '{"index":1,"size":10}'
    Fields map[string]string `json:"fields,omitempty"` // 按字段名匹配，如 "*.tenantId": "T001"
}
```

### GlobalConfig（全局配置，对应 global.yaml）

```go
type GlobalConfig struct {
    Global       GlobalSettings          `json:"global"`
    Environments map[string]*Environment `json:"environments"`
    Defaults     *DefaultRule            `json:"defaults"`         // 全局默认值规则
    Hooks        *Hooks                  `json:"hooks,omitempty"` // 全局脚本钩子
}

type GlobalSettings struct {
    Timeout string `json:"timeout"` // 默认请求超时
    Output  string `json:"output"`  // 输出格式：pretty / json / raw
}
```

---

## 设计原则

1. **约定优于配置**：接口定义从标准规范（Swagger / Proto）自动生成，减少手动配置。
2. **可读性优先**：分组 Markdown 展示完整请求结构，用户可直接阅读和编辑，程序通过结构化规则解析。
3. **最小发送数据**：展示完整结构供参考，实际发送时仅包含有值的字段，不填充冗余数据。
4. **环境隔离**：多环境配置独立，切换环境不影响 Collection 定义。
5. **分层拆分**：按一级路径 / gRPC 服务名拆分为多个小文件，避免单文件过大；脚本分全局和接口级两层，接口级覆盖全局。
6. **可扩展**：Generate / Collection / Sender 模块解耦；默认值规则支持按类型和按字段名两种匹配方式；脚本通过标准 stdin/stdout 协议对接，不限定语言。