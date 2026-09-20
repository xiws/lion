package collection

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lion/internal/config"
	"lion/internal/markdown"
	"lion/internal/model"
)

// ShowResult 包含 Show 命令的输出信息
type ShowResult struct {
	Name      string
	Source    string
	SourceURL string
	Service   string
	Groups    []GroupInfo
	TotalAPIs int

	// 全局配置信息
	GlobalTimeout string
	GlobalOutput  string
	Environments  map[string]*EnvInfo
	Defaults      *DefaultInfo
	Hooks         *HooksInfo
	ScriptRunners map[string]string
}

// EnvInfo 环境信息
type EnvInfo struct {
	Resolve     string
	Host        string
	ConsulAddr  string
	ConsulToken string
}

// DefaultInfo 默认值规则
type DefaultInfo struct {
	Types  map[string]string
	Fields map[string]string
}

// HooksInfo 脚本钩子
type HooksInfo struct {
	PreRequest  []string
	PostRequest []string
}

// GroupInfo 分组摘要
type GroupInfo struct {
	Name     string
	FilePath string
	APICount int
}

// Show 展示 Collection 配置信息
func Show(name string) (*ShowResult, error) {
	coll, err := Load(name)
	if err != nil {
		return nil, err
	}

	result := &ShowResult{
		Name:      coll.Name,
		Source:    coll.Source,
		SourceURL: coll.SourceURL,
		Service:   coll.Service,
	}

	for _, g := range coll.Groups {
		result.Groups = append(result.Groups, GroupInfo{
			Name:     g.Name,
			FilePath: g.Path,
			APICount: len(g.APIs),
		})
		result.TotalAPIs += len(g.APIs)
	}

	// 加载全局配置
	cfg, err := config.LoadGlobalConfig()
	if err == nil {
		result.GlobalTimeout = cfg.Global.Timeout
		result.GlobalOutput = cfg.Global.Output

		if len(cfg.Environments) > 0 {
			result.Environments = make(map[string]*EnvInfo)
			for name, env := range cfg.Environments {
				result.Environments[name] = &EnvInfo{
					Resolve:     env.Resolve,
					Host:        env.Host,
					ConsulAddr:  env.ConsulAddr,
					ConsulToken: env.ConsulToken,
				}
			}
		}

		if cfg.Defaults != nil {
			result.Defaults = &DefaultInfo{
				Types:  cfg.Defaults.Types,
				Fields: cfg.Defaults.Fields,
			}
		}

		if cfg.Hooks != nil {
			result.Hooks = &HooksInfo{
				PreRequest:  []string(cfg.Hooks.PreRequest),
				PostRequest: []string(cfg.Hooks.PostRequest),
			}
		}

		if len(cfg.Global.ScriptRunners) > 0 {
			result.ScriptRunners = cfg.Global.ScriptRunners
		}
	}

	return result, nil
}

// Init 初始化一份 Collection 配置（创建目录和模板文件）
func Init(name string) error {
	dir, err := config.LionDir()
	if err != nil {
		return err
	}

	collectionDir := filepath.Join(dir, "api", name)

	// 检查是否已存在
	if _, err := os.Stat(collectionDir); err == nil {
		return fmt.Errorf("Collection %q 已存在，如需重新生成请使用 generate 命令", name)
	}

	if err := os.MkdirAll(collectionDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 生成模板 Markdown 文件
	template := generateTemplate(name)
	templatePath := filepath.Join(collectionDir, "default.md")
	if err := os.WriteFile(templatePath, []byte(template), 0644); err != nil {
		return fmt.Errorf("创建模板文件失败: %w", err)
	}

	return nil
}

// CheckResult 包含 Check 命令的检查结果
type CheckResult struct {
	Errors   []string
	Warnings []string
}

// IsValid 检查是否全部通过
func (r *CheckResult) IsValid() bool {
	return len(r.Errors) == 0
}

// Check 检查 Collection 配置是否正确
func Check(name string) (*CheckResult, error) {
	result := &CheckResult{}

	dir, err := config.LionDir()
	if err != nil {
		return nil, err
	}

	collectionDir := filepath.Join(dir, "api", name)

	// 1. 检查目录是否存在
	if _, err := os.Stat(collectionDir); os.IsNotExist(err) {
		result.Errors = append(result.Errors, fmt.Sprintf("Collection 目录不存在: %s", collectionDir))
		return result, nil
	}

	// 2. 检查是否有 Markdown 文件
	entries, err := os.ReadDir(collectionDir)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	var mdFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			mdFiles = append(mdFiles, entry.Name())
		}
	}

	if len(mdFiles) == 0 {
		result.Errors = append(result.Errors, "Collection 目录下没有 Markdown 文件")
		return result, nil
	}

	// 3. 解析每个 Markdown 文件并检查
	apiIDs := make(map[string]string) // apiID -> 所在文件
	for _, mdFile := range mdFiles {
		filePath := filepath.Join(collectionDir, mdFile)

		// 解析分组文件
		group, err := markdown.ParseGroupFile(filePath)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("文件 %s 解析失败: %v", mdFile, err))
			continue
		}

		if len(group.APIs) == 0 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("文件 %s 中没有定义接口", mdFile))
		}

		// 检查 API ID 唯一性
		for _, api := range group.APIs {
			if api.ID == "" {
				result.Errors = append(result.Errors, fmt.Sprintf("文件 %s 中存在空的接口 ID", mdFile))
				continue
			}
			if prev, exists := apiIDs[api.ID]; exists {
				result.Errors = append(result.Errors, fmt.Sprintf("接口 ID %q 重复定义: %s 和 %s", api.ID, prev, mdFile))
			}
			apiIDs[api.ID] = mdFile
		}

		// 检查元数据
		meta, err := markdown.ParseMetadata(filePath)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("文件 %s 元数据解析失败: %v", mdFile, err))
			continue
		}

		if meta["source"] == "" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("文件 %s 缺少 source 元数据", mdFile))
		}

		// 检查脚本引用是否存在
		for _, api := range group.APIs {
			for _, scriptPath := range api.Scripts {
				expanded := expandPath(scriptPath)
				if _, err := os.Stat(expanded); os.IsNotExist(err) {
					result.Errors = append(result.Errors, fmt.Sprintf("接口 %q 引用的脚本不存在: %s", api.ID, scriptPath))
				}
			}
		}
	}

	// 4. 检查全局配置中的环境
	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("全局配置加载失败: %v", err))
	} else if len(cfg.Environments) == 0 {
		result.Warnings = append(result.Warnings, "全局配置中未定义任何环境")
	}

	// 5. 如果 Collection 指定了 service，检查 Consul 配置
	coll, err := Load(name)
	if err == nil && strings.TrimSpace(coll.Service) != "" {
		hasConsul := false
		for _, env := range cfg.Environments {
			if env.Resolve == "consul" && env.ConsulAddr != "" {
				hasConsul = true
				break
			}
		}
		if !hasConsul {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Collection 使用了 Consul 服务名 %q，但全局配置中没有配置 Consul 环境", coll.Service))
		}
	}

	return result, nil
}

// generateTemplate 生成 Collection 模板 Markdown
func generateTemplate(name string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", name))
	sb.WriteString("> source: swagger\n")
	sb.WriteString(fmt.Sprintf("> collection: %s\n", name))
	sb.WriteString("> service: \n")
	sb.WriteString("\n")
	sb.WriteString("### GET /example `example`\n\n")
	sb.WriteString("```json\n")
	sb.WriteString("{}\n")
	sb.WriteString("```\n")
	return sb.String()
}

// expandPath 展开 ~ 为家目录
func expandPath(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}

// FormatShowResult 格式化 Show 输出
func FormatShowResult(r *ShowResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Collection: %s\n", r.Name))
	if r.Source != "" {
		sb.WriteString(fmt.Sprintf("来源类型: %s\n", r.Source))
	}
	if r.SourceURL != "" {
		sb.WriteString(fmt.Sprintf("来源地址: %s\n", r.SourceURL))
	}
	if r.Service != "" {
		sb.WriteString(fmt.Sprintf("Consul 服务: %s\n", r.Service))
	}
	sb.WriteString(fmt.Sprintf("接口总数: %d\n", r.TotalAPIs))

	if len(r.Groups) > 0 {
		sb.WriteString("\n分组列表:\n")
		for _, g := range r.Groups {
			sb.WriteString(fmt.Sprintf("  📁 %-30s (%d 个接口)\n", g.Name, g.APICount))
			sb.WriteString(fmt.Sprintf("     %s\n", g.FilePath))
		}
	} else {
		sb.WriteString("\n暂无分组\n")
	}

	// 全局配置
	sb.WriteString("\n全局配置:\n")
	if r.GlobalTimeout != "" {
		sb.WriteString(fmt.Sprintf("  超时时间: %s\n", r.GlobalTimeout))
	}
	if r.GlobalOutput != "" {
		sb.WriteString(fmt.Sprintf("  输出格式: %s\n", r.GlobalOutput))
	}

	// 环境信息
	if len(r.Environments) > 0 {
		sb.WriteString("\n环境配置:\n")
		for name, env := range r.Environments {
			sb.WriteString(fmt.Sprintf("  🌍 %s:\n", name))
			sb.WriteString(fmt.Sprintf("     寻址方式: %s\n", env.Resolve))
			if env.Host != "" {
				sb.WriteString(fmt.Sprintf("     直连地址: %s\n", env.Host))
			}
			if env.ConsulAddr != "" {
				sb.WriteString(fmt.Sprintf("     Consul:  %s\n", env.ConsulAddr))
			}
			if env.ConsulToken != "" {
				sb.WriteString(fmt.Sprintf("     Token:   %s\n", env.ConsulToken))
			}
		}
	} else {
		sb.WriteString("\n环境配置: 未配置\n")
	}

	// 默认值规则
	if r.Defaults != nil && (len(r.Defaults.Types) > 0 || len(r.Defaults.Fields) > 0) {
		sb.WriteString("\n默认值规则:\n")
		if len(r.Defaults.Types) > 0 {
			sb.WriteString("  按类型:\n")
			for typeName, value := range r.Defaults.Types {
				sb.WriteString(fmt.Sprintf("    %s = %s\n", typeName, value))
			}
		}
		if len(r.Defaults.Fields) > 0 {
			sb.WriteString("  按字段:\n")
			for fieldName, value := range r.Defaults.Fields {
				sb.WriteString(fmt.Sprintf("    %s = %s\n", fieldName, value))
			}
		}
	}

	// 脚本钩子
	if r.Hooks != nil && (len(r.Hooks.PreRequest) > 0 || len(r.Hooks.PostRequest) > 0) {
		sb.WriteString("\n脚本钩子:\n")
		if len(r.Hooks.PreRequest) > 0 {
			sb.WriteString("  请求前:\n")
			for _, s := range r.Hooks.PreRequest {
				sb.WriteString(fmt.Sprintf("    - %s\n", s))
			}
		}
		if len(r.Hooks.PostRequest) > 0 {
			sb.WriteString("  请求后:\n")
			for _, s := range r.Hooks.PostRequest {
				sb.WriteString(fmt.Sprintf("    - %s\n", s))
			}
		}
	}

	// 脚本执行器配置
	if len(r.ScriptRunners) > 0 {
		sb.WriteString("\n脚本执行器:\n")
		for name, path := range r.ScriptRunners {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", name, path))
		}
	}

	return sb.String()
}

// FormatCheckResult 格式化 Check 输出
func FormatCheckResult(name string, r *CheckResult) string {
	var sb strings.Builder

	if r.IsValid() && len(r.Warnings) == 0 {
		sb.WriteString(fmt.Sprintf("✅ Collection %q 配置检查通过\n", name))
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("Collection %q 配置检查结果:\n", name))

	if len(r.Errors) > 0 {
		sb.WriteString(fmt.Sprintf("\n❌ 错误 (%d):\n", len(r.Errors)))
		for _, e := range r.Errors {
			sb.WriteString(fmt.Sprintf("  - %s\n", e))
		}
	}

	if len(r.Warnings) > 0 {
		sb.WriteString(fmt.Sprintf("\n⚠️  警告 (%d):\n", len(r.Warnings)))
		for _, w := range r.Warnings {
			sb.WriteString(fmt.Sprintf("  - %s\n", w))
		}
	}

	if r.IsValid() {
		sb.WriteString("\n✅ 无错误，配置基本可用\n")
	} else {
		sb.WriteString(fmt.Sprintf("\n❌ 发现 %d 个错误，请修复后重试\n", len(r.Errors)))
	}

	return sb.String()
}

// Ensure model is imported (used indirectly via Load)
var _ = (*model.Collection)(nil)
