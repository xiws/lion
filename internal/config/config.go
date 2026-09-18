package config

import (
	"fmt"
	"os"
	"path/filepath"

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
