package script

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"lion/internal/model"
)

// Engine 脚本执行引擎
type Engine struct {
	runners map[string]string // 脚本执行器配置 (python -> /usr/bin/python3, node -> /usr/bin/node)
}

// NewEngine 创建脚本引擎
func NewEngine() *Engine {
	return &Engine{}
}

// NewEngineWithRunners 创建带执行器配置的脚本引擎
func NewEngineWithRunners(runners map[string]string) *Engine {
	return &Engine{runners: runners}
}

// ExecutePreScript 执行前置脚本
func (e *Engine) ExecutePreScript(scriptPath, action string, ctx *model.ScriptContext) (*model.ScriptResult, error) {
	result, _, err := e.execute(scriptPath, action, ctx)
	return result, err
}

// ExecutePostScript 执行后置脚本
func (e *Engine) ExecutePostScript(scriptPath, action string, ctx *model.ScriptContext) (*model.ScriptResult, error) {
	result, _, err := e.execute(scriptPath, action, ctx)
	return result, err
}

// ContextSnapshot 脚本执行前后的请求上下文快照
type ContextSnapshot struct {
	Headers map[string]string
	Params  map[string]string
	Body    interface{}
}

// PreScriptStep 前置脚本链式执行中的一个步骤
type PreScriptStep struct {
	Script    string           // 脚本文件路径
	Error     error            // 脚本执行错误（nil 表示成功）
	Stderr    string           // 脚本 stderr（即使退出码为 0，也可能有诊断日志）
	Before    *ContextSnapshot // 执行前的上下文
	After     *ContextSnapshot // 应用脚本返回值之后的上下文
	Result    *model.ScriptResult
	ElapsedMs int64 // 本脚本耗时（毫秒）
}

// PostScriptStep 后置脚本链式执行中的一个步骤
type PostScriptStep struct {
	Script    string // 脚本文件路径
	Error     error  // 脚本执行错误（nil 表示成功）
	Stderr    string // 脚本 stderr（即使退出码为 0，也可能有诊断日志）
	Result    *model.ScriptResult
	ElapsedMs int64 // 本脚本耗时（毫秒）
}

// ExecutePreChain 链式执行多个前置脚本，并记录每一步执行前后的上下文。
// 脚本出错时中止后续脚本，已执行步骤仍会返回，便于 --verbose 定位问题。
func (e *Engine) ExecutePreChain(scripts []string, action string, ctx *model.ScriptContext) ([]PreScriptStep, int64) {
	steps := make([]PreScriptStep, 0, len(scripts))
	start := time.Now()
	for _, sp := range scripts {
		step := PreScriptStep{
			Script: sp,
			Before: snapshotContext(ctx),
		}

		stepStart := time.Now()
		result, stderrText, err := e.execute(sp, action, ctx)
		step.ElapsedMs = time.Since(stepStart).Milliseconds()
		step.Stderr = stderrText

		if err != nil {
			step.Error = fmt.Errorf("脚本 %s: %w", sp, err)
			step.After = snapshotContext(ctx)
			steps = append(steps, step)
			break
		}

		applyPreResult(ctx, result)
		step.Result = result
		step.After = snapshotContext(ctx)
		steps = append(steps, step)
	}
	return steps, time.Since(start).Milliseconds()
}

// ExecutePostChain 链式执行多个后置脚本，并记录每一步的结果。
// 脚本出错时中止后续脚本，已执行步骤仍会返回，便于 --verbose 定位问题。
func (e *Engine) ExecutePostChain(scripts []string, action string, ctx *model.ScriptContext) ([]PostScriptStep, []*model.ScriptResult) {
	steps := make([]PostScriptStep, 0, len(scripts))
	results := make([]*model.ScriptResult, 0, len(scripts))
	for _, sp := range scripts {
		step := PostScriptStep{Script: sp}

		stepStart := time.Now()
		result, stderrText, err := e.execute(sp, action, ctx)
		step.ElapsedMs = time.Since(stepStart).Milliseconds()
		step.Stderr = stderrText

		if err != nil {
			step.Error = fmt.Errorf("脚本 %s: %w", sp, err)
			steps = append(steps, step)
			break
		}

		step.Result = result
		steps = append(steps, step)
		results = append(results, result)

		// 将后置结果中的 extracted 数据反馈到下一个脚本的上下文
		if result.Extracted != nil {
			if ctx.Body == nil {
				ctx.Body = make(map[string]interface{})
			}
		}
	}
	return steps, results
}

// snapshotContext 复制当前上下文，供前后对比使用。
// Body 在链式执行中只会被整体替换、不会被原地修改，因此可共享引用。
func snapshotContext(ctx *model.ScriptContext) *ContextSnapshot {
	return &ContextSnapshot{
		Headers: copyMap(ctx.Headers),
		Params:  copyMap(ctx.Params),
		Body:    ctx.Body,
	}
}

// copyMap 复制 map[string]string
func copyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	nm := make(map[string]string, len(m))
	for k, v := range m {
		nm[k] = v
	}
	return nm
}

// CheckScriptExists 检查脚本是否存在
func CheckScriptExists(scriptPath string) error {
	expandedPath := expandPath(scriptPath)
	if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
		return fmt.Errorf("脚本文件不存在: %s", scriptPath)
	}
	return nil
}

// execute 执行脚本，返回脚本结果、stderr 内容和执行错误
func (e *Engine) execute(scriptPath, action string, ctx *model.ScriptContext) (*model.ScriptResult, string, error) {
	// 展开路径中的 ~
	expandedPath := expandPath(scriptPath)

	// 检查脚本是否存在
	if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
		return nil, "", fmt.Errorf("脚本文件不存在: %s", scriptPath)
	}

	// 序列化上下文为 JSON
	input, err := json.Marshal(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("序列化脚本上下文失败: %w", err)
	}

	// 构造命令
	cmd := e.buildCommand(expandedPath, action)
	cmd.Stdin = bytes.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 执行脚本
	if err := cmd.Run(); err != nil {
		return nil, stderr.String(), fmt.Errorf("执行脚本失败: %w\nstderr: %s", err, stderr.String())
	}

	// 解析输出
	var result model.ScriptResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, stderr.String(), fmt.Errorf("解析脚本输出失败: %w\noutput: %s", err, stdout.String())
	}

	return &result, stderr.String(), nil
}

// buildCommand 根据脚本文件扩展名构造执行命令
func (e *Engine) buildCommand(scriptPath, action string) *exec.Cmd {
	ext := strings.ToLower(scriptPath)
	if idx := strings.LastIndex(ext, "."); idx >= 0 {
		ext = ext[idx:]
	}

	switch ext {
	case ".py":
		python := e.getRunner("python", "python3", "python")
		return exec.Command(python, scriptPath, "--action", action)
	case ".js":
		node := e.getRunner("node", "node")
		return exec.Command(node, scriptPath, "--action", action)
	case ".sh":
		if runtime.GOOS == "windows" {
			bash := e.getRunner("bash", "bash")
			return exec.Command(bash, scriptPath, "--action", action)
		}
		return exec.Command("bash", scriptPath, "--action", action)
	default:
		// 尝试直接执行
		return exec.Command(scriptPath, "--action", action)
	}
}

// getRunner 获取脚本执行器路径
// 优先使用配置中的路径，否则尝试 candidates 中的命令
func (e *Engine) getRunner(name string, candidates ...string) string {
	// 1. 检查配置
	if e.runners != nil {
		if path, ok := e.runners[name]; ok && path != "" {
			return path
		}
	}

	// 2. 尝试环境变量 LION_PYTHON / LION_NODE 等
	envKey := "LION_" + strings.ToUpper(name)
	if path := os.Getenv(envKey); path != "" {
		return path
	}

	// 3. 按顺序尝试候选命令，返回第一个存在的
	for _, cmd := range candidates {
		if path, err := exec.LookPath(cmd); err == nil {
			return path
		}
	}

	// 4. 回退到第一个候选
	if len(candidates) > 0 {
		return candidates[0]
	}
	return name
}

// ResolveScripts 解析接口实际使用的脚本路径列表
// 优先级：接口级 > 分组级（collection/group） > 全局 hooks
func ResolveScripts(api *model.API, groupHooks *model.Hooks, globalHooks *model.Hooks, isPre bool) []string {
	// 1. 接口级脚本最高优先
	if len(api.Scripts) > 0 {
		return api.Scripts
	}

	// 2. 分组级（Collection）钩子次之
	if groupHooks != nil {
		if isPre {
			return groupHooks.PreRequest
		}
		return groupHooks.PostRequest
	}

	// 3. 回退到全局 hooks
	if globalHooks != nil {
		if isPre {
			return globalHooks.PreRequest
		}
		return globalHooks.PostRequest
	}

	return nil
}

// applyPreResult 将前置脚本结果应用到上下文（供链式执行使用）
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

// expandPath 展开路径中的 ~ 为用户主目录
func expandPath(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return strings.Replace(path, "~", home, 1)
}
