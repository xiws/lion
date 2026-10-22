package sender

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"lion/internal/config"
)

const testGlobalYAML = `
global:
  headers:
    X-Layer: global
    X-Global: global-only
environments:
  dev:
    resolve: direct
    host: "127.0.0.1:8080"
    headers:
      X-Layer: global-env
      X-Env: global-env-only
  test:
    resolve: direct
    host: "127.0.0.1:8081"
    headers:
      X-Layer: global-test-env
  prod:
    resolve: direct
    host: "127.0.0.1:8082"
`

const testCollectionAllYAML = `
global:
  headers:
    X-Layer: collection
    X-Collection: collection-only
environments:
  dev:
    headers:
      X-Layer: collection-env
      X-Collection-Env: collection-env-only
`

const testCollectionGlobalOnlyYAML = `
global:
  headers:
    X-Layer: collection
    X-Collection: collection-only
`

const testCollectionEnvOnlyYAML = `
environments:
  dev:
    headers:
      X-Layer: collection-env
      X-Collection-Env: collection-env-only
`

const testGlobalCaseYAML = `
global:
  headers:
    x-layer: global
environments:
  dev:
    headers:
      x-layer: global-env
`

const testCollectionCaseYAML = `
global:
  headers:
    X-Layer: collection
environments:
  dev:
    headers:
      X-Layer: collection-env
`

func TestFixedHeaderPrecedenceAcrossConfigFiles(t *testing.T) {
	cases := []struct {
		name           string
		globalYAML     string
		collectionName string
		collectionYAML string
		env            string
		cli            map[string]string
		want           map[string]string
	}{
		{
			name:           "collection env wins over every config layer",
			globalYAML:     testGlobalYAML,
			collectionName: "petrel",
			collectionYAML: testCollectionAllYAML,
			env:            "dev",
			want: map[string]string{
				"X-Layer":          "collection-env",
				"X-Global":         "global-only",
				"X-Env":            "global-env-only",
				"X-Collection":     "collection-only",
				"X-Collection-Env": "collection-env-only",
			},
		},
		{
			name:           "cli header wins over collection env",
			globalYAML:     testGlobalYAML,
			collectionName: "petrel",
			collectionYAML: testCollectionAllYAML,
			env:            "dev",
			cli:            map[string]string{"x-layer": "cli"},
			want: map[string]string{
				"X-Layer":          "cli",
				"X-Global":         "global-only",
				"X-Env":            "global-env-only",
				"X-Collection":     "collection-only",
				"X-Collection-Env": "collection-env-only",
			},
		},
		{
			name:           "collection global wins over global env",
			globalYAML:     testGlobalYAML,
			collectionName: "petrel",
			collectionYAML: testCollectionGlobalOnlyYAML,
			env:            "dev",
			want: map[string]string{
				"X-Layer":      "collection",
				"X-Global":     "global-only",
				"X-Env":        "global-env-only",
				"X-Collection": "collection-only",
			},
		},
		{
			name:           "collection env wins when collection has no global headers",
			globalYAML:     testGlobalYAML,
			collectionName: "petrel",
			collectionYAML: testCollectionEnvOnlyYAML,
			env:            "dev",
			want: map[string]string{
				"X-Layer":          "collection-env",
				"X-Global":         "global-only",
				"X-Env":            "global-env-only",
				"X-Collection-Env": "collection-env-only",
			},
		},
		{
			name:           "missing collection config keeps global env headers",
			globalYAML:     testGlobalYAML,
			collectionName: "missing",
			env:            "dev",
			want: map[string]string{
				"X-Layer":  "global-env",
				"X-Global": "global-only",
				"X-Env":    "global-env-only",
			},
		},
		{
			name:           "empty collection config keeps global env headers",
			globalYAML:     testGlobalYAML,
			collectionName: "petrel",
			collectionYAML: "\n",
			env:            "dev",
			want: map[string]string{
				"X-Layer":  "global-env",
				"X-Global": "global-only",
				"X-Env":    "global-env-only",
			},
		},
		{
			name:           "collection global applies to env not mentioned in collection",
			globalYAML:     testGlobalYAML,
			collectionName: "petrel",
			collectionYAML: testCollectionGlobalOnlyYAML,
			env:            "test",
			want: map[string]string{
				"X-Layer":      "collection",
				"X-Global":     "global-only",
				"X-Collection": "collection-only",
			},
		},
		{
			name:           "global headers alone",
			globalYAML:     testGlobalYAML,
			collectionName: "missing",
			env:            "prod",
			want: map[string]string{
				"X-Layer":  "global",
				"X-Global": "global-only",
			},
		},
		{
			name:           "header names are case insensitive across files",
			globalYAML:     testGlobalCaseYAML,
			collectionName: "petrel",
			collectionYAML: testCollectionCaseYAML,
			env:            "dev",
			want: map[string]string{
				"X-Layer": "collection-env",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)

			lionDir := filepath.Join(home, ".lion")
			if err := os.MkdirAll(lionDir, 0o755); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}
			if err := os.WriteFile(filepath.Join(lionDir, "global.yaml"), []byte(tc.globalYAML), 0o644); err != nil {
				t.Fatalf("write global.yaml: %v", err)
			}
			if tc.collectionYAML != "" {
				path := filepath.Join(lionDir, tc.collectionName+".yaml")
				if err := os.WriteFile(path, []byte(tc.collectionYAML), 0o644); err != nil {
					t.Fatalf("write collection config: %v", err)
				}
			}

			cfg, err := config.LoadGlobalConfig()
			if err != nil {
				t.Fatalf("LoadGlobalConfig: %v", err)
			}
			cfg, err = config.ApplyCollectionConfig(cfg, tc.collectionName)
			if err != nil {
				t.Fatalf("ApplyCollectionConfig: %v", err)
			}

			s, err := NewSender(cfg, tc.env, "", "", "", false, tc.cli)
			if err != nil {
				t.Fatalf("NewSender: %v", err)
			}

			if !reflect.DeepEqual(s.headers, tc.want) {
				t.Fatalf("headers = %#v, want %#v", s.headers, tc.want)
			}
		})
	}
}
