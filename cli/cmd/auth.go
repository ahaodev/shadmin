package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"shadmin-cli/internal/client"
	"shadmin-cli/internal/clierr"
	"shadmin-cli/internal/config"
	"shadmin-cli/internal/output"
)

var (
	loginUsername      string
	loginPasswordStdin bool
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to Shadmin and cache tokens locally",
	RunE: runE(func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		if flagServer != "" {
			cfg.ServerURL = flagServer
		}
		if cfg.ServerURL == "" {
			return clierr.New(clierr.ExitUsage, "server url required: pass --server URL or set SHADMIN_SERVER")
		}

		username := loginUsername
		if username == "" {
			fmt.Fprint(os.Stderr, "Username: ")
			line, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				return clierr.Wrap(clierr.ExitGeneric, err, "read username")
			}
			username = strings.TrimSpace(line)
		}
		if username == "" {
			return clierr.New(clierr.ExitUsage, "username required")
		}

		var password string
		if loginPasswordStdin {
			b, err := io.ReadAll(os.Stdin)
			if err != nil {
				return clierr.Wrap(clierr.ExitGeneric, err, "read password from stdin")
			}
			password = strings.TrimRight(string(b), "\r\n")
		} else {
			fmt.Fprint(os.Stderr, "Password: ")
			pw, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return clierr.Wrap(clierr.ExitGeneric, err, "read password")
			}
			password = string(pw)
		}
		if password == "" {
			return clierr.New(clierr.ExitUsage, "password required")
		}

		cli := client.NewUnauth(cfg.ServerURL)
		ctx, cancel := context.WithTimeout(context.Background(), 30*sec())
		defer cancel()
		access, refresh, err := cli.Login(ctx, username, password)
		if err != nil {
			return err
		}

		cfg.Username = username
		cfg.AccessToken = access
		cfg.RefreshToken = refresh
		if err := config.Save(cfg); err != nil {
			return err
		}

		out := output.New(outputFormat())
		return out.JSON(map[string]any{
			"status":    "ok",
			"username":  username,
			"server":    cfg.ServerURL,
			"config":    mustConfigPath(),
			"logged_in": true,
		})
	}),
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out and clear local tokens",
	RunE: runE(func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		if cfg.AccessToken != "" && cfg.ServerURL != "" {
			cli, err := client.New(cfg)
			if err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 15*sec())
				defer cancel()
				_ = cli.Logout(ctx) // 本地清理优先，忽略服务端登出错误
			}
		}
		if err := config.Clear(); err != nil {
			return err
		}
		out := output.New(outputFormat())
		return out.JSON(map[string]any{"status": "ok", "logged_out": true})
	}),
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current logged-in user profile",
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
		ctx, cancel := context.WithTimeout(context.Background(), 15*sec())
		defer cancel()
		raw, err := cli.Profile(ctx)
		if err != nil {
			return err
		}
		if flagPretty {
			var m map[string]any
			_ = json.Unmarshal(raw, &m)
			fmt.Printf("Server:   %s\n", cfg.ServerURL)
			fmt.Printf("Username: %s\n", cfg.Username)
			for _, k := range []string{"id", "name", "email", "phone", "status"} {
				if v, ok := m[k]; ok {
					fmt.Printf("%-9s %v\n", capitalize(k)+":", v)
				}
			}
			return nil
		}
		_, err = os.Stdout.Write(append(raw, '\n'))
		return err
	}),
}

func mustConfigPath() string {
	p, _ := config.Path()
	return p
}

// capitalize 将 ASCII 字符串首字母大写，取代已废弃的 strings.Title
func capitalize(s string) string {
	if s == "" {
		return s
	}
	b := []byte(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 'a' - 'A'
	}
	return string(b)
}

func init() {
	loginCmd.Flags().StringVarP(&loginUsername, "username", "u", "", "username (prompted if empty)")
	loginCmd.Flags().BoolVar(&loginPasswordStdin, "password-stdin", false, "read password from stdin (non-interactive)")

	rootCmd.AddCommand(loginCmd, logoutCmd, whoamiCmd)
}
