package generate

import "encoding/json"

// SwaggerDoc Swagger/OpenAPI 文档
type SwaggerDoc struct {
	Swagger     string                    `json:"swagger"`
	OpenAPI     string                    `json:"openapi"`
	Info        SwaggerInfo               `json:"info"`
	BasePath    string                    `json:"basePath"`
	Paths       map[string]*SwaggerPath   `json:"paths"`
	Definitions map[string]*SwaggerSchema `json:"definitions"`
	Components  struct {
		Schemas map[string]*SwaggerSchema `json:"schemas"`
	} `json:"components"`
}

// SwaggerInfo 文档信息
type SwaggerInfo struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

// SwaggerPath 路径对象
type SwaggerPath struct {
	Get     *SwaggerOperation `json:"get"`
	Post    *SwaggerOperation `json:"post"`
	Put     *SwaggerOperation `json:"put"`
	Delete  *SwaggerOperation `json:"delete"`
	Patch   *SwaggerOperation `json:"patch"`
	Options *SwaggerOperation `json:"options"`
	Head    *SwaggerOperation `json:"head"`
}

// Operations 返回所有操作
func (p *SwaggerPath) Operations() map[string]*SwaggerOperation {
	ops := make(map[string]*SwaggerOperation)
	if p.Get != nil {
		ops["get"] = p.Get
	}
	if p.Post != nil {
		ops["post"] = p.Post
	}
	if p.Put != nil {
		ops["put"] = p.Put
	}
	if p.Delete != nil {
		ops["delete"] = p.Delete
	}
	if p.Patch != nil {
		ops["patch"] = p.Patch
	}
	if p.Options != nil {
		ops["options"] = p.Options
	}
	if p.Head != nil {
		ops["head"] = p.Head
	}
	return ops
}

// SwaggerOperation 操作对象
type SwaggerOperation struct {
	OperationID string              `json:"operationId"`
	Summary     string              `json:"summary"`
	Description string              `json:"description"`
	Parameters  []*SwaggerParam     `json:"parameters"`
	RequestBody *SwaggerRequestBody `json:"requestBody"`
}

// SwaggerParam 参数对象
type SwaggerParam struct {
	Name     string         `json:"name"`
	In       string         `json:"in"` // query, header, path, body
	Required bool           `json:"required"`
	Type     string         `json:"type"`
	Schema   *SwaggerSchema `json:"schema"`
}

// SwaggerRequestBody 请求体对象 (OpenAPI 3.x)
type SwaggerRequestBody struct {
	Content map[string]SwaggerMediaType `json:"content"`
}

// SwaggerMediaType 媒体类型对象
type SwaggerMediaType struct {
	Schema *SwaggerSchema `json:"schema"`
}

// SwaggerSchema Schema 对象
type SwaggerSchema struct {
	Type       string                   `json:"type"`
	Format     string                   `json:"format"`
	Ref        string                   `json:"$ref"`
	Properties map[string]SwaggerSchema `json:"properties"`
	Items      *SwaggerSchema           `json:"items"`
	Required   []string                 `json:"required"`
	Default    interface{}              `json:"default"`
	Example    interface{}              `json:"example"`
}

// ResolveRef 解析 $ref 引用
func (d *SwaggerDoc) ResolveRef(ref string) *SwaggerSchema {
	// 格式: #/definitions/XXX 或 #/components/schemas/XXX
	if len(ref) < 2 {
		return nil
	}

	// Swagger 2.0
	if d.Definitions != nil {
		prefix := "#/definitions/"
		if len(ref) > len(prefix) && ref[:len(prefix)] == prefix {
			name := ref[len(prefix):]
			if schema, ok := d.Definitions[name]; ok {
				return schema
			}
		}
	}

	// OpenAPI 3.x
	if d.Components.Schemas != nil {
		prefix := "#/components/schemas/"
		if len(ref) > len(prefix) && ref[:len(prefix)] == prefix {
			name := ref[len(prefix):]
			if schema, ok := d.Components.Schemas[name]; ok {
				return schema
			}
		}
	}

	return nil
}

// UnmarshalJSON 自定义反序列化
func (p *SwaggerPath) UnmarshalJSON(data []byte) error {
	type Alias SwaggerPath
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(p),
	}
	return json.Unmarshal(data, aux)
}
