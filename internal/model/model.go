package model

// Collection 接口集合，对应 api/${collection}/ 目录
type Collection struct {
	Name      string   `json:"name"`       // 集合名称（目录名）
	Source    string   `json:"source"`     // 来源类型：swagger / grpc / proto
	SourceURL string   `json:"source_url"` // 来源地址
	Service   string   `json:"service"`    // Consul 服务名称（用于 consul 寻址模式）
	Groups    []*Group `json:"groups"`     // 分组列表，每组对应一个 .md 文件
}

// Group 分组，对应一个 Markdown 文件
type Group struct {
	Name string `json:"name"` // 分组名（文件名，如 "order"、"rbe.entity.petrel.ScheduleTask"）
	Path string `json:"path"` // Markdown 文件完整路径
	APIs []*API `json:"apis"` // 该分组下的接口列表
}

// API 单个接口
type API struct {
	ID          string `json:"id"`          // 唯一标识（反引号内内容）
	Method      string `json:"method"`      // HTTP Method 或 gRPC 方法名
	Path        string `json:"path"`        // 请求路径或全限定方法名
	BodyJSON    string `json:"body_json"`   // 完整请求结构 JSON（代码块内容，用户可编辑）
	Script      string `json:"script"`      // 接口级脚本路径（空则使用全局 hooks）
	Description string `json:"description"` // 接口描述
}

// GlobalConfig 全局配置，对应 global.yaml
type GlobalConfig struct {
	Global       GlobalSettings          `json:"global"       yaml:"global"`
	Environments map[string]*Environment `json:"environments" yaml:"environments"`
	Defaults     *DefaultRule            `json:"defaults"      yaml:"defaults"`          // 全局默认值规则
	Hooks        *Hooks                  `json:"hooks,omitempty" yaml:"hooks,omitempty"` // 全局脚本钩子
}

// GlobalSettings 全局设置
type GlobalSettings struct {
	Timeout       string            `json:"timeout"        yaml:"timeout"`                            // 默认请求超时
	Output        string            `json:"output"         yaml:"output"`                             // 输出格式：pretty / json / raw
	ScriptRunners map[string]string `json:"script_runners,omitempty" yaml:"script_runners,omitempty"` // 脚本执行器配置
}

// Environment 环境配置
type Environment struct {
	ConsulAddr  string `json:"consul_addr,omitempty"  yaml:"consul_addr,omitempty"`  // Consul 地址
	ConsulToken string `json:"consul_token,omitempty" yaml:"consul_token,omitempty"` // Consul Token
	Resolve     string `json:"resolve"                yaml:"resolve"`                // 寻址方式：consul / direct
	Host        string `json:"host,omitempty"         yaml:"host,omitempty"`         // 直连地址
}

// DefaultRule 默认值规则
type DefaultRule struct {
	Types  map[string]string `json:"types,omitempty"  yaml:"types,omitempty"`  // 按类型名匹配
	Fields map[string]string `json:"fields,omitempty" yaml:"fields,omitempty"` // 按字段名匹配
}

// Hooks 全局脚本钩子
type Hooks struct {
	PreRequest  string `json:"pre_request,omitempty"  yaml:"pre_request,omitempty"`  // 请求前脚本路径
	PostRequest string `json:"post_request,omitempty" yaml:"post_request,omitempty"` // 请求后脚本路径
}

// ScriptContext 脚本上下文，传递给脚本的完整信息
type ScriptContext struct {
	APIID      string            `json:"api_id"`
	Method     string            `json:"method"`  // HTTP Method
	URL        string            `json:"url"`     // 请求 URL
	Headers    map[string]string `json:"headers"` // 请求头
	Params     map[string]string `json:"params"`  // Query / Path 参数
	Body       interface{}       `json:"body"`    // 请求体
	StatusCode int               `json:"status_code,omitempty"`
	RespBody   interface{}       `json:"resp_body,omitempty"`
	ElapsedMs  int64             `json:"elapsed_ms,omitempty"`
}

// ScriptResult 脚本执行结果
type ScriptResult struct {
	Headers    map[string]string `json:"headers,omitempty"`
	Params     map[string]string `json:"params,omitempty"`
	Body       interface{}       `json:"body,omitempty"`
	Assertions []Assertion       `json:"assertions,omitempty"`
	Extracted  map[string]string `json:"extracted,omitempty"`
	Output     string            `json:"output,omitempty"`
}

// Assertion 断言结果
type Assertion struct {
	Name    string `json:"name"`
	Pass    bool   `json:"pass"`
	Message string `json:"message"`
}
