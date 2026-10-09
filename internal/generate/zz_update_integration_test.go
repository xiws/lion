package generate

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lion/internal/collection"
)

const swaggerV1 = `{
  "swagger": "2.0",
  "basePath": "/api",
  "paths": {
    "/order/create": {
      "post": {
        "operationId": "createOrder",
        "summary": "创建订单",
        "parameters": [
          {"name": "body", "in": "body", "schema": {"$ref": "#/definitions/Order"}}
        ]
      }
    }
  },
  "definitions": {
    "Order": {
      "type": "object",
      "properties": {
        "orderNo": {"type": "string"},
        "amount": {"type": "integer"}
      }
    }
  }
}`

const swaggerV2 = `{
  "swagger": "2.0",
  "basePath": "/api",
  "paths": {
    "/order/create": {
      "post": {
        "operationId": "createOrder",
        "summary": "创建订单",
        "parameters": [
          {"name": "body", "in": "body", "schema": {"$ref": "#/definitions/Order"}}
        ]
      }
    },
    "/order/cancel": {
      "post": {
        "operationId": "cancelOrder",
        "summary": "取消订单",
        "parameters": [
          {"name": "body", "in": "body", "schema": {"$ref": "#/definitions/Order"}}
        ]
      }
    }
  },
  "definitions": {
    "Order": {
      "type": "object",
      "properties": {
        "orderNo": {"type": "string"},
        "amount": {"type": "integer"}
      }
    }
  }
}`

func TestUpdateCommandFlow(t *testing.T) {
	// 隔离 ~/.lion
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1.json", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(swaggerV1))
	})
	mux.HandleFunc("/v2.json", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(swaggerV2))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	srcV1 := srv.URL + "/v1.json"
	srcV2 := srv.URL + "/v2.json"

	// 1. 首次生成
	if err := Run(&Options{Name: "wms", Type: "swagger", Source: srcV1, Service: "wms-service"}); err != nil {
		t.Fatalf("首次生成失败: %v", err)
	}

	mdPath := filepath.Join(home, ".lion", "api", "wms", "order.md")
	raw, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	for _, want := range []string{"> source_type: swagger", "> source_url: " + srcV1, "> collection: wms", "> service: wms-service"} {
		if !strings.Contains(content, want) {
			t.Fatalf("元数据缺少 %q:\n%s", want, content)
		}
	}
	if !strings.Contains(content, "### POST /api/order/create `createOrder`") {
		t.Fatalf("接口标题不符合预期:\n%s", content)
	}

	// 2. 模拟用户编辑 JSON 代码块 + 来源地址指向 v2
	edited := strings.Replace(content, `"amount": 0`, `"amount": 999`, 1)
	if edited == content {
		t.Fatalf("未能替换 amount 默认值:\n%s", content)
	}
	edited = strings.Replace(edited, srcV1, srcV2, 1)
	if err := os.WriteFile(mdPath, []byte(edited), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. 更新（不传 --type / source）
	opts := &Options{Name: "wms"}
	if err := Update(opts); err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if opts.Source != srcV2 || opts.Type != "swagger" || opts.Service != "wms-service" {
		t.Fatalf("来源信息未从元数据还原: %+v", opts)
	}

	raw, err = os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	content = string(raw)
	if !strings.Contains(content, "cancelOrder") {
		t.Fatalf("更新后未包含新增接口:\n%s", content)
	}
	if !strings.Contains(content, `"amount": 999`) {
		t.Fatalf("更新后未保留用户编辑的 JSON 值:\n%s", content)
	}
	if !strings.Contains(content, "> service: wms-service") {
		t.Fatalf("更新后未保留 service:\n%s", content)
	}

	// 4. 不存在的 Collection 更新应报错
	err = Update(&Options{Name: "not-exist"})
	if err == nil {
		t.Fatal("更新不存在的 Collection 应当报错")
	}
	if !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("错误信息不符合预期: %v", err)
	}

	// 5. 元数据缺少 source_url 时的提示
	oldDir := filepath.Join(home, ".lion", "api", "legacy")
	if err := os.MkdirAll(oldDir, 0755); err != nil {
		t.Fatal(err)
	}
	legacy := "# legacy\n\n> source: swagger\n> collection: legacy\n\n### GET /a `a`\n\n```json\n{}\n```\n"
	if err := os.WriteFile(filepath.Join(oldDir, "a.md"), []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}
	err = Update(&Options{Name: "legacy"})
	if err == nil || !strings.Contains(err.Error(), "source_url") {
		t.Fatalf("缺少 source_url 时应提示显式传参，实际: %v", err)
	}

	// 6. Exists 判定
	ok, err := collection.Exists("wms")
	if err != nil || !ok {
		t.Fatalf("Exists(wms) = %v, %v", ok, err)
	}
	ok, err = collection.Exists("nope")
	if err != nil || ok {
		t.Fatalf("Exists(nope) = %v, %v", ok, err)
	}
}
