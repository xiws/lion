# lion

lion 是一个用于发送 gRPC / HTTP 请求的命令行工具，面向微服务调试与接口联调场景。通过对接服务发现（Consul）、接口规范（Swagger / gRPC 反射），实现接口自动发现、请求自动生成与一键发送。

## 核心特性

- **接口自动生成** — 从 Swagger / OpenAPI 文档或 gRPC Server Reflection 自动解析并生成接口定义和请求结构
- **Collection 分组管理** — 按服务来源组织接口，支持列表、详情查看、导出和删除
- **HTTP & gRPC 发送** — 支持 GET / POST / PUT / DELETE 等 HTTP 方法及 gRPC 动态调用（基于 Server Reflection）
- **Consul 服务发现** — 集成 Consul 自动寻址，也支持直连模式
- **多环境配置** — 独立管理 dev / staging / prod 等多套环境，一键切换
- **脚本钩子** — 支持前置 / 后置脚本（Python / Node.js / Shell），用于签名、Token 注入、断言等
- **Markdown 驱动** — 接口定义以 Markdown 文件存储，可直接编辑请求参数，可读可编辑
- **默认值规则** — 按类型名或字段名全局填充默认值，减少重复配置

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

---

## 快速开始

### 1. 初始化配置目录

```bash
lion init
```

这会在 `~/.lion/` 下创建默认目录结构和配置文件：

```
~/.lion/
├── global.yaml      # 全局配置（环境、超时、脚本钩子、默认值规则等）
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
# 指定环境发送
lion send <api-id> --env dev

# 直连发送
lion send <api-id> --addr "127.0.0.1:8080"

# 覆盖参数
lion send <api-id> --env dev --param "orderId=12345"
```

---

## 配置详解

lion 的配置分为三层：**全局配置** (`global.yaml`)、**Collection 配置** (`<collection>.yaml`，可选) 和 **Collection Markdown**（接口定义）。

### 配置目录结构

```
~/.lion/
├── global.yaml                          # 全局配置
├── petrel.yaml                          # Collection 级配置（可选，覆盖 global.yaml）
├── api/                                 # Collection 接口定义
│   ├── petstore/                        # Collection: petstore
│   │   ├── pet.md                       # 分组: /pet（HTTP 一级路径）
│   │   └── store.md                     # 分组: /store
│   └── wms/                             # Collection: wms
│       └── rbe.entity.petrel.Task.md    # 分组: gRPC 服务名
└── scripts/                             # 脚本文件目录
    ├── pre_sign.py
    └── post_check.py
```

### global.yaml — 全局配置

`~/.lion/global.yaml` 是 lion 的核心配置文件，包含以下配置块：

```yaml
# ─────────────────────────────────────────────
# 1. 全局设置
# ─────────────────────────────────────────────
global:
  timeout: 30s                  # 请求超时时间（Go duration 格式，如 30s、1m、500ms）
  output: pretty                # 默认输出格式：pretty / json / raw
  script_runners:               # 脚本执行器路径配置（可选）
    python: "/usr/bin/python3"  #   Python 解释器路径
    node: "/usr/bin/node"       #   Node.js 解释器路径
    bash: "/bin/bash"           #   Bash 解释器路径
  headers:                      # 全局固定请求头 / gRPC metadata（可选）
    X-Client: lion

# ─────────────────────────────────────────────
# 2. 环境配置（可配置多个）
# ─────────────────────────────────────────────
environments:
  dev:
    resolve: direct             # 寻址方式：direct / consul
    host: "127.0.0.1:8080"     # 直连地址（resolve: direct 时使用）
    headers:                    # 环境级固定请求头 / gRPC metadata（可选）
      x-tenant-id: "T001"
  staging:
    resolve: consul             # 通过 Consul 服务发现寻址
    consul_addr: "https://consul.staging.example.com"
    consul_token: "your-token"
  prod:
    resolve: consul
    consul_addr: "https://consul.prod.example.com"
    consul_token: "prod-token"

# ─────────────────────────────────────────────
# 3. 默认值规则（所有 Collection 共享）
# ─────────────────────────────────────────────
defaults:
  types:                        # 按类型名匹配 — 当字段类型包含此 key 时，用 value 填充
    basic.PageRequest: '{"index":1,"size":10}'
    common.Pagination: '{"page":1,"limit":20}'
  fields:                       # 按字段名匹配 — 当字段名匹配此 pattern 时，用 value 填充
    "*.tenantId": "T001"       #   * 表示匹配任意前缀
    "*.timestamp": "1726639200"
    "*.operatorId": "admin"

# ─────────────────────────────────────────────
# 4. 全局脚本钩子
# ─────────────────────────────────────────────
hooks:
  pre_request: "~/.lion/scripts/pre_sign.py"     # 请求前执行（签名、Token 注入等）
  post_request: "~/.lion/scripts/post_check.py"  # 请求后执行（断言、日志记录等）
```

#### 配置项说明

| 配置路径 | 类型 | 默认值 | 说明 |
|----------|------|--------|------|
| `global.timeout` | string | `30s` | 请求超时时间，支持 Go duration 格式（`30s`、`1m`、`500ms`） |
| `global.output` | string | `pretty` | 默认输出格式：`pretty`（美化 JSON）/ `json`（原始 JSON）/ `raw`（原始响应） |
| `global.script_runners` | map | 空 | 脚本解释器路径映射，key 为语言名，value 为解释器绝对路径 |
| `global.headers` | map | 空 | 所有环境共享的固定请求头 / gRPC metadata |
| `environments` | map | 空 | 环境配置表，key 为环境名 |
| `environments.*.resolve` | string | 无（必填） | 寻址方式：`direct`（直连）或 `consul`（服务发现） |
| `environments.*.host` | string | 空 | 直连地址，`resolve: direct` 时使用 |
| `environments.*.consul_addr` | string | 空 | Consul 地址，`resolve: consul` 时使用 |
| `environments.*.consul_token` | string | 空 | Consul ACL Token |
| `environments.*.headers` | map | 空 | 当前环境固定请求头 / gRPC metadata，覆盖同一文件中的 `global.headers` |
| `defaults.types` | map | 空 | 按类型名匹配的默认值，key 为类型名，value 为 JSON 字符串 |
| `defaults.fields` | map | 空 | 按字段名匹配的默认值，key 为 `*.fieldName` 格式 pattern，value 为默认值 |
| `hooks.pre_request` | string | 空 | 全局前置脚本路径 |
| `hooks.post_request` | string | 空 | 全局后置脚本路径 |

#### 固定 Header / gRPC Metadata

需要每次请求都带上的固定 header（gRPC 即 metadata），可以写在 `global.yaml` 里，不用每次 `-H`，也不用写脚本：

```yaml
global:
  headers:                  # 所有环境共享
    X-Client: lion
    x-tenant-id: "T001"

environments:
  dev:
    headers:                # 仅 dev 环境，覆盖 global.headers 中的同名 key
      authorization: "Bearer dev-token"
  test:
    headers:
      authorization: "Bearer test-token"
```

优先级（低 → 高）：

```text
global.headers  <  environments.<env>.headers  <  --header/-H
```

配置了 Collection 级 `<collection>.yaml` 时，完整优先级见下文「Collection 级配置」。

示例：

```bash
# 使用 dev 环境时，最终会带上 X-Client、x-tenant-id、authorization
lion send rbe.userc.quokka.User/List --env dev

# 临时覆盖配置中的 x-tenant-id
lion send rbe.userc.quokka.User/List --env dev -H "x-tenant-id=T002"
```

说明：

- HTTP 接口会把这些值作为请求头发送；gRPC 接口会作为 metadata 发送，key 在发送时统一转成小写
- `--header` 支持 `key=value` 或 `key: value`，可多次指定，优先级最高
- 前置脚本收到的是合并后的 headers；如果脚本返回 `headers`，会**整体覆盖**当前 headers，所以脚本需要保留配置里的固定 header（`auto_auth.py` 这种读取原 headers 再追加的方式不会丢）
- header 名按大小写不敏感处理，`X-Tenant-Id` 和 `x-tenant-id` 会视为同一个 key

#### Collection 级配置（`<collection>.yaml`）

除了全局 `global.yaml`，每个 Collection 还可以有自己的一份配置：

```text
~/.lion/petrel.yaml      # 对应 ~/.lion/api/petrel/ 这个 Collection
~/.lion/quokka.yaml
```

文件结构与 `global.yaml` 完全一致，**只需要写要覆盖的字段**，不用把整份配置复制一遍。合并规则是 Collection 配置覆盖全局配置，未写的字段继续继承 `global.yaml`。例如：

```yaml
# ~/.lion/petrel.yaml
global:
  headers:
    service: "bsi.rbe.entity.petrel"
    bsi-instance: "RBE_AA"
    debugkey: "aK8#pW7zN1qR5eL"

environments:
  dev:
    headers:
      x-tenant-id: "T002"   # 仅 petrel + dev 环境，覆盖上层同名 key
```

这样只有 `petrel` 集合的请求会带这三个 metadata；`dev` 环境的请求还会额外带上 `x-tenant-id`，其他集合不受影响。

合并细节：

- `headers`：按 `global.yaml` 全局 < `global.yaml` 环境 < `<collection>.yaml` 全局 < `<collection>.yaml` 环境逐层合并，后层覆盖前层的同名 key（大小写不敏感）；`<collection>.yaml` 的 `global.headers` 会应用到该 Collection 的所有环境
- `environments.*`：按环境名深合并；Collection 只写 `headers` 时，仍继承全局的 `host` / `consul_addr` / `consul_token`
- `hooks`：Collection 写了 `pre_request` 或 `post_request` 就覆盖对应的全局 hook，没写的继续用全局
- `defaults` / `script_runners`：按 key 合并

最终 header / metadata 优先级：

```text
global.yaml 的 global.headers
  < global.yaml 的 environments.<env>.headers
  < <collection>.yaml 的 global.headers
  < <collection>.yaml 的 environments.<env>.headers
  < --header/-H
```

例如四层分别把 `x-tenant-id` 配成 `T000`、`T001`、`T002`、`T003`，`--env dev` 时最终发送 `T003`；再加 `-H x-tenant-id=T004`，最终发送 `T004`。

可以用 `lion config show petrel` 查看合并后的配置；其中 `<collection>.yaml` 的 `global.headers` 会显示在每个环境的固定 Headers 中。

#### 默认值规则详解

默认值规则在 `generate` 生成接口定义时生效，决定 JSON 代码块中字段的初始值：

**`defaults.types` — 按类型匹配：**
- key 是 proto/Swagger 中的类型名（如 `basic.PageRequest`）
- 当字段的类型名**包含**此 key 时，用 value（JSON 字符串）作为该字段的默认值
- 适用于嵌套对象类型的批量填充

```yaml
defaults:
  types:
    basic.PageRequest: '{"index":1,"size":10}'
```
生成时，类型为 `basic.PageRequest` 的字段会自动填入 `{"index":1,"size":10}`。

**`defaults.fields` — 按字段名匹配：**
- key 是 `*.fieldName` 格式的 pattern，`*` 匹配任意前缀
- 当字段名匹配此 pattern 时，用 value 作为默认值
- 适用于通用字段（如 `tenantId`、`timestamp`）的全局填充

```yaml
defaults:
  fields:
    "*.tenantId": "T001"
```
生成时，所有名为 `tenantId` 的字段会自动填入 `"T001"`。

#### 脚本执行器解析链

lion 通过以下优先级链确定脚本解释器路径：

```
1. global.yaml 的 script_runners 配置（最高优先级）
      ↓ 未配置
2. 环境变量 LION_PYTHON / LION_NODE / LION_BASH
      ↓ 未设置
3. 系统 PATH 中查找（python3 → python / node / bash）
      ↓ 未找到
4. 回退到默认命令名（python3 / node / bash）
```

配置示例：
```yaml
global:
  script_runners:
    python: "/opt/homebrew/bin/python3"
    node: "/usr/local/bin/node"
```

或通过环境变量：
```bash
export LION_PYTHON=/opt/homebrew/bin/python3
export LION_NODE=/usr/local/bin/node
```

### Collection Markdown — 接口定义

由 `generate` 自动创建，每个分组一个文件，位于 `~/.lion/api/<collection>/<group>.md`。

#### 文件格式

```markdown
# order

> source: swagger
> source_type: swagger
> source_url: http://tms-service:8080/swagger.json
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

### GET /order/detail `getOrder`

```json
{
  "orderId": ""
}
```
```

#### Markdown 元素说明

| 元素 | 格式 | 必填 | 说明 |
|------|------|------|------|
| 分组标题 | `# {group_name}` | 是 | 一级标题，取一级路径（HTTP）或服务名（gRPC） |
| `source` | `> source: {type}` | 是 | 来源类型：`swagger` / `grpc` / `proto` |
| `source_type` | `> source_type: {type}` | 否 | 生成时使用的 `--type`，供 `--update` 还原参数 |
| `source_url` | `> source_url: {url}` | 否 | 生成时使用的来源地址，供 `--update` 还原参数 |
| `proto_root` | `> proto_root: {dir}` | 否 | proto import 根目录（空格分隔），仅 `--type proto` 时写入 |
| `collection` | `> collection: {name}` | 是 | Collection 名称 |
| `service` | `> service: {name}` | 否 | Consul 服务名称，`resolve: consul` 时必填 |
| 接口标题 | `### {METHOD} {path} \`{api_id}\`` | 是 | 三级标题，反引号内为接口唯一 ID |
| 接口脚本 | `> script: {path}` | 否 | 该接口的专属脚本路径，优先级高于全局 `hooks` |
| 请求体 | ` ```json ... ``` ` | 是 | JSON 代码块，包含完整请求参数结构，**用户可直接编辑** |

#### 分组规则

- **HTTP**：取请求路径第一段，如 `/order/create` → 分组 `order` → 文件 `order.md`
- **gRPC**：取服务名，如 `rbe.entity.petrel.ScheduleTask/List` → 分组 `rbe.entity.petrel.ScheduleTask` → 文件 `rbe.entity.petrel.ScheduleTask.md`

#### 手动编辑

用户可以直接编辑 Markdown 文件中的 JSON 代码块来修改请求参数。`generate --force` 重新生成时会保留用户编辑的 JSON 值和脚本配置。

---

## 脚本编写指南

脚本通过 stdin/stdout 以 JSON 格式传递数据，支持 Python、Node.js、Shell 等任意语言。lion 调用脚本解释器运行脚本文件，并通过 `--action` 参数区分执行阶段。

### 脚本配置方式

脚本分两层配置，**接口级优先**：

| 层级 | 配置位置 | 作用范围 | 优先级 |
|------|----------|----------|--------|
| 接口级 | Markdown 中 `> script: {path}` | 仅该接口 | **最高** |
| 全局 | `global.yaml` 的 `hooks` | 所有接口 | 较低 |

当接口配置了 `> script:` 时，该脚本**同时作为前置和后置脚本**，通过 `--action` 参数区分阶段。全局的 `pre_request` 和 `post_request` 可以分别指定不同脚本。

### 脚本调用方式

lion 根据脚本文件扩展名自动选择解释器：

| 扩展名 | 解释器 | 调用命令 |
|--------|--------|----------|
| `.py` | Python | `python <script> --action <action>` |
| `.js` | Node.js | `node <script> --action <action>` |
| `.sh` | Bash | `bash <script> --action <action>` |
| 其他 | 直接执行 | `<script> --action <action>` |

其中 `<action>` 为 `pre_request`（前置）或 `post_request`（后置）。

### 前置脚本（pre_request）

请求发送前执行，用于动态修改请求参数（签名、注入 Token、时间戳等）。

**输入（stdin）：** lion 将当前请求上下文以 JSON 写入 stdin。

| 字段 | 类型 | 说明 |
|------|------|------|
| `api_id` | string | 接口唯一标识 |
| `method` | string | HTTP Method（如 `POST`）或 `gRPC` |
| `url` | string | 请求完整 URL |
| `headers` | map[string]string | 请求头 |
| `params` | map[string]string | Query / Path 参数 |
| `body` | object | 请求体（JSON） |

**输出（stdout）：** 脚本将修改后的请求上下文写入 stdout（仅输出需要覆盖的字段即可）。

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `headers` | map[string]string | 否 | 修改后的请求头（覆盖原值） |
| `params` | map[string]string | 否 | 修改后的参数（覆盖原值） |
| `body` | object | 否 | 修改后的请求体（覆盖原值） |

### 后置脚本（post_request）

请求完成后执行，用于断言、数据提取、日志记录等。

**输入（stdin）：** 除请求上下文外，还包含响应信息。

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

**输出（stdout）：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `assertions` | array | 否 | 断言结果列表 |
| `assertions[].name` | string | 是 | 断言名称 |
| `assertions[].pass` | bool | 是 | 是否通过 |
| `assertions[].message` | string | 是 | 断言描述信息 |
| `extracted` | map[string]string | 否 | 提取的变量（可供后续使用） |
| `output` | string | 否 | 输出信息（显示在终端） |

### 完整脚本示例

#### Python 示例 — 签名 + 断言

`~/.lion/scripts/auth_hook.py`：

```python
#!/usr/bin/env python3
import sys
import json
import time
import hashlib
import argparse

def pre_request(data):
    """请求前处理：注入 Token 和签名"""
    # 读取 stdin
    data = json.loads(sys.stdin.read())

    # 注入 Authorization header
    headers = data.get("headers", {})
    headers["Authorization"] = "Bearer my-token-xxx"
    headers["X-Timestamp"] = str(int(time.time()))

    # 计算签名
    body_str = json.dumps(data.get("body", {}), sort_keys=True)
    sign = hashlib.md5((body_str + headers["X-Timestamp"]).encode()).hexdigest()
    headers["X-Signature"] = sign

    # 输出修改后的请求上下文
    print(json.dumps({
        "headers": headers,
        "params": data.get("params", {}),
        "body": data.get("body", {})
    }))

def post_request(data):
    """请求后处理：断言响应"""
    data = json.loads(sys.stdin.read())

    assertions = []

    # 断言状态码
    assertions.append({
        "name": "status_code",
        "pass": data.get("status_code") == 200,
        "message": f"status_code = {data.get('status_code')}"
    })

    # 断言业务码
    resp = data.get("resp_body", {})
    if isinstance(resp, dict):
        assertions.append({
            "name": "business_code",
            "pass": resp.get("code") == 0,
            "message": f"code = {resp.get('code')}"
        })

    # 输出结果
    print(json.dumps({
        "assertions": assertions,
        "output": f"请求完成，耗时 {data.get('elapsed_ms', 0)}ms"
    }))

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--action", required=True)
    args = parser.parse_args()

    if args.action == "pre_request":
        pre_request(sys.stdin.read())
    elif args.action == "post_request":
        post_request(sys.stdin.read())
```

#### Node.js 示例 — Token 注入

`~/.lion/scripts/token_hook.js`：

```javascript
#!/usr/bin/env node
const args = process.argv.slice(2);
const actionIdx = args.indexOf('--action');
const action = args[actionIdx + 1];

let input = '';
process.stdin.on('data', chunk => input += chunk);
process.stdin.on('end', () => {
    const data = JSON.parse(input);

    if (action === 'pre_request') {
        // 注入 Token
        const headers = data.headers || {};
        headers['Authorization'] = 'Bearer my-token-xxx';
        console.log(JSON.stringify({
            headers: headers,
            params: data.params || {},
            body: data.body || {}
        }));
    } else if (action === 'post_request') {
        // 断言
        const resp = data.resp_body || {};
        const pass = resp.code === 0;
        console.log(JSON.stringify({
            assertions: [{
                name: 'code_check',
                pass: pass,
                message: `code = ${resp.code}`
            }],
            output: `请求完成，耗时 ${data.elapsed_ms}ms`
        }));
    }
});
```

#### Shell 示例 — 简单日志

`~/.lion/scripts/log_hook.sh`：

```bash
#!/bin/bash

ACTION=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --action) ACTION="$2"; shift 2 ;;
        *) shift ;;
    esac
done

# 读取 stdin
INPUT=$(cat)

if [ "$ACTION" = "post_request" ]; then
    # 提取 api_id 和 status_code 输出日志
    API_ID=$(echo "$INPUT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('api_id',''))")
    STATUS=$(echo "$INPUT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('status_code',''))")
    echo "{\"output\": \"[$API_ID] status=$STATUS\"}"
else
    # 前置脚本不做修改，原样输出
    echo "$INPUT"
fi
```

### 脚本错误处理

- 脚本文件不存在：lion 输出警告 `⚠️ 前置/后置脚本警告: 脚本文件不存在: <path>`，**请求继续发送**
- 脚本执行失败（非零退出码）：输出 stderr 内容作为警告，**请求继续发送**
- 脚本输出非法 JSON：输出解析错误作为警告，**请求继续发送**

> 脚本错误不会中断请求流程，仅作为警告提示。

### 使用 `--verbose` 排查脚本

`--verbose` 除了打印请求报文，还会输出每个前置 / 后置脚本的执行详情：脚本实际返回了哪些字段、headers（gRPC 为 metadata）/ params / body 的前后变化，以及脚本执行完真正发出的请求。配合脚本报错信息，可以快速判断是脚本没返回字段、字段被覆盖，还是请求头被误删。

```bash
lion send createOrder --env dev --verbose
```

输出示例（节选）：

```
── 请求（脚本执行前） ──────────────────────────
POST http://127.0.0.1:8080/api/order/create
Headers（2）:
  Content-Type: application/json
  X-CLI: cli
Body:
{
  "amount": 0
}
──────────────────────────────────

── 前置脚本（pre_request，2 个脚本，共 96ms） ──────────────
   接口: createOrder
  1. ✅ ~/.lion/scripts/sign.py（48ms）
     📤 返回字段: headers, params, body, output
     📝 已注入签名
     🔧 Headers（2 处变化）:
          + X-Sign = "sig-123"
          ~ Authorization: "Bearer old" → "Bearer new"
     🔧 Params（1 处变化）:
          + traceId = "trace-1"
     🔧 Body（1 处变化）:
          ~ amount: 0 → 42
  2. ✅ ~/.lion/scripts/token.py（47ms）
     📤 返回字段: headers
     🔧 Headers（1 处变化）:
          - X-Sign（原值 "sig-123"）
──────────────────────────────────────────────

── 最终请求（脚本执行后） ──────────────────────────
POST http://127.0.0.1:8080/api/order/create?traceId=trace-1
Headers（3）:
  Authorization: Bearer new
  Content-Type: application/json
  X-CLI: cli
Body:
{
  "amount": 42
}
──────────────────────────────────
```

说明：

- `+` 表示新增、`~` 表示修改、`-` 表示删除（括号内为删除前的值）
- `接口` 是传给脚本的 `api_id`，便于核对脚本里的接口前缀 / 集合判断条件
- `返回字段` 是脚本 stdout 中实际出现的字段；如果这里没有 `headers` / `params` / `body`，说明脚本没有返回对应修改
- `🪵 stderr` 会展示脚本写到 stderr 的诊断日志，**即使脚本正常退出也会显示**；如果脚本「返回了 headers 但没有任何变化」，优先看这里是否有登录失败、跳过鉴权、找不到配置等日志
- 前置脚本返回 `headers` / `params` / `body` 时，会**整体覆盖**对应字段，未返回的字段保持不变；要删除某个 header，就在返回的 `headers` 中不再包含它
- `最终请求` 是脚本执行后真正发送的报文，包含 Query 参数、最终 headers 和 body；gRPC 接口对应显示为 `Metadata`（key 会被统一转成小写）
- 没有前置脚本时只打印一个 `请求` 块，不会重复输出
- 脚本执行失败时，`--verbose` 会显示失败脚本的 stderr 以及「后续脚本已中止」，同时仍打印最终请求，便于确认实际发出了什么
- 后置脚本会显示返回字段、`output`、断言结果和 `extracted`；后置脚本返回的 `headers` / `params` / `body` 只会在「返回字段」中列出，不会修改已发送的请求

---

## 请求生命周期

理解 lion 发送请求的完整流程，有助于排查问题和编写脚本：

```
lion send <api-id> --env dev
    │
    ▼
┌─────────────────────────────────────────────┐
│ 1. 加载全局配置 (~/.lion/global.yaml)         │
│    - 读取环境、超时、输出格式、默认值规则等      │
└─────────────────┬───────────────────────────┘
                  ▼
┌─────────────────────────────────────────────┐
│ 2. 查找接口                                  │
│    - 遍历 ~/.lion/api/*/ 下所有 .md 文件      │
│    - 按 api-id 匹配接口                       │
│    - 读取该接口的 JSON 代码块作为请求体          │
└─────────────────┬───────────────────────────┘
                  ▼
┌─────────────────────────────────────────────┐
│ 3. 解析目标地址                                │
│    - --addr 参数 > Consul 服务发现 > 环境 host │
│    - 拼接完整 URL: http://{host}{path}        │
└─────────────────┬───────────────────────────┘
                  ▼
┌─────────────────────────────────────────────┐
│ 4. 应用命令行参数覆盖                          │
│    - --param key=value 覆盖请求体中的字段       │
└─────────────────┬───────────────────────────┘
                  ▼
┌─────────────────────────────────────────────┐
│ 5. 执行前置脚本 (pre_request)                 │
│    - 解析脚本路径: 接口级 > 全局 hooks          │
│    - 构造 ScriptContext → stdin               │
│    - 执行脚本 → 读取 stdout                    │
│    - 用输出的 headers/params/body 覆盖原请求    │
└─────────────────┬───────────────────────────┘
                  ▼
┌─────────────────────────────────────────────┐
│ 6. 发送请求                                   │
│    - HTTP: 构造 http.Request 并发送            │
│    - gRPC: 通过反射动态调用                    │
│    - 记录耗时                                  │
└─────────────────┬───────────────────────────┘
                  ▼
┌─────────────────────────────────────────────┐
│ 7. 执行后置脚本 (post_request)                │
│    - 构造包含响应的 ScriptContext → stdin      │
│    - 执行脚本 → 读取 stdout                    │
│    - 展示 assertions 结果和 output 信息        │
└─────────────────┬───────────────────────────┘
                  ▼
┌─────────────────────────────────────────────┐
│ 8. 输出响应                                   │
│    - pretty: 格式化 JSON 输出                  │
│    - json: 原始 JSON                          │
│    - raw: 原始响应体                           │
└─────────────────────────────────────────────┘
```

### 参数填充优先级

请求体中的字段值按以下优先级填充（高 → 低）：

| 优先级 | 来源 | 说明 |
|--------|------|------|
| 1（最高） | `--param key=value` | 命令行显式传入，覆盖 JSON 中对应字段 |
| 2 | Markdown JSON 代码块 | 用户编辑后的值（`generate` 后手动修改的） |
| 3 | `defaults` 规则 | `generate` 时按类型/字段名匹配的默认值 |
| 4 | 类型零值 | `""` / `0` / `false`（Swagger/Proto 无默认值时） |

### 服务寻址优先级

```
--addr 参数（最高）
    ↓ 未指定
Consul 服务发现（resolve: consul 时）
    ↓ 非 consul 模式
环境 host 字段（resolve: direct 时）
    ↓ 未配置
报错：无法确定目标地址
```

---

## CLI 命令参考

### `lion init`

初始化配置目录和默认配置文件。

```bash
lion init
```

创建 `~/.lion/` 目录及 `global.yaml` 默认配置（timeout: 30s, output: pretty，空环境列表）。

### `lion generate`

根据 Swagger / gRPC 反射 / Proto 文件生成接口集合。

```bash
lion generate [source] --name <name> --type <type> [flags]
```

| 参数 | 必填 | 说明 |
|------|------|------|
| `[source]` | 首次生成必填 | 来源地址（Swagger URL / gRPC 地址 / Proto 目录路径），`--update` 时可省略 |
| `--name` | 是 | Collection 名称，决定目录路径 `api/{name}/` |
| `--type` | 首次生成必填 | 来源类型：`swagger` / `grpc` / `proto`，`--update` 时可省略 |
| `--proto_root` | 否 | proto import 根目录（可多次指定），仅 `--type proto` 时有效 |
| `--force` | 否 | 整体覆盖：重新拉取接口列表并覆盖文件，**不**保留用户编辑 |
| `--update` | 否 | 更新已有 Collection：来源信息从元数据自动读取，合并保留用户编辑 |
| `--service` | 否 | Consul 服务名称，写入 Markdown 元数据 `> service:` |

**示例：**

```bash
# 从 Swagger 生成
lion generate --name petstore --type swagger "https://petstore.swagger.io/v2/swagger.json"

# 从 gRPC 反射生成，指定 Consul 服务名
lion generate --name wms --type grpc "127.0.0.1:9090" --service wms-service


# 更新已存在的 Collection：无需重复指定 --type 和 source
lion generate --name wms --update

# 整体覆盖（丢弃用户在 JSON 代码块中的编辑）
lion generate --name petstore --type swagger "https://petstore.swagger.io/v2/swagger.json" --force
```

**`--update` / 普通生成行为：** 重新从源拉取接口列表并更新文件，但对已存在的接口 ID，保留用户在 JSON 代码块中编辑的值和 `> script:` 配置。新增的接口使用默认值规则填充。

**`--force` 行为：** 重新从源拉取接口列表并**整体覆盖**文件，不保留用户编辑的 JSON 值与脚本配置。

**`--update` 行为：** 首次生成时，来源信息会写入 Markdown 元数据：

```markdown
# wms

> source: grpc
> source_type: grpc
> source_url: 127.0.0.1:9090
> proto_root: ./protos ./third_party/protos
> collection: wms
> service: wms-service
```

`--update` 会读回 `source_type` / `source_url` / `proto_root` / `service`，因此只需 `--name` 即可重新拉取接口列表，并按上述合并规则保留用户编辑。显式传入的 `--type` / `source` / `--proto_root` / `--service` 优先于元数据；`--update` 不能与 `--force` 同时使用。若目标 Collection 不存在，或元数据中缺少来源地址（旧版本生成的文件），会报错并提示先做首次生成或显式传参。

### `lion collections`

管理接口分组。

```bash
# 列出所有 Collection
lion collections --list

# 列出指定 Collection 下的所有接口
lion collections --name <name> --list

# 查看接口详情（含请求体 JSON）
lion collections details <api-id>

# 删除 Collection（删除整个目录）
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
| `<api-id>` | 是 | 接口 ID（Markdown 中反引号内的标识符） |
| `--env` | 否 | 目标环境名（不指定则使用配置中的第一个环境） |
| `--addr` | 直连地址 | 直连地址，覆盖环境配置中的 `host` |
| `--header` / `-H` | 否 | 自定义请求头 / gRPC metadata，格式 `key=value`，覆盖配置中的固定值（可多次使用） |
| `--param` | 否 | 覆盖参数，格式 `key=value`（可多次使用） |
| `--plaintext` | 否 | 完整请求体 JSON（字符串），**整体覆盖**接口定义中的请求体 |
| `--output` | 否 | 输出格式：`pretty` / `json` / `raw`（覆盖全局配置） |
| `--verbose` | 否 | 显示完整请求/响应报文，以及脚本返回字段、headers/metadata、params、body 的前后变化和最终请求 |

**示例：**

```bash
lion send createOrder --env dev
lion send createOrder --addr "127.0.0.1:8080" --param "orderId=12345"
lion send createOrder --env dev -H "x-tenant-id=T001"
lion send getOrder --env dev --output json
lion send getUser --env staging --param "userId=100" --param "fields=name,email"
```

#### 使用 `--plaintext` 指定完整请求体

`--plaintext` 接收一段**完整的请求体 JSON 字符串**，直接作为请求体发送，忽略 Markdown 接口定义（JSON 代码块）中的内容，也忽略 `--param` 的覆盖。适用于临时构造一个与接口定义完全不同的报文，或粘贴一段现成的 JSON 报文调试：

```bash
# HTTP：直接传入一段完整 JSON 报文
lion send createOrder --env dev --plaintext '{"orderId":"12345","warehouseId":"W001"}'

# HTTP：多行 JSON 便于阅读（使用单引号包裹，内部字段用双引号）
lion send createOrder --env dev --plaintext '{
  "orderId": "12345",
  "warehouseId": "W001",
  "items": [
    {"skuId": "SKU-1", "quantity": 2}
  ]
}'

# gRPC：同样适用于 gRPC 接口的请求体
lion send List --addr "127.0.0.1:9090" --plaintext '{"index":1,"size":10}'

# 配合 --verbose 查看实际发送的请求体
lion send createOrder --env dev --verbose --plaintext '{"orderId":"12345"}'
```

**注意事项：**

- `--plaintext` 的值必须是**合法 JSON**（对象或数组），否则报错 `解析 --plaintext 参数失败: ...` 且不会发送请求
- 生效顺序为「Markdown 请求体 → `--param` 覆盖 → `--plaintext` 整体覆盖」，因此 `--plaintext` 优先级最高，同时使用 `--param` 不会生效
- 在 Shell 中建议用**单引号**包裹 JSON，避免 `"` 被 Shell 解析；Windows CMD 中请使用双引号并转义内部引号

### `lion config`

管理 Collection 配置，包含三个子命令：

#### `lion config show <name>`

显示 Collection 的完整配置信息，包括分组列表、全局配置、环境配置、默认值规则、脚本钩子和脚本执行器配置。

```bash
lion config show petstore
```

输出示例：
```
Collection: petstore
来源类型: swagger
接口总数: 15

分组列表:
  📁 pet                            (5 个接口)
     /Users/xxx/.lion/api/petstore/pet.md
  📁 store                          (3 个接口)
     /Users/xxx/.lion/api/petstore/store.md

全局配置:
  超时时间: 30s
  输出格式: pretty

环境配置:
  🌍 dev:
     寻址方式: direct
     直连地址: 127.0.0.1:8080
```

#### `lion config init <name>`

手动初始化一个空的 Collection 目录和模板 Markdown 文件。适用于不通过 `generate` 而手动创建接口定义的场景。

```bash
lion config init my-service
```

这会在 `~/.lion/api/my-service/` 下创建 `default.md` 模板文件，包含基本的 Markdown 结构。

#### `lion config check <name>`

检查 Collection 配置是否正确，用于排查问题：

```bash
lion config check petstore
```

检查项目包括：
- Markdown 文件是否存在且可解析
- 接口 ID 是否唯一（不允许重复）
- 接口引用的脚本文件是否存在
- 元数据是否完整（`source`、`collection`）
- 全局环境配置是否有效
- Consul 服务名与 Consul 环境配置是否匹配

输出示例：
```
Collection "petstore" 配置检查结果:

❌ 错误 (1):
  - 接口 "createOrder" 引用的脚本不存在: ~/.lion/scripts/order.py

⚠️ 警告 (1):
  - Collection 使用了 Consul 服务名 "pet-service"，但全局配置中没有配置 Consul 环境

❌ 发现 1 个错误，请修复后重试
```

---

## License

MIT
