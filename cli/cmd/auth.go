package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"shadmin-cli/internal/client"
	"shadmin-cli/internal/clierr"
	"shadmin-cli/internal/config"
	"shadmin-cli/internal/output"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to Shadmin with device authorization flow",
	RunE: runE(func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfigForLogin()
		if err != nil {
			return err
		}
		if cfg.ServerURL == "" {
			return clierr.New(clierr.ExitUsage, "server url required: pass --server URL or set SHADMIN_SERVER")
		}

		cli := client.NewUnauth(cfg.ServerURL)
		ctx, cancel := context.WithTimeout(context.Background(), 30*sec())
		defer cancel()

		deviceCode, err := cli.RequestDeviceCode(ctx, "shadmin-cli", "Shadmin CLI")
		if err != nil {
			return err
		}
		cancel()

		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(deviceCode.ExpiresIn)*sec()+15*sec())
		defer cancel()

		verificationURI := resolveVerificationURI(cfg.ServerURL, deviceCode.VerificationURI)
		fmt.Fprintf(os.Stderr, "Open this URL in your browser:\n\n  %s\n\n", verificationURI)
		fmt.Fprintf(os.Stderr, "Then enter this code:\n\n  %s\n\n", deviceCode.UserCode)
		fmt.Fprintln(os.Stderr, "Waiting for authorization...")

		token, err := pollDeviceAuthorization(ctx, cli, deviceCode)
		if err != nil {
			return err
		}

		cfg.AccessToken = token.AccessToken
		cfg.RefreshToken = token.RefreshToken
		if err := config.Save(cfg); err != nil {
			return err
		}

		username := ""
		profileCtx, profileCancel := context.WithTimeout(context.Background(), 15*sec())
		defer profileCancel()
		if profile, err := fetchLoginProfile(profileCtx, cfg); err == nil {
			username = profile
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

func resolveVerificationURI(serverURL, verificationURI string) string {
	uri := strings.TrimSpace(verificationURI)
	if uri == "" {
		uri = "/auth/device"
	}
	if parsed, err := url.Parse(uri); err == nil && parsed.IsAbs() {
		return uri
	}
	baseURL := strings.TrimRight(serverURL, "/")
	if strings.HasPrefix(uri, "/") {
		return baseURL + uri
	}
	return baseURL + "/" + uri
}

func pollDeviceAuthorization(ctx context.Context, cli *client.Client, deviceCode *client.DeviceCodeResponse) (*client.LoginTokenResponse, error) {
	interval := deviceCode.Interval
	if interval <= 0 {
		interval = 5
	}
	timer := time.NewTimer(time.Duration(interval) * sec())
	defer timer.Stop()

	deadline := time.NewTimer(time.Duration(deviceCode.ExpiresIn) * sec())
	defer deadline.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, clierr.Wrap(clierr.ExitGeneric, ctx.Err(), "device authorization timed out")
		case <-deadline.C:
			return nil, clierr.New(clierr.ExitGeneric, "device authorization expired; run login again")
		case <-timer.C:
			token, err := cli.PollDeviceToken(ctx, "shadmin-cli", deviceCode.DeviceCode)
			if err == nil {
				return token, nil
			}
			msg := err.Error()
			switch {
			case strings.Contains(msg, "authorization_pending"):
				timer.Reset(time.Duration(interval) * sec())
			case strings.Contains(msg, "slow_down"):
				interval += 5
				timer.Reset(time.Duration(interval) * sec())
			case strings.Contains(msg, "expired_token"):
				return nil, clierr.New(clierr.ExitGeneric, "device authorization expired; run login again")
			case strings.Contains(msg, "access_denied"):
				return nil, clierr.New(clierr.ExitUnauth, "device authorization denied")
			default:
				return nil, err
			}
		}
	}
}

func fetchLoginProfile(ctx context.Context, cfg *config.Config) (string, error) {
	cli, err := client.New(cfg)
	if err != nil {
		return "", err
	}
	raw, err := cli.Profile(ctx)
	if err != nil {
		return "", err
	}
	var profile map[string]any
	if err := json.Unmarshal(raw, &profile); err != nil {
		return "", err
	}
	for _, key := range []string{"username", "name", "email", "id"} {
		if value, ok := profile[key].(string); ok && value != "" {
			return value, nil
		}
	}
	return "", nil
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
	rootCmd.AddCommand(loginCmd, logoutCmd, whoamiCmd)
}
