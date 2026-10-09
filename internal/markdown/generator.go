package markdown

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lion/internal/model"
)

// GenerateGroupMarkdown 生成分组 Markdown 文件
//
// 来源信息（source_type / source_url / proto_root）写入元数据，供
// `lion generate --name <name> --update` 在只提供 --name 时还原首次生成的参数。
func GenerateGroupMarkdown(collection *model.Collection, group *model.Group, force bool) error {
	dir, err := collectionDir(collection.Name)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	filePath := filepath.Join(dir, group.Name+".md")
	group.Path = filePath

	service := collection.Service

	// 如果文件已存在且非强制模式，合并用户编辑的默认值
	if !force {
		if existing, err := ParseGroupFile(filePath); err == nil {
			mergeDefaults(existing, group)
		}
		// 保留用户已配置的 service（从已有元数据读取）
		if service == "" {
			if meta, err := ParseMetadata(filePath); err == nil && meta["service"] != "" {
				service = meta["service"]
			}
		}
	}

	var sb strings.Builder
	// 写入一级标题
	sb.WriteString(fmt.Sprintf("# %s\n\n", group.Name))

	// 写入元数据
	sb.WriteString(fmt.Sprintf("> source: %s\n", guessSource(group)))
	if collection.Source != "" {
		sb.WriteString(fmt.Sprintf("> source_type: %s\n", collection.Source))
	}
	if collection.SourceURL != "" {
		sb.WriteString(fmt.Sprintf("> source_url: %s\n", collection.SourceURL))
	}
	if len(collection.ProtoRoot) > 0 {
		sb.WriteString(fmt.Sprintf("> proto_root: %s\n", strings.Join(collection.ProtoRoot, " ")))
	}
	sb.WriteString(fmt.Sprintf("> collection: %s\n", collection.Name))
	if service != "" {
		sb.WriteString(fmt.Sprintf("> service: %s\n", service))
	}
	sb.WriteString("\n")

	// 写入每个接口
	for _, api := range group.APIs {
		header := api.Path
		if api.Method != "" && api.Method != "gRPC" && api.Method != api.Path {
			header = api.Method + " " + api.Path
		}
		sb.WriteString(fmt.Sprintf("### %s `%s`\n\n", header, api.ID))

		for _, s := range api.Scripts {
			sb.WriteString(fmt.Sprintf("> script: %s\n", s))
		}
		if len(api.Scripts) > 0 {
			sb.WriteString("\n")
		}

		// 写入 JSON 代码块
		sb.WriteString("```json\n")
		if api.BodyJSON != "" {
			sb.WriteString(api.BodyJSON)
		} else {
			sb.WriteString("{}")
		}
		sb.WriteString("\n```\n\n")
	}

	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}

// GenerateCollectionMarkdowns 为整个 Collection 生成 Markdown 文件
func GenerateCollectionMarkdowns(collection *model.Collection, force bool) error {
	if collection.Name == "" {
		return fmt.Errorf("Collection 名称不能为空")
	}

	var firstErr error
	for _, group := range collection.Groups {
		if err := GenerateGroupMarkdown(collection, group, force); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("生成分组 %s 失败: %w", group.Name, err)
			}
		}
	}
	return firstErr
}

// FormatBodyJSON 格式化 JSON 字符串（带缩进）
func FormatBodyJSON(body string) (string, error) {
	var v interface{}
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// mergeDefaults 合并用户编辑的默认值到新接口中
func mergeDefaults(old, new *model.Group) {
	oldMap := make(map[string]*model.API)
	for _, api := range old.APIs {
		oldMap[api.ID] = api
	}

	for _, api := range new.APIs {
		if oldAPI, ok := oldMap[api.ID]; ok {
			// 保留用户编辑的 BodyJSON
			if oldAPI.BodyJSON != "" {
				api.BodyJSON = oldAPI.BodyJSON
			}
			// 保留用户配置的脚本
			if len(oldAPI.Scripts) > 0 {
				api.Scripts = oldAPI.Scripts
			}
		}
	}
}

// guessSource 根据接口信息推断来源类型
func guessSource(group *model.Group) string {
	if len(group.APIs) == 0 {
		return "unknown"
	}
	// gRPC 接口通常没有 HTTP Method
	api := group.APIs[0]
	if api.Method == "" || api.Method == "gRPC" {
		return "grpc"
	}
	return "swagger"
}
