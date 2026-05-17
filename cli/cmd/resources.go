package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"shadmin-cli/internal/client"
	"shadmin-cli/internal/clierr"
	"shadmin-cli/internal/output"
)

var (
	listPage     int
	listPageSize int
	listKeyword  string
)

func addListFlags(c *cobra.Command) {
	c.Flags().IntVar(&listPage, "page", 0, "page number (1-based)")
	c.Flags().IntVar(&listPageSize, "page-size", 0, "page size")
	c.Flags().StringVarP(&listKeyword, "keyword", "k", "", "keyword filter")
}

func buildList() client.ListParams {
	return client.ListParams{Page: listPage, PageSize: listPageSize, Keyword: listKeyword}
}

// newReadonlyGroup 构造 "parent" + "parent list / parent get <id>" 常规只读命令
func newReadonlyGroup(use, short, basePath string) *cobra.Command {
	parent := &cobra.Command{Use: use, Short: short}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: short + " - list",
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if err := requireAuth(cfg); err != nil {
				return err
			}
			cli, err := client.New(cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*sec())
			defer cancel()
			raw, err := cli.GetJSON(ctx, basePath, buildList())
			if err != nil {
				return err
			}
			return writeRaw(raw, renderListPretty)
		}),
	}
	addListFlags(listCmd)

	getCmd := &cobra.Command{
		Use:   "get <id>",
		Short: short + " - get by id",
		Args:  cobra.ExactArgs(1),
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if err := requireAuth(cfg); err != nil {
				return err
			}
			cli, err := client.New(cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*sec())
			defer cancel()
			raw, err := cli.GetJSON(ctx, basePath+"/"+args[0], client.ListParams{})
			if err != nil {
				return err
			}
			return writeRaw(raw, renderObjectPretty)
		}),
	}

	parent.AddCommand(listCmd, getCmd)
	return parent
}

// writeRaw 按照全局 format 输出
func writeRaw(raw json.RawMessage, pretty func(v any)) error {
	w := output.New(outputFormat())
	if w.Format == output.FormatJSON {
		return w.JSON(json.RawMessage(raw))
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return clierr.Wrap(clierr.ExitGeneric, err, "decode response for pretty print")
	}
	pretty(v)
	return nil
}

// renderListPretty 打印 list 响应。兼容 domain.PagedResult 或裸数组
func renderListPretty(v any) {
	switch d := v.(type) {
	case map[string]any:
		if list, ok := d["list"].([]any); ok {
			fmt.Printf("Total: %v  Page: %v  PageSize: %v  TotalPages: %v\n",
				d["total"], d["page"], d["page_size"], d["total_pages"])
			printObjects(list)
			return
		}
		printObjects([]any{d})
	case []any:
		printObjects(d)
	default:
		b, _ := json.MarshalIndent(v, "", "  ")
		fmt.Println(string(b))
	}
}

func renderObjectPretty(v any) {
	switch d := v.(type) {
	case map[string]any:
		printObjects([]any{d})
	default:
		b, _ := json.MarshalIndent(v, "", "  ")
		fmt.Println(string(b))
	}
}

// printObjects 对 map 切片选取少量关键字段按行打印；其他情况退化为 JSON
func printObjects(list []any) {
	if len(list) == 0 {
		fmt.Println("(empty)")
		return
	}
	preferred := []string{"id", "name", "username", "code", "path", "method", "title", "status"}
	headers := []string{}
	seen := map[string]bool{}
	for _, it := range list {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		for _, k := range preferred {
			if _, exists := m[k]; exists && !seen[k] {
				headers = append(headers, k)
				seen[k] = true
			}
		}
	}
	if len(headers) == 0 {
		b, _ := json.MarshalIndent(list, "", "  ")
		fmt.Println(string(b))
		return
	}
	// header
	for i, h := range headers {
		if i > 0 {
			fmt.Print("  ")
		}
		fmt.Printf("%-20s", h)
	}
	fmt.Println()
	for _, it := range list {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		for i, h := range headers {
			if i > 0 {
				fmt.Print("  ")
			}
			fmt.Printf("%-20v", m[h])
		}
		fmt.Println()
	}
}

func init() {
	rootCmd.AddCommand(newReadonlyGroup("users", "Manage users (read-only)", "/api/v1/system/user"))
	rootCmd.AddCommand(newReadonlyGroup("roles", "Manage roles (read-only)", "/api/v1/system/role"))

	// menus 有额外 tree 子命令
	menus := &cobra.Command{Use: "menus", Short: "Manage menus (read-only)"}
	menusList := &cobra.Command{
		Use:   "list",
		Short: "List menus",
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			return simpleGet(cmd, "/api/v1/system/menu", true)
		}),
	}
	addListFlags(menusList)
	menusGet := &cobra.Command{
		Use:   "get <id>",
		Short: "Get menu by id",
		Args:  cobra.ExactArgs(1),
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			return simpleGet(cmd, "/api/v1/system/menu/"+args[0], false)
		}),
	}
	menusTree := &cobra.Command{
		Use:   "tree",
		Short: "Get menu tree",
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			return simpleGet(cmd, "/api/v1/system/menu/tree", false)
		}),
	}
	menus.AddCommand(menusList, menusGet, menusTree)
	rootCmd.AddCommand(menus)

	// api-resources 只有 list
	apiRes := &cobra.Command{Use: "api-resources", Short: "API resources registered by the backend"}
	apiResList := &cobra.Command{
		Use:   "list",
		Short: "List api resources",
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			return simpleGet(cmd, "/api/v1/system/api-resources", true)
		}),
	}
	addListFlags(apiResList)
	apiRes.AddCommand(apiResList)
	rootCmd.AddCommand(apiRes)
}

// simpleGet 复用 GET 流程；isList 控制 pretty 渲染方式
func simpleGet(cmd *cobra.Command, path string, isList bool) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if err := requireAuth(cfg); err != nil {
		return err
	}
	cli, err := client.New(cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*sec())
	defer cancel()
	raw, err := cli.GetJSON(ctx, path, buildList())
	if err != nil {
		return err
	}
	if isList {
		return writeRaw(raw, renderListPretty)
	}
	return writeRaw(raw, renderObjectPretty)
}
