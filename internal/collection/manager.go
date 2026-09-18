package collection

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lion/internal/config"
	"lion/internal/markdown"
	"lion/internal/model"
)

// List 列出所有 Collection
func List() ([]string, error) {
	dir, err := config.LionDir()
	if err != nil {
		return nil, err
	}

	apiDir := filepath.Join(dir, "api")
	entries, err := os.ReadDir(apiDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取 api 目录失败: %w", err)
	}

	var collections []string
	for _, entry := range entries {
		if entry.IsDir() {
			collections = append(collections, entry.Name())
		}
	}

	return collections, nil
}

// Load 加载指定 Collection
func Load(name string) (*model.Collection, error) {
	return markdown.ParseCollectionDir(name)
}

// ListAPIs 列出 Collection 下所有接口
func ListAPIs(name string) ([]*model.API, error) {
	collection, err := Load(name)
	if err != nil {
		return nil, err
	}

	var apis []*model.API
	for _, g := range collection.Groups {
		apis = append(apis, g.APIs...)
	}

	return apis, nil
}

// GetAPI 根据 ID 获取接口详情
func GetAPI(apiID string) (*model.Collection, *model.Group, *model.API, error) {
	collections, err := List()
	if err != nil {
		return nil, nil, nil, err
	}

	for _, name := range collections {
		collection, err := Load(name)
		if err != nil {
			continue
		}

		group, api, err := markdown.FindAPIByID(collection, apiID)
		if err == nil {
			return collection, group, api, nil
		}
	}

	return nil, nil, nil, fmt.Errorf("接口 %q 不存在", apiID)
}

// Delete 删除 Collection
func Delete(name string) error {
	dir, err := config.LionDir()
	if err != nil {
		return err
	}

	collectionDir := filepath.Join(dir, "api", name)
	if _, err := os.Stat(collectionDir); os.IsNotExist(err) {
		return fmt.Errorf("Collection %q 不存在", name)
	}

	return os.RemoveAll(collectionDir)
}

// Export 导出 Collection 为 JSON
func Export(name string) (string, error) {
	collection, err := Load(name)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(collection, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化 Collection 失败: %w", err)
	}

	return string(data), nil
}

// ResolveCollectionAndGroup 从 api-id 中解析 collection 和 group
// api-id 格式: collection.apiID 或 apiID
func ResolveCollectionAndGroup(apiID string) (collectionName, id string) {
	parts := strings.SplitN(apiID, ".", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", apiID
}
