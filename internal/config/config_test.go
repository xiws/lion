package config

import (
	"reflect"
	"testing"

	"lion/internal/model"
)

func TestMergeHeaderMapCaseInsensitiveAndEmptyKeys(t *testing.T) {
	base := map[string]string{
		"X-Tenant-Id": "T001",
		"X-Keep":      "keep",
		"":            "empty-base",
		"   ":         "blank-base",
	}
	overlay := map[string]string{
		"x-tenant-id": "T002",
		"X-New":       "new",
		"":            "empty-overlay",
	}

	got := mergeHeaderMap(base, overlay)
	want := map[string]string{
		"x-tenant-id": "T002",
		"X-Keep":      "keep",
		"X-New":       "new",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mergeHeaderMap() = %#v, want %#v", got, want)
	}

	// 不应修改入参
	if base["X-Tenant-Id"] != "T001" {
		t.Fatalf("base map was mutated: %#v", base)
	}

	// 没有 overlay 时也应返回副本，避免调用方修改 base
	withoutOverlay := mergeHeaderMap(base, nil)
	withoutOverlay["X-Keep"] = "changed"
	if base["X-Keep"] != "keep" {
		t.Fatalf("mergeHeaderMap() returned an aliased map: %#v", base)
	}
}

func TestMergeGlobalConfigHeaderLayers(t *testing.T) {
	base := &model.GlobalConfig{
		Global: model.GlobalSettings{
			Headers: map[string]string{
				"X-Layer":  "global",
				"X-Global": "global-only",
			},
		},
		Environments: map[string]*model.Environment{
			"dev": {
				Host: "127.0.0.1:8080",
				Headers: map[string]string{
					"X-Layer": "global-env",
					"X-Env":   "global-env-only",
				},
			},
			"test": {
				Host: "127.0.0.1:8081",
				Headers: map[string]string{
					"X-Layer": "global-test-env",
				},
			},
		},
	}
	overlay := &model.GlobalConfig{
		Global: model.GlobalSettings{
			Headers: map[string]string{
				"X-Layer":      "collection",
				"X-Collection": "collection-only",
			},
		},
		Environments: map[string]*model.Environment{
			"dev": {
				Headers: map[string]string{
					"X-Layer":          "collection-env",
					"X-Collection-Env": "collection-env-only",
				},
			},
			"staging": {
				Headers: map[string]string{
					"X-Layer": "collection-new-env",
				},
			},
		},
	}

	merged := mergeGlobalConfig(base, overlay)

	wantGlobal := map[string]string{
		"X-Layer":      "collection",
		"X-Global":     "global-only",
		"X-Collection": "collection-only",
	}
	if !reflect.DeepEqual(merged.Global.Headers, wantGlobal) {
		t.Fatalf("merged global headers = %#v, want %#v", merged.Global.Headers, wantGlobal)
	}

	wantDev := map[string]string{
		"X-Layer":          "collection-env",
		"X-Env":            "global-env-only",
		"X-Collection":     "collection-only",
		"X-Collection-Env": "collection-env-only",
	}
	if !reflect.DeepEqual(merged.Environments["dev"].Headers, wantDev) {
		t.Fatalf("merged dev headers = %#v, want %#v", merged.Environments["dev"].Headers, wantDev)
	}

	// collection 没有为 test 环境写 headers，但 global.headers 仍应生效
	if got := merged.Environments["test"].Headers["X-Layer"]; got != "collection" {
		t.Fatalf("test env X-Layer = %q, want %q", got, "collection")
	}
	if got := merged.Environments["test"].Host; got != "127.0.0.1:8081" {
		t.Fatalf("test env host = %q, want %q", got, "127.0.0.1:8081")
	}

	// collection 新增的环境应先应用 collection.global.headers，再应用环境级 headers
	if got := merged.Environments["staging"].Headers["X-Layer"]; got != "collection-new-env" {
		t.Fatalf("staging env X-Layer = %q, want %q", got, "collection-new-env")
	}
	if got := merged.Environments["staging"].Headers["X-Collection"]; got != "collection-only" {
		t.Fatalf("staging env X-Collection = %q, want %q", got, "collection-only")
	}

	// 最终发送时的顺序：global -> env -> CLI
	resolve := func(envName string, cli map[string]string) map[string]string {
		out := mergeHeaderMap(nil, merged.Global.Headers)
		if env := merged.Environments[envName]; env != nil {
			out = mergeHeaderMap(out, env.Headers)
		}
		return mergeHeaderMap(out, cli)
	}
	if got := resolve("dev", nil)["X-Layer"]; got != "collection-env" {
		t.Fatalf("final dev X-Layer = %q, want %q", got, "collection-env")
	}
	if got := resolve("dev", map[string]string{"X-Layer": "cli"})["X-Layer"]; got != "cli" {
		t.Fatalf("final dev X-Layer with CLI = %q, want %q", got, "cli")
	}

	// 合并过程不能修改 base
	if got := base.Global.Headers["X-Layer"]; got != "global" {
		t.Fatalf("base global X-Layer mutated to %q", got)
	}
	if got := base.Environments["dev"].Headers["X-Layer"]; got != "global-env" {
		t.Fatalf("base dev X-Layer mutated to %q", got)
	}
}
