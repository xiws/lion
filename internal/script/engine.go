package script

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

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
	return e.execute(scriptPath, action, ctx)
}

// ExecutePostScript 执行后置脚本
func (e *Engine) ExecutePostScript(scriptPath, action string, ctx *model.ScriptContext) (*model.ScriptResult, error) {
	return e.execute(scriptPath, action, ctx)
}

// CheckScriptExists 检查脚本是否存在
func CheckScriptExists(scriptPath string) error {
	expandedPath := expandPath(scriptPath)
	if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
		return fmt.Errorf("脚本文件不存在: %s", scriptPath)
	}
	return nil
}

// execute 执行脚本
func (e *Engine) execute(scriptPath, action string, ctx *model.ScriptContext) (*model.ScriptResult, error) {
	// 展开路径中的 ~
	expandedPath := expandPath(scriptPath)

	// 检查脚本是否存在
	if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("脚本文件不存在: %s", scriptPath)
	}

	// 序列化上下文为 JSON
	input, err := json.Marshal(ctx)
	if err != nil {
		return nil, fmt.Errorf("序列化脚本上下文失败: %w", err)
	}

	// 构造命令
	cmd := e.buildCommand(expandedPath, action)
	cmd.Stdin = bytes.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 执行脚本
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("执行脚本失败: %w\nstderr: %s", err, stderr.String())
	}

	// 解析输出
	var result model.ScriptResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("解析脚本输出失败: %w\noutput: %s", err, stdout.String())
	}

	return &result, nil
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

// ResolveScript 解析接口实际使用的脚本路径
// 优先级：接口级 > 全局 hooks
func ResolveScript(api *model.API, hooks *model.Hooks, isPre bool) string {
	// 接口级脚本优先
	if api.Script != "" {
		return api.Script
	}

	// 回退到全局 hooks
	if hooks != nil {
		if isPre {
			return hooks.PreRequest
		}
		return hooks.PostRequest
	}

	return ""
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
