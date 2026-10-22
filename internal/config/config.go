package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lion/internal/model"

	"gopkg.in/yaml.v3"
)

// LionDir 返回 ~/.lion 目录路径
func LionDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户主目录失败: %w", err)
	}
	return filepath.Join(home, ".lion"), nil
}

// GlobalConfigPath 返回 global.yaml 的完整路径
func GlobalConfigPath() (string, error) {
	dir, err := LionDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "global.yaml"), nil
}

// LoadGlobalConfig 加载全局配置文件
func LoadGlobalConfig() (*model.GlobalConfig, error) {
	path, err := GlobalConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultGlobalConfig(), nil
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg model.GlobalConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 设置默认值
	if cfg.Global.Timeout == "" {
		cfg.Global.Timeout = "30s"
	}
	if cfg.Global.Output == "" {
		cfg.Global.Output = "pretty"
	}
	if cfg.Environments == nil {
		cfg.Environments = make(map[string]*model.Environment)
	}

	return &cfg, nil
}

// CollectionConfigPath 返回 Collection 级配置文件路径：~/.lion/<collection>.yaml
func CollectionConfigPath(collectionName string) (string, error) {
	dir, err := LionDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, collectionName+".yaml"), nil
}

// ApplyCollectionConfig 加载 <collection>.yaml 并合并到全局配置。
// Collection 级配置覆盖 global.yaml 中的同名配置，未定义的字段继续继承全局配置。
func ApplyCollectionConfig(base *model.GlobalConfig, collectionName string) (*model.GlobalConfig, error) {
	if base == nil {
		base = defaultGlobalConfig()
	}
	if collectionName == "" || collectionName == "global" {
		return base, nil
	}

	path, err := CollectionConfigPath(collectionName)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return base, nil
		}
		return nil, fmt.Errorf("读取 Collection 配置 %s 失败: %w", path, err)
	}

	var overlay model.GlobalConfig
	if err := yaml.Unmarshal(data, &overlay); err != nil {
		return nil, fmt.Errorf("解析 Collection 配置 %s 失败: %w", path, err)
	}

	return mergeGlobalConfig(base, &overlay), nil
}

// mergeGlobalConfig 将 overlay 合并到 base，返回新配置，不修改 base。
func mergeGlobalConfig(base, overlay *model.GlobalConfig) *model.GlobalConfig {
	out := *base

	if overlay.Global.Timeout != "" {
		out.Global.Timeout = overlay.Global.Timeout
	}
	if overlay.Global.Output != "" {
		out.Global.Output = overlay.Global.Output
	}
	out.Global.ScriptRunners = mergeStringMap(base.Global.ScriptRunners, overlay.Global.ScriptRunners)
	out.Global.Headers = mergeHeaderMap(base.Global.Headers, overlay.Global.Headers)

	// Environment 按环境名深合并：Collection 只写 headers 时仍保留全局的 host/consul 配置
	out.Environments = cloneEnvironments(base.Environments)
	for name, overEnv := range overlay.Environments {
		if overEnv == nil {
			continue
		}
		env := out.Environments[name]
		if env == nil {
			env = &model.Environment{}
			out.Environments[name] = env
		}
		if overEnv.Resolve != "" {
			env.Resolve = overEnv.Resolve
		}
		if overEnv.Host != "" {
			env.Host = overEnv.Host
		}
		if overEnv.ConsulAddr != "" {
			env.ConsulAddr = overEnv.ConsulAddr
		}
		if overEnv.ConsulToken != "" {
			env.ConsulToken = overEnv.ConsulToken
		}
		env.Headers = mergeHeaderMap(env.Headers, overEnv.Headers)
	}

	if overlay.Defaults != nil {
		defaults := &model.DefaultRule{}
		if base.Defaults != nil {
			defaults.Types = copyStringMap(base.Defaults.Types)
			defaults.Fields = copyStringMap(base.Defaults.Fields)
		}
		defaults.Types = mergeStringMap(defaults.Types, overlay.Defaults.Types)
		defaults.Fields = mergeStringMap(defaults.Fields, overlay.Defaults.Fields)
		out.Defaults = defaults
	}

	if overlay.Hooks != nil {
		hooks := &model.Hooks{}
		if base.Hooks != nil {
			hooks.PreRequest = append(model.StringSlice{}, base.Hooks.PreRequest...)
			hooks.PostRequest = append(model.StringSlice{}, base.Hooks.PostRequest...)
		}
		if overlay.Hooks.PreRequest != nil {
			hooks.PreRequest = overlay.Hooks.PreRequest
		}
		if overlay.Hooks.PostRequest != nil {
			hooks.PostRequest = overlay.Hooks.PostRequest
		}
		out.Hooks = hooks
	}

	return &out
}

func cloneEnvironments(src map[string]*model.Environment) map[string]*model.Environment {
	dst := make(map[string]*model.Environment, len(src))
	for name, env := range src {
		if env == nil {
			dst[name] = nil
			continue
		}
		copied := *env
		copied.Headers = copyStringMap(env.Headers)
		dst[name] = &copied
	}
	return dst
}

func copyStringMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func mergeStringMap(base, overlay map[string]string) map[string]string {
	if len(overlay) == 0 {
		return base
	}
	dst := copyStringMap(base)
	if dst == nil {
		dst = make(map[string]string, len(overlay))
	}
	for k, v := range overlay {
		dst[k] = v
	}
	return dst
}

// mergeHeaderMap 合并 header / metadata，按大小写不敏感覆盖，避免同名 key 重复
func mergeHeaderMap(base, overlay map[string]string) map[string]string {
	if len(overlay) == 0 {
		return base
	}
	dst := copyStringMap(base)
	if dst == nil {
		dst = make(map[string]string, len(overlay))
	}
	for k, v := range overlay {
		for existing := range dst {
			if existing != k && strings.EqualFold(existing, k) {
				delete(dst, existing)
			}
		}
		dst[k] = v
	}
	return dst
}

// SaveGlobalConfig 保存全局配置
func SaveGlobalConfig(cfg *model.GlobalConfig) error {
	path, err := GlobalConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// GetEnvironment 获取指定环境配置，未指定时返回第一个环境
func GetEnvironment(cfg *model.GlobalConfig, envName string) (string, *model.Environment, error) {
	if len(cfg.Environments) == 0 {
		return "", nil, fmt.Errorf("未配置任何环境")
	}

	if envName != "" {
		env, ok := cfg.Environments[envName]
		if !ok {
			return "", nil, fmt.Errorf("环境 %q 不存在", envName)
		}
		return envName, env, nil
	}

	// 返回第一个环境
	for name, env := range cfg.Environments {
		return name, env, nil
	}

	return "", nil, fmt.Errorf("未配置任何环境")
}

// defaultGlobalConfig 返回默认全局配置
func defaultGlobalConfig() *model.GlobalConfig {
	return &model.GlobalConfig{
		Global: model.GlobalSettings{
			Timeout: "30s",
			Output:  "pretty",
		},
		Environments: make(map[string]*model.Environment),
	}
}

// InitConfigDir 初始化配置目录和默认配置文件
func InitConfigDir() error {
	dir, err := LionDir()
	if err != nil {
		return err
	}

	dirs := []string{
		dir,
		filepath.Join(dir, "api"),
		filepath.Join(dir, "scripts"),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %w", d, err)
		}
	}

	// 如果 global.yaml 不存在，创建默认配置
	path := filepath.Join(dir, "global.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return SaveGlobalConfig(defaultGlobalConfig())
	}

	return nil
}
