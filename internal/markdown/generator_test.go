package markdown

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lion/internal/model"
)

// newGroup 构造一个待写入的分组
func newGroup() (*model.Collection, *model.Group) {
	coll := &model.Collection{Name: "wms", Source: "swagger", SourceURL: "http://example.com/v1.json"}
	group := &model.Group{
		Name: "order",
		APIs: []*model.API{{
			ID:       "createOrder",
			Method:   "POST",
			Path:     "/api/order/create",
			BodyJSON: "{\n  \"amount\": 0\n}",
			Scripts:  []string{"~/.lion/scripts/new.py"},
		}},
	}
	return coll, group
}

const existingMarkdown = "# order\n\n> source: swagger\n> source_url: http://example.com/v1.json\n> collection: wms\n\n" +
	"### POST /api/order/create `createOrder`\n\n> script: ~/.lion/scripts/user.py\n\n" +
	"```json\n{\n  \"amount\": 999\n}\n```\n"

// 非 force（含 --update）应保留用户编辑的 JSON 值与脚本配置
func TestGenerateGroupMarkdown_MergesUserEdits(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	coll, group := newGroup()
	filePath := filepath.Join(home, ".lion", "api", "wms", "order.md")
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte(existingMarkdown), 0644); err != nil {
		t.Fatal(err)
	}

	if err := GenerateGroupMarkdown(coll, group, false); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, filePath)
	if !strings.Contains(got, `"amount": 999`) {
		t.Errorf("未保留用户编辑的 JSON 值:\n%s", got)
	}
	if !strings.Contains(got, "user.py") {
		t.Errorf("未保留用户配置的脚本:\n%s", got)
	}
}

// force 为整体覆盖：不保留用户编辑的 JSON 值与脚本配置
func TestGenerateGroupMarkdown_ForceOverwrites(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	coll, group := newGroup()
	filePath := filepath.Join(home, ".lion", "api", "wms", "order.md")
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte(existingMarkdown), 0644); err != nil {
		t.Fatal(err)
	}

	if err := GenerateGroupMarkdown(coll, group, true); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, filePath)
	if strings.Contains(got, "999") {
		t.Errorf("--force 应整体覆盖，不应保留用户编辑值:\n%s", got)
	}
	if strings.Contains(got, "user.py") {
		t.Errorf("--force 应整体覆盖，不应保留用户脚本:\n%s", got)
	}
	if !strings.Contains(got, "new.py") {
		t.Errorf("--force 应写入来源提供的脚本配置:\n%s", got)
	}
}

// 元数据中应记录来源信息，供 --update 还原参数
func TestGenerateGroupMarkdown_WritesSourceMetadata(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	coll, group := newGroup()
	coll.Source = "proto"
	coll.SourceURL = "./protos"
	coll.ProtoRoot = []string{"./protos", "./third_party/protos"}
	coll.Service = "wms-service"

	if err := GenerateGroupMarkdown(coll, group, false); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, filepath.Join(home, ".lion", "api", "wms", "order.md"))
	for _, want := range []string{
		"> source_type: proto",
		"> source_url: ./protos",
		"> proto_root: ./protos ./third_party/protos",
		"> collection: wms",
		"> service: wms-service",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("元数据缺少 %q:\n%s", want, got)
		}
	}

	// 解析回读
	parsed, err := ParseCollectionDir("wms")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Source != "proto" || parsed.SourceURL != "./protos" || parsed.Service != "wms-service" {
		t.Errorf("来源信息回读不一致: %+v", parsed)
	}
	if len(parsed.ProtoRoot) != 2 || parsed.ProtoRoot[0] != "./protos" || parsed.ProtoRoot[1] != "./third_party/protos" {
		t.Errorf("proto_root 回读不一致: %v", parsed.ProtoRoot)
	}
}

// 旧版元数据只有 > source: 时，应退化为该值
func TestParseCollectionDir_LegacyMetadata(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".lion", "api", "legacy")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	legacy := "# a\n\n> source: swagger\n> collection: legacy\n\n### GET /a `a`\n\n```json\n{}\n```\n"
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseCollectionDir("legacy")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Source != "swagger" {
		t.Errorf("Source = %q, 期望 swagger", parsed.Source)
	}
	if parsed.SourceURL != "" {
		t.Errorf("旧版元数据不应有 source_url, 实际 %q", parsed.SourceURL)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
