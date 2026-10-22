package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"lion/internal/collection"
	"lion/internal/config"
	"lion/internal/generate"
	"lion/internal/model"
	"lion/internal/sender"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "lion",
		Short: "lion - gRPC/HTTP 请求命令行工具",
		Long:  "lion 是一个用于发送 gRPC / HTTP 请求的命令行工具，面向微服务调试与接口联调场景。",
	}

	// 添加子命令
	rootCmd.AddCommand(
		newGenerateCmd(),
		newCollectionsCmd(),
		newSendCmd(),
		newInitCmd(),
		newConfigCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// newInitCmd 初始化命令
func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "初始化配置目录和默认配置文件",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.InitConfigDir(); err != nil {
				return err
			}
			fmt.Println("✅ 初始化完成，配置目录: ~/.lion/")
			return nil
		},
	}
}

// newGenerateCmd 生成接口集合命令
func newGenerateCmd() *cobra.Command {
	var (
		name      string
		typ       string
		protoRoot []string
		force     bool
		service   string
		update    bool
	)

	cmd := &cobra.Command{
		Use:   "generate [source]",
		Short: "生成接口集合",
		Long: `根据 Swagger / gRPC 反射 / Proto 文件生成接口集合。

使用 --update 时，来源类型、来源地址、proto 根目录与 service 会从已有 Collection
的 Markdown 元数据中自动读取，因此只需指定名称即可重新拉取接口列表：

  lion generate --name wms --update

接口列表会重新从来源拉取，已存在的接口 ID 保留用户编辑过的 JSON 值与脚本配置。`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source := ""
			if len(args) > 0 {
				source = args[0]
			}

			if !update && source == "" {
				return fmt.Errorf("缺少来源地址参数，或使用 --update 从已有 Collection 的元数据中读取")
			}
			if !update && typ == "" {
				return fmt.Errorf("缺少 --type 参数（swagger / grpc / proto），或使用 --update 从已有 Collection 的元数据中读取")
			}
			if update && force {
				return fmt.Errorf("--update 为合并模式（保留用户编辑的 JSON 值与脚本），不能与 --force（整体覆盖）同时使用")
			}

			opts := &generate.Options{
				Name:      name,
				Type:      typ,
				Source:    source,
				ProtoRoot: protoRoot,
				Force:     force,
				Service:   service,
			}

			if update {
				if err := generate.Update(opts); err != nil {
					return err
				}
				fmt.Printf("✅ Collection %q 更新完成\n", name)
				return nil
			}

			if err := generate.Run(opts); err != nil {
				return err
			}

			fmt.Printf("✅ Collection %q 生成完成\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Collection 名称（必填）")
	cmd.Flags().StringVar(&typ, "type", "", "来源类型: swagger / grpc / proto（首次生成必填，--update 时可省略）")
	cmd.Flags().StringArrayVar(&protoRoot, "proto_root", nil, "proto import 根目录（可多次指定）")
	cmd.Flags().BoolVar(&force, "force", false, "强制整体覆盖：重新拉取接口列表并覆盖文件，不保留用户编辑的 JSON 值与脚本")
	cmd.Flags().BoolVar(&update, "update", false, "更新已有 Collection：来源信息从元数据读取，无需重复指定 --type 和 source")
	cmd.Flags().StringVar(&service, "service", "", "Consul 服务名称（用于 consul 寻址模式）")
	_ = cmd.MarkFlagRequired("name")

	return cmd
}

// newCollectionsCmd 管理接口分组命令
func newCollectionsCmd() *cobra.Command {
	var (
		list   bool
		name   string
		delete bool
		export bool
	)

	cmd := &cobra.Command{
		Use:   "collections",
		Short: "管理接口分组",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 列出所有 Collection
			if list && name == "" {
				collections, err := collection.List()
				if err != nil {
					return err
				}
				if len(collections) == 0 {
					fmt.Println("暂无 Collection")
					return nil
				}
				for _, c := range collections {
					fmt.Printf("  📁 %s\n", c)
				}
				return nil
			}

			// 列出指定 Collection 的接口
			if list && name != "" {
				apis, err := collection.ListAPIs(name)
				if err != nil {
					return err
				}
				if len(apis) == 0 {
					fmt.Printf("Collection %q 暂无接口\n", name)
					return nil
				}
				fmt.Printf("Collection: %s\n", name)
				for _, api := range apis {
					if isHTTPMethod(api.Method) {
						fmt.Printf("  🔗 %-30s %s %s\n", api.ID, api.Method, api.Path)
					} else {
						fmt.Printf("  🔗 %-30s %s\n", api.ID, api.Path)
					}
				}
				return nil
			}

			// 删除 Collection
			if delete && name != "" {
				if err := collection.Delete(name); err != nil {
					return err
				}
				fmt.Printf("✅ Collection %q 已删除\n", name)
				return nil
			}

			// 导出 Collection
			if export && name != "" {
				data, err := collection.Export(name)
				if err != nil {
					return err
				}
				fmt.Println(data)
				return nil
			}

			return cmd.Help()
		},
	}

	// details 子命令
	detailsCmd := &cobra.Command{
		Use:   "details <api-id>",
		Short: "查看接口详情",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiID := args[0]
			_, _, api, err := collection.GetAPI(apiID)
			if err != nil {
				return err
			}

			fmt.Printf("接口 ID:  %s\n", api.ID)
			fmt.Printf("方法:     %s\n", api.Method)
			fmt.Printf("路径:     %s\n", api.Path)
			if len(api.Scripts) > 0 {
				for i, s := range api.Scripts {
					if i == 0 {
						fmt.Printf("脚本:     %s\n", s)
					} else {
						fmt.Printf("          %s\n", s)
					}
				}
			}
			if api.Description != "" {
				fmt.Printf("描述:     %s\n", api.Description)
			}
			fmt.Printf("\n请求体:\n")
			if api.BodyJSON != "" {
				// 尝试格式化 JSON
				var v interface{}
				if err := json.Unmarshal([]byte(api.BodyJSON), &v); err == nil {
					data, _ := json.MarshalIndent(v, "", "  ")
					fmt.Println(string(data))
				} else {
					fmt.Println(api.BodyJSON)
				}
			} else {
				fmt.Println("  (空)")
			}

			return nil
		},
	}
	cmd.AddCommand(detailsCmd)

	cmd.Flags().BoolVar(&list, "list", false, "列出 Collection 或接口")
	cmd.Flags().StringVar(&name, "name", "", "Collection 名称")
	cmd.Flags().BoolVar(&delete, "delete", false, "删除 Collection")
	cmd.Flags().BoolVar(&export, "export", false, "导出 Collection 为 JSON")

	return cmd
}

// newSendCmd 发送请求命令
func newSendCmd() *cobra.Command {
	var (
		env       string
		addr      string
		params    []string
		headers   []string
		output    string
		verbose   bool
		plaintext string
		copy      bool
	)

	cmd := &cobra.Command{
		Use:          "send <api-id>",
		Short:        "发送请求",
		SilenceUsage: true, // 请求失败时不打印 Usage
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiID := args[0]

			// 加载全局配置
			cfg, err := config.LoadGlobalConfig()
			if err != nil {
				return fmt.Errorf("加载配置失败: %w", err)
			}

			// 查找接口（同时获取 Group，用于分组级 hooks）
			coll, group, api, err := collection.GetAPI(apiID)
			if err != nil {
				return err
			}

			// 合并 Collection 级配置（~/.lion/<collection>.yaml），覆盖 global.yaml
			cfg, err = config.ApplyCollectionConfig(cfg, coll.Name)
			if err != nil {
				return fmt.Errorf("加载 Collection %q 配置失败: %w", coll.Name, err)
			}

			// 覆盖输出格式
			if output != "" {
				cfg.Global.Output = output
			}

			// 解析 --param 参数
			overrides := make(map[string]string)
			for _, p := range params {
				parts := strings.SplitN(p, "=", 2)
				if len(parts) == 2 {
					overrides[parts[0]] = parts[1]
				}
			}

			// 解析 --header 参数
			headerMap := make(map[string]string)
			for _, h := range headers {
				idx := strings.Index(h, "=")
				if idx > 0 {
					headerMap[h[:idx]] = h[idx+1:]
				} else {
					// 也支持冒号分隔
					idx = strings.Index(h, ":")
					if idx > 0 {
						headerMap[h[:idx]] = h[idx+1:]
					}
				}
			}

			// 创建发送器（传入 Collection 的 Consul 服务名称、headers、是否 verbose）
			s, err := sender.NewSender(cfg, env, addr, coll.Service, plaintext, verbose, headerMap)
			if err != nil {
				return fmt.Errorf("创建发送器失败: %w", err)
			}

			// --copy 模式：生成等效命令，不发送请求
			if copy {
				if isGRPC(api) {
					cmd, err := s.CopyGRPC(api, overrides)
					if err != nil {
						return err
					}
					fmt.Println(cmd)
				} else {
					cmd, err := s.CopyHTTP(api, overrides)
					if err != nil {
						return err
					}
					fmt.Println(cmd)
				}
				return nil
			}

			// 判断是 HTTP 还是 gRPC
			if isGRPC(api) {
				return s.SendGRPC(api, group, overrides)
			}
			return s.SendHTTP(api, group, overrides)
		},
	}

	cmd.Flags().StringVarP(&env, "env", "e", "", "目标环境")
	cmd.Flags().StringVarP(&addr, "addr", "a", "", "直连地址（覆盖配置中的 host）")
	cmd.Flags().StringArrayVar(&params, "param", nil, "覆盖参数 (key=value)")
	cmd.Flags().StringArrayVarP(&headers, "header", "H", nil, "自定义请求头 / gRPC metadata（覆盖配置中的固定值，可多次指定）")
	cmd.Flags().StringVarP(&output, "output", "o", "", "输出格式: pretty / json / raw")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "显示完整请求信息与脚本执行详情")
	cmd.Flags().StringVarP(&plaintext, "plaintext", "P", "", "完整请求体 JSON（覆盖接口定义的请求体）")
	cmd.Flags().BoolVarP(&copy, "copy", "c", false, "生成等效 curl/grpcurl 命令（不发送请求）")

	return cmd
}

// isGRPC 判断接口是否为 gRPC
func isGRPC(api *model.API) bool {
	// gRPC 接口通常没有 HTTP Method，或 Method 为 gRPC
	return api.Method == "" || api.Method == "gRPC" ||
		!isHTTPMethod(api.Method)
}

// newConfigCmd 配置管理命令
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "管理 Collection 配置",
	}

	// show 子命令
	showCmd := &cobra.Command{
		Use:   "show <collection_name>",
		Short: "显示 Collection 配置信息",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			result, err := collection.Show(name)
			if err != nil {
				return err
			}
			fmt.Print(collection.FormatShowResult(result))
			return nil
		},
	}

	// init 子命令
	initCmd := &cobra.Command{
		Use:   "init <collection_name>",
		Short: "初始化 Collection 配置",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := collection.Init(name); err != nil {
				return err
			}
			fmt.Printf("✅ Collection %q 初始化完成\n", name)
			return nil
		},
	}

	// check 子命令
	checkCmd := &cobra.Command{
		Use:   "check <collection_name>",
		Short: "检查 Collection 配置是否正确",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			result, err := collection.Check(name)
			if err != nil {
				return err
			}
			fmt.Print(collection.FormatCheckResult(name, result))
			if !result.IsValid() {
				return fmt.Errorf("配置检查失败")
			}
			return nil
		},
	}

	cmd.AddCommand(showCmd, initCmd, checkCmd)
	return cmd
}

// isHTTPMethod 判断是否为 HTTP 方法
func isHTTPMethod(method string) bool {
	switch strings.ToUpper(method) {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD":
		return true
	}
	return false
}
