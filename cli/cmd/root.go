package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"shadmin-cli/internal/clierr"
	"shadmin-cli/internal/config"
	"shadmin-cli/internal/output"
)

var (
	versionInfo = struct {
		Version string
		Commit  string
		Date    string
	}{"dev", "unknown", "unknown"}

	flagPretty bool
	flagServer string
)

var rootCmd = &cobra.Command{
	Use:           "shadmin-cli",
	Short:         "CLI for Shadmin backend, designed for external AI agents",
	Long:          "shadmin-cli is a thin wrapper over Shadmin REST API. It inherits the logged-in user's RBAC permissions and outputs JSON by default for AI agent consumption.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute 注入版本并执行根命令
func Execute(version, commit, date string) {
	versionInfo.Version = version
	versionInfo.Commit = commit
	versionInfo.Date = date
	rootCmd.Version = fmt.Sprintf("%s (commit %s, built %s)", version, commit, date)

	if err := rootCmd.Execute(); err != nil {
		clierr.Fatal(err)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&flagPretty, "pretty", false, "output human-readable table")
	rootCmd.PersistentFlags().StringVar(&flagServer, "server", "", "Shadmin server URL (overrides config and SHADMIN_SERVER)")
}

// outputFormat 根据 flag 决定输出格式
func outputFormat() output.Format {
	if flagPretty {
		return output.FormatPretty
	}
	return output.FormatJSON
}

// loadConfig 加载配置并应用本次命令的 --server 覆盖
func loadConfig() (*config.Config, error) {
	return loadConfigWithServerPersistence(false)
}

// loadConfigForLogin persists --server because login is the CLI setup command.
func loadConfigForLogin() (*config.Config, error) {
	return loadConfigWithServerPersistence(true)
}

func loadConfigWithServerPersistence(persistFlagServer bool) (*config.Config, error) {
	c, err := config.Load()
	if err != nil {
		return nil, err
	}
	config.ApplyServerOverride(c, flagServer, persistFlagServer)
	return c, nil
}

// requireAuth 确保已登录
func requireAuth(c *config.Config) error {
	if c.ServerURL == "" {
		return clierr.New(clierr.ExitUnauth, "server url not set; run 'shadmin-cli login --server URL' first")
	}
	if c.AccessToken == "" {
		return clierr.New(clierr.ExitUnauth, "not logged in; run 'shadmin-cli login' first")
	}
	return nil
}

// runE 包装 cobra.RunE，统一处理错误退出码
func runE(fn func(cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		return fn(cmd, args)
	}
}
