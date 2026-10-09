package generate

import (
	"fmt"

	"lion/internal/collection"
)

// Update 更新一个已经存在的 Collection
//
// 与 Run 的区别：首次生成时写入 Markdown 元数据的来源信息（source_type /
// source_url / proto_root / service）会被自动读回，因此
// `lion generate --name wms --update` 不需要重复指定 --type 和 source。
//
// 更新采用合并模式：接口列表重新从来源拉取，已存在的接口 ID 保留用户在 JSON 代码块
// 中编辑过的值与 `> script:` 配置（即 opts.Force 保持 false）；新增接口使用默认值。
// 显式传入的参数优先于元数据中的同名值。
func Update(opts *Options) error {
	// 如果目标 Collection 还不存在，则本次没有可更新的内容
	exists, err := collection.Exists(opts.Name)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("Collection %q 不存在，无法更新；请先执行首次生成，例如:\n  lion generate --name %s --type <swagger|grpc|proto> <source>",
			opts.Name, opts.Name)
	}

	stored, err := LoadSourceInfo(opts.Name)
	if err != nil {
		return err
	}

	// 补齐未显式指定的参数
	if opts.Type == "" {
		opts.Type = stored.Type
	}
	if opts.Source == "" {
		opts.Source = stored.SourceURL
	}
	if len(opts.ProtoRoot) == 0 {
		opts.ProtoRoot = stored.ProtoRoot
	}
	if opts.Service == "" {
		opts.Service = stored.Service
	}
	// 更新 = 合并，保留用户编辑的 JSON 值与脚本配置
	opts.Force = false

	if opts.Type == "" {
		return fmt.Errorf("Collection %q 的元数据中没有记录来源类型，请显式指定 --type", opts.Name)
	}
	if opts.Source == "" {
		return fmt.Errorf("Collection %q 的元数据中没有记录来源地址（> source_url:），请显式指定 source 参数", opts.Name)
	}

	if stored.Type != "" && stored.Type != opts.Type {
		fmt.Printf("⚠ 来源类型与元数据记录不一致（元数据: %s，本次: %s），已按本次指定的类型更新\n", stored.Type, opts.Type)
	}
	if stored.SourceURL != "" && stored.SourceURL != opts.Source {
		fmt.Printf("⚠ 来源地址与元数据记录不一致，已按本次指定的地址更新\n")
	}

	return Run(opts)
}

// SourceInfo 从已有 Collection 的 Markdown 元数据中读回的来源信息
type SourceInfo struct {
	Type      string   // 来源类型: swagger / grpc / proto
	SourceURL string   // 来源地址
	ProtoRoot []string // proto import 根目录
	Service   string   // Consul 服务名称
}

// LoadSourceInfo 读取 Collection 元数据中记录的来源信息
//
// 元数据在 `lion generate` 首次生成时写入每个分组文件的头部；旧版本生成的文件
// 可能缺少 source_url / proto_root / source_type，此时对应字段为空。
func LoadSourceInfo(name string) (*SourceInfo, error) {
	coll, err := collection.Load(name)
	if err != nil {
		return nil, err
	}

	// `> source:` 元数据历史上只记录 swagger / grpc，无法区分 proto；
	// 解析时优先使用 `> source_type:`，缺失则退化为 `> source:` 的值。
	return &SourceInfo{
		Type:      coll.Source,
		SourceURL: coll.SourceURL,
		ProtoRoot: coll.ProtoRoot,
		Service:   coll.Service,
	}, nil
}
