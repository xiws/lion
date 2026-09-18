package markdown

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"lion/internal/model"
)

var (
	// 匹配三级标题: ### POST /order/create `createOrder`
	reAPIHeader = regexp.MustCompile(`^###\s+(.+?)\s+` + "`" + `(.+?)` + "`" + `\s*$`)
	// 匹配 script 引用块: > script: ~/.lion/scripts/xxx.py
	reScript = regexp.MustCompile(`^>\s+script:\s*(.+?)\s*$`)
	// 匹配元数据引用块: > key: value
	reMeta = regexp.MustCompile(`^>\s+(\w+):\s*(.+?)\s*$`)
	// 匹配一级标题: # group_name
	reGroupTitle = regexp.MustCompile(`^#\s+(.+?)\s*$`)
)

// ParseGroupFile 解析分组 Markdown 文件，返回 Group 对象
func ParseGroupFile(filePath string) (*model.Group, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	group := &model.Group{
		Name: strings.TrimSuffix(filepath.Base(filePath), ".md"),
		Path: filePath,
	}

	scanner := bufio.NewScanner(f)
	var currentAPI *model.API
	var inJSONBlock bool
	var jsonLines []string

	for scanner.Scan() {
		line := scanner.Text()

		// 处理 JSON 代码块
		if strings.HasPrefix(line, "```json") {
			inJSONBlock = true
			jsonLines = nil
			continue
		}
		if inJSONBlock && strings.HasPrefix(line, "```") {
			inJSONBlock = false
			if currentAPI != nil {
				currentAPI.BodyJSON = strings.Join(jsonLines, "\n")
			}
			continue
		}
		if inJSONBlock {
			jsonLines = append(jsonLines, line)
			continue
		}

		// 匹配接口标题
		if m := reAPIHeader.FindStringSubmatch(line); m != nil {
			// 保存上一个接口
			if currentAPI != nil {
				group.APIs = append(group.APIs, currentAPI)
			}
			currentAPI = &model.API{
				Method: m[1],
				Path:   m[1],
				ID:     m[2],
			}
			// 对于 HTTP 接口，Method 和 Path 需要拆分
			parts := strings.SplitN(m[1], " ", 2)
			if len(parts) == 2 {
				currentAPI.Method = parts[0]
				currentAPI.Path = parts[1]
			}
			continue
		}

		// 匹配接口脚本
		if currentAPI != nil {
			if m := reScript.FindStringSubmatch(line); m != nil {
				currentAPI.Script = m[1]
				continue
			}
		}
	}

	// 保存最后一个接口
	if currentAPI != nil {
		group.APIs = append(group.APIs, currentAPI)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	return group, nil
}

// ParseCollectionDir 解析整个 Collection 目录
func ParseCollectionDir(collectionName string) (*model.Collection, error) {
	dir, err := collectionDir(collectionName)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return &model.Collection{Name: collectionName}, nil
		}
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	collection := &model.Collection{
		Name: collectionName,
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		group, err := ParseGroupFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("解析文件 %s 失败: %w", filePath, err)
		}

		// 从第一个分组文件的元数据中提取 Collection 级别信息
		if len(collection.Groups) == 0 {
			meta, err := ParseMetadata(filePath)
			if err == nil {
				collection.Source = meta["source"]
				collection.Service = meta["service"]
			}
		}

		collection.Groups = append(collection.Groups, group)
	}

	return collection, nil
}

// ParseMetadata 解析 Markdown 文件的元数据（> key: value）
func ParseMetadata(filePath string) (map[string]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	meta := make(map[string]string)
	scanner := bufio.NewScanner(f)
	foundTitle := false

	for scanner.Scan() {
		line := scanner.Text()

		// 跳过一级标题
		if !foundTitle {
			if reGroupTitle.MatchString(line) {
				foundTitle = true
				continue
			}
			continue
		}

		// 解析元数据
		if m := reMeta.FindStringSubmatch(line); m != nil {
			meta[m[1]] = m[2]
			continue
		}

		// 遇到非引用块内容就停止
		if !strings.HasPrefix(line, ">") && strings.TrimSpace(line) != "" {
			break
		}
	}

	return meta, scanner.Err()
}

// FindAPIByID 在 Collection 中按 ID 查找接口
func FindAPIByID(collection *model.Collection, apiID string) (*model.Group, *model.API, error) {
	for _, g := range collection.Groups {
		for _, api := range g.APIs {
			if api.ID == apiID {
				return g, api, nil
			}
		}
	}
	return nil, nil, fmt.Errorf("接口 %q 不存在", apiID)
}

// collectionDir 返回 Collection 目录路径
func collectionDir(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户主目录失败: %w", err)
	}
	return filepath.Join(home, ".lion", "api", name), nil
}
