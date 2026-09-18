package generate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"lion/internal/markdown"
	"lion/internal/model"
)

// Generator 接口生成器接口
type Generator interface {
	Generate() (*model.Collection, error)
}

// Options 生成选项
type Options struct {
	Name      string   // Collection 名称
	Type      string   // 来源类型: swagger / grpc / proto
	Source    string   // 来源地址
	ProtoRoot []string // proto import 根目录
	Force     bool     // 强制重新生成
	Service   string   // Consul 服务名称
}

// Run 执行生成
func Run(opts *Options) error {
	var gen Generator
	var err error

	switch opts.Type {
	case "swagger":
		gen, err = NewSwaggerGenerator(opts)
	case "grpc":
		gen, err = NewGRPCGenerator(opts)
	case "proto":
		gen, err = NewProtoGenerator(opts)
	default:
		return fmt.Errorf("不支持的来源类型: %s", opts.Type)
	}

	if err != nil {
		return err
	}

	collection, err := gen.Generate()
	if err != nil {
		return err
	}

	// 设置 Consul 服务名称
	if opts.Service != "" {
		collection.Service = opts.Service
	}

	return markdown.GenerateCollectionMarkdowns(collection, opts.Force)
}

// SwaggerGenerator Swagger/OpenAPI 生成器
type SwaggerGenerator struct {
	opts *Options
	doc  *SwaggerDoc
}

// NewSwaggerGenerator 创建 Swagger 生成器
func NewSwaggerGenerator(opts *Options) (*SwaggerGenerator, error) {
	return &SwaggerGenerator{opts: opts}, nil
}

// Generate 从 Swagger/OpenAPI 文档生成 Collection
func (g *SwaggerGenerator) Generate() (*model.Collection, error) {
	doc, err := g.fetchDoc()
	if err != nil {
		return nil, err
	}

	collection := &model.Collection{
		Name:      g.opts.Name,
		Source:    "swagger",
		SourceURL: g.opts.Source,
	}

	// 按一级路径分组
	groupMap := make(map[string]*model.Group)

	for path, methods := range doc.Paths {
		groupName := extractGroupName(path)

		if _, ok := groupMap[groupName]; !ok {
			groupMap[groupName] = &model.Group{
				Name: groupName,
			}
		}
		group := groupMap[groupName]

		// 处理每个 HTTP 方法
		for method, op := range methods.Operations() {
			if op == nil {
				continue
			}

			apiID := op.OperationID
			if apiID == "" {
				apiID = fmt.Sprintf("%s.%s.%s", g.opts.Name, groupName, method)
			}

			bodyJSON := g.buildBodyJSON(op, doc)

			// 拼接 basePath（Swagger 2.0）
			fullPath := doc.BasePath + path

			api := &model.API{
				ID:          apiID,
				Method:      strings.ToUpper(method),
				Path:        fullPath,
				BodyJSON:    bodyJSON,
				Description: op.Summary,
			}

			group.APIs = append(group.APIs, api)
		}
	}

	for _, group := range groupMap {
		collection.Groups = append(collection.Groups, group)
	}

	return collection, nil
}

// fetchDoc 获取 Swagger 文档
func (g *SwaggerGenerator) fetchDoc() (*SwaggerDoc, error) {
	var data []byte
	var err error

	if strings.HasPrefix(g.opts.Source, "http://") || strings.HasPrefix(g.opts.Source, "https://") {
		resp, err := http.Get(g.opts.Source)
		if err != nil {
			return nil, fmt.Errorf("获取 Swagger 文档失败: %w", err)
		}
		defer resp.Body.Close()

		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("读取 Swagger 文档失败: %w", err)
		}
	} else {
		data, err = io.ReadAll(strings.NewReader(g.opts.Source))
		if err != nil {
			return nil, fmt.Errorf("读取 Swagger 文档失败: %w", err)
		}
	}

	var doc SwaggerDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("解析 Swagger 文档失败: %w", err)
	}

	return &doc, nil
}

// buildBodyJSON 构建请求体 JSON
func (g *SwaggerGenerator) buildBodyJSON(op *SwaggerOperation, doc *SwaggerDoc) string {
	body := make(map[string]interface{})

	// 从请求体参数构建 JSON
	for _, param := range op.Parameters {
		if param.In == "body" && param.Schema != nil {
			body = g.resolveSchema(param.Schema, doc)
			break
		}
	}

	data, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}

// resolveSchema 解析 Schema 为 map
func (g *SwaggerGenerator) resolveSchema(schema *SwaggerSchema, doc *SwaggerDoc) map[string]interface{} {
	result := make(map[string]interface{})

	// 处理 $ref
	if schema.Ref != "" {
		resolved := doc.ResolveRef(schema.Ref)
		if resolved != nil {
			return g.resolveSchema(resolved, doc)
		}
		return result
	}

	for name, prop := range schema.Properties {
		result[name] = g.resolveValue(&prop, doc)
	}

	return result
}

// resolveValue 解析属性值
func (g *SwaggerGenerator) resolveValue(schema *SwaggerSchema, doc *SwaggerDoc) interface{} {
	if schema.Ref != "" {
		resolved := doc.ResolveRef(schema.Ref)
		if resolved != nil {
			return g.resolveSchema(resolved, doc)
		}
		return nil
	}

	switch schema.Type {
	case "string":
		return ""
	case "integer", "number":
		return 0
	case "boolean":
		return false
	case "array":
		if schema.Items != nil {
			return []interface{}{g.resolveValue(schema.Items, doc)}
		}
		return []interface{}{}
	case "object":
		return g.resolveSchema(schema, doc)
	default:
		return nil
	}
}

// extractGroupName 从路径提取分组名
func extractGroupName(path string) string {
	path = strings.TrimPrefix(path, "/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return "default"
}

// ProtoGenerator Proto 文件生成器
type ProtoGenerator struct {
	opts *Options
}

// NewProtoGenerator 创建 Proto 生成器
func NewProtoGenerator(opts *Options) (*ProtoGenerator, error) {
	return &ProtoGenerator{opts: opts}, nil
}

// Generate 从 Proto 文件生成 Collection
func (g *ProtoGenerator) Generate() (*model.Collection, error) {
	// TODO: 实现 Proto 文件解析
	return nil, fmt.Errorf("Proto 文件生成暂未实现")
}
