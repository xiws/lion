package sender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"lion/internal/config"
	"lion/internal/consul"
	"lion/internal/model"
	"lion/internal/script"
)

// Sender 请求发送器
type Sender struct {
	cfg     *model.GlobalConfig
	env     *model.Environment
	envName string
	engine  *script.Engine
	addr    string // 命令行 --addr 覆盖
	service string // Consul 服务名称
}

// NewSender 创建发送器
func NewSender(cfg *model.GlobalConfig, envName, addr, service string) (*Sender, error) {
	name, env, err := config.GetEnvironment(cfg, envName)
	if err != nil {
		return nil, err
	}

	// 获取脚本执行器配置
	var runners map[string]string
	if cfg.Global.ScriptRunners != nil {
		runners = cfg.Global.ScriptRunners
	}

	return &Sender{
		cfg:     cfg,
		env:     env,
		envName: name,
		engine:  script.NewEngineWithRunners(runners),
		addr:    addr,
		service: service,
	}, nil
}

// SendHTTP 发送 HTTP 请求
func (s *Sender) SendHTTP(api *model.API, params map[string]string, overrides map[string]string) error {
	// 构造请求 URL
	baseURL, err := s.resolveURL(api)
	if err != nil {
		return err
	}

	// 构造请求头
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// 解析请求体
	var body interface{}
	if api.BodyJSON != "" {
		if err := json.Unmarshal([]byte(api.BodyJSON), &body); err != nil {
			return fmt.Errorf("解析请求体失败: %w", err)
		}
	}

	// 应用参数覆盖
	if len(overrides) > 0 && body != nil {
		if bodyMap, ok := body.(map[string]interface{}); ok {
			for k, v := range overrides {
				bodyMap[k] = v
			}
		}
	}

	// 构造脚本上下文
	ctx := &model.ScriptContext{
		APIID:   api.ID,
		Method:  api.Method,
		URL:     baseURL,
		Headers: headers,
		Params:  params,
		Body:    body,
	}

	// 执行前置脚本
	preScript := script.ResolveScript(api, s.cfg.Hooks, true)
	if preScript != "" {
		result, err := s.engine.ExecutePreScript(preScript, "pre_request", ctx)
		if err != nil {
			fmt.Printf("⚠️  前置脚本警告: %v\n", err)
		} else {
			applyPreResult(ctx, result)
		}
	}

	// 构造 HTTP 请求
	bodyBytes, err := json.Marshal(ctx.Body)
	if err != nil {
		return fmt.Errorf("序列化请求体失败: %w", err)
	}

	req, err := http.NewRequest(ctx.Method, ctx.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	for k, v := range ctx.Headers {
		req.Header.Set(k, v)
	}

	// 设置 Query 参数
	if len(ctx.Params) > 0 {
		q := req.URL.Query()
		for k, v := range ctx.Params {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	// 发送请求
	timeout := parseTimeout(s.cfg.Global.Timeout)
	client := &http.Client{Timeout: timeout}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()
	elapsed := time.Since(start).Milliseconds()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	// 构造后置脚本上下文
	postCtx := &model.ScriptContext{
		APIID:      api.ID,
		Method:     api.Method,
		URL:        ctx.URL,
		Headers:    ctx.Headers,
		Params:     ctx.Params,
		Body:       ctx.Body,
		StatusCode: resp.StatusCode,
		ElapsedMs:  elapsed,
	}

	var respJSON interface{}
	if err := json.Unmarshal(respBody, &respJSON); err == nil {
		postCtx.RespBody = respJSON
	} else {
		postCtx.RespBody = string(respBody)
	}

	// 执行后置脚本
	postScript := script.ResolveScript(api, s.cfg.Hooks, false)
	if postScript != "" {
		result, err := s.engine.ExecutePostScript(postScript, "post_request", postCtx)
		if err != nil {
			fmt.Printf("⚠️  后置脚本警告: %v\n", err)
		} else {
			printPostResult(result)
		}
	}

	// 输出响应
	printHTTPResponse(resp.StatusCode, resp.Header, respBody, elapsed)

	return nil
}

// resolveURL 解析请求 URL
func (s *Sender) resolveURL(api *model.API) (string, error) {
	host, err := s.resolveHost()
	if err != nil {
		return "", err
	}

	// 如果 host 已包含协议前缀，直接使用
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return host + api.Path, nil
	}

	return "http://" + host + api.Path, nil
}

// resolveHost 解析目标主机地址
func (s *Sender) resolveHost() (string, error) {
	// 优先级: --addr > consul > env.host
	if s.addr != "" {
		return s.addr, nil
	}

	if s.env.Resolve == "consul" && s.env.ConsulAddr != "" {
		if s.service == "" {
			return "", fmt.Errorf("consul 寻址需要配置服务名称（在 Markdown 元数据中添加 > service: <服务名>）")
		}
		addr, err := ResolveServiceAddr(s.env, s.service)
		if err != nil {
			return "", fmt.Errorf("Consul 寻址失败: %w", err)
		}
		return addr, nil
	}

	// direct 模式回退到 host 字段
	if s.env.Host != "" {
		return s.env.Host, nil
	}

	return "", fmt.Errorf("无法确定目标地址，请指定 --addr 或配置环境 host")
}

// applyPreResult 将前置脚本结果应用到上下文
func applyPreResult(ctx *model.ScriptContext, result *model.ScriptResult) {
	if result.Headers != nil {
		ctx.Headers = result.Headers
	}
	if result.Params != nil {
		ctx.Params = result.Params
	}
	if result.Body != nil {
		ctx.Body = result.Body
	}
}

// printPostResult 输出后置脚本结果
func printPostResult(result *model.ScriptResult) {
	if result.Output != "" {
		fmt.Printf("📝 %s\n", result.Output)
	}

	for _, a := range result.Assertions {
		if a.Pass {
			fmt.Printf("✅ %s: %s\n", a.Name, a.Message)
		} else {
			fmt.Printf("❌ %s: %s\n", a.Name, a.Message)
		}
	}
}

// printHTTPResponse 输出 HTTP 响应
func printHTTPResponse(statusCode int, headers http.Header, body []byte, elapsed int64) {
	fmt.Printf("\n── 响应 ──────────────────────────\n")
	fmt.Printf("状态: %d (%dms)\n", statusCode, elapsed)
	fmt.Printf("──────────────────────────────────\n")

	// 尝试格式化 JSON 输出
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, body, "", "  "); err == nil {
		fmt.Println(prettyJSON.String())
	} else {
		fmt.Println(string(body))
	}
}

// parseTimeout 解析超时时间字符串
func parseTimeout(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

// ResolveServiceAddr 通过 Consul 解析服务地址
func ResolveServiceAddr(env *model.Environment, serviceName string) (string, error) {
	if env.ConsulAddr == "" {
		return "", fmt.Errorf("Consul 地址未配置")
	}

	client, err := consul.NewClient(env.ConsulAddr, env.ConsulToken)
	if err != nil {
		return "", err
	}

	instance, err := client.PickInstance(serviceName)
	if err != nil {
		return "", err
	}

	return instance.Addr(), nil
}
