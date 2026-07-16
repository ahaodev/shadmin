package conf

import (
	"errors"
	"fmt"
	"os"
	"shadmin/pkg"
	"strings"

	"github.com/spf13/viper"
)

const (
	AppEnvDev      = "dev"
	AppEnvProd     = "prod"
	AppEnvTest     = "test"
	DbTypeMysql    = "mysql"
	DbTypeSqlite   = "sqlite"
	DbTypePgSql    = "postgres"
	CacheTypeMem   = "mem"
	CacheTypeRedis = "redis"
	StoreTypeMinio = "minio"
	StoreTypeDisk  = "disk"
)

type Env struct {
	AppEnv         string `mapstructure:"APP_ENV"`
	ContextTimeout int    `mapstructure:"CONTEXT_TIMEOUT"`
	Port           string `mapstructure:"PORT"`

	// Database configuration
	DBType string `mapstructure:"DB_TYPE"` // "sqlite" or "postgres"
	DBDSN  string `mapstructure:"DB_DSN"`  // PostgreSQL connection string
	// 令牌配置
	AccessTokenExpiryMinute  int    `mapstructure:"ACCESS_TOKEN_EXPIRY_MINUTE"`
	RefreshTokenExpiryMinute int    `mapstructure:"REFRESH_TOKEN_EXPIRY_MINUTE"`
	AccessTokenSecret        string `mapstructure:"ACCESS_TOKEN_SECRET"`
	RefreshTokenSecret       string `mapstructure:"REFRESH_TOKEN_SECRET"`

	// 管理员配置
	AdminUsername string `mapstructure:"ADMIN_USERNAME"`
	AdminPassword string `mapstructure:"ADMIN_PASSWORD"`
	AdminEmail    string `mapstructure:"ADMIN_EMAIL"`

	// 存储配置
	StorageType     string `mapstructure:"STORAGE_TYPE"`      // "disk" 或 "minio"
	StorageBasePath string `mapstructure:"STORAGE_BASE_PATH"` // 本地存储基础路径

	// S3/MinIO 配置
	S3Address   string `mapstructure:"S3_ADDRESS"`
	S3AccessKey string `mapstructure:"S3_ACCESS_KEY"`
	S3SecretKey string `mapstructure:"S3_SECRET_KEY"`
	S3Bucket    string `mapstructure:"S3_BUCKET"`
	S3Token     string `mapstructure:"S3_TOKEN"`

	// 缓存配置（mem=内存，redis=Redis）
	CacheType     string `mapstructure:"CACHE_TYPE"`
	RedisAddress  string `mapstructure:"REDIS_ADDRESS"`
	RedisUsername string `mapstructure:"REDIS_USERNAME"`
	RedisPassword string `mapstructure:"REDIS_PASSWORD"`
	RedisDB       int    `mapstructure:"REDIS_DB"`

	// 社交登录配置
	SocialBaseURL       string `mapstructure:"SOCIAL_BASE_URL"`       // 后端外部可达地址，用于拼接 OAuth 回调 URL
	SocialRedirectURL   string `mapstructure:"SOCIAL_REDIRECT_URL"`   // 前端回调地址，登录成功后带 token 重定向到这里
	SocialSessionSecret string `mapstructure:"SOCIAL_SESSION_SECRET"` // gothic session cookie 签名密钥
	GoogleClientID      string `mapstructure:"GOOGLE_CLIENT_ID"`
	GoogleClientSecret  string `mapstructure:"GOOGLE_CLIENT_SECRET"`
	GitHubClientID      string `mapstructure:"GITHUB_CLIENT_ID"`
	GitHubClientSecret  string `mapstructure:"GITHUB_CLIENT_SECRET"`
}

// CacheTypeValue 返回规范化后的缓存类型。
func (e *Env) CacheTypeValue() string {
	return strings.ToLower(strings.TrimSpace(e.CacheType))
}

// RedisEnabled 返回是否启用 Redis 缓存后端。
func (e *Env) RedisEnabled() bool {
	return e.CacheTypeValue() == CacheTypeRedis && strings.TrimSpace(e.RedisAddress) != ""
}

func setDefaults() {
	defaults := map[string]interface{}{
		// 基础配置
		"APP_ENV":         AppEnvDev,
		"CONTEXT_TIMEOUT": 60,
		"PORT":            ":55667",

		// 数据库配置
		"DB_TYPE": DbTypeSqlite,
		"DB_DSN":  "",

		// 令牌配置
		"ACCESS_TOKEN_EXPIRY_MINUTE":  180,
		"REFRESH_TOKEN_EXPIRY_MINUTE": 1800,
		"ACCESS_TOKEN_SECRET":         "default-access-secret",
		"REFRESH_TOKEN_SECRET":        "default-refresh-secret",

		// 管理员配置
		"ADMIN_USERNAME": "admin",
		"ADMIN_PASSWORD": "123",
		"ADMIN_EMAIL":    "admin@gmail.com",

		// 存储配置
		"STORAGE_TYPE":      StoreTypeDisk,
		"STORAGE_BASE_PATH": ".uploads",

		// S3/MinIO 配置
		"S3_ADDRESS":    "192.168.8.6:9000",
		"S3_ACCESS_KEY": "IjJm2N3ZZTYjt8C9WkJf",
		"S3_SECRET_KEY": "eIuV0i4ChbLqx54g9rhsZDRTC2LE1xEcnIAnAw1C",
		"S3_BUCKET":     "shadmin",
		"S3_TOKEN":      "",

		// 缓存配置
		"CACHE_TYPE":     CacheTypeMem,
		"REDIS_ADDRESS":  "",
		"REDIS_USERNAME": "",
		"REDIS_PASSWORD": "",
		"REDIS_DB":       0,

		// 社交登录配置
		"SOCIAL_BASE_URL":       "http://localhost:55667",
		"SOCIAL_REDIRECT_URL":   "http://localhost:5173/oauth-callback",
		"SOCIAL_SESSION_SECRET": "shadmin-social-session-secret-change-me",
		"GOOGLE_CLIENT_ID":      "",
		"GOOGLE_CLIENT_SECRET":  "",
		"GITHUB_CLIENT_ID":      "",
		"GITHUB_CLIENT_SECRET":  "",
	}
	// 绑定环境变量
	viper.AutomaticEnv()
	for key, value := range defaults {
		viper.SetDefault(key, value)
		viper.BindEnv(key)
	}
}

func generateEnvFile() error {
	configSections := []struct {
		title string
		keys  []string
	}{
		{
			title: "# 基础配置",
			keys:  []string{"APP_ENV", "CONTEXT_TIMEOUT", "PORT"},
		},
		{
			title: "# 数据库配置",
			keys:  []string{"DB_TYPE", "DB_DSN"},
		},
		{
			title: "# 令牌配置",
			keys:  []string{"ACCESS_TOKEN_EXPIRY_MINUTE", "REFRESH_TOKEN_EXPIRY_MINUTE", "ACCESS_TOKEN_SECRET", "REFRESH_TOKEN_SECRET"},
		},
		{
			title: "# 管理员配置",
			keys:  []string{"ADMIN_USERNAME", "ADMIN_PASSWORD", "ADMIN_EMAIL"},
		},
		{
			title: "# 存储配置",
			keys:  []string{"STORAGE_TYPE", "STORAGE_BASE_PATH"},
		},
		{
			title: "# S3/MinIO 配置",
			keys:  []string{"S3_ADDRESS", "S3_ACCESS_KEY", "S3_SECRET_KEY", "S3_BUCKET", "S3_TOKEN"},
		},
		{
			title: "# 缓存配置（mem=内存，redis=Redis）",
			keys:  []string{"CACHE_TYPE", "REDIS_ADDRESS", "REDIS_USERNAME", "REDIS_PASSWORD", "REDIS_DB"},
		},
		{
			title: "# 社交登录配置",
			keys:  []string{"SOCIAL_BASE_URL", "SOCIAL_REDIRECT_URL", "SOCIAL_SESSION_SECRET", "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GITHUB_CLIENT_ID", "GITHUB_CLIENT_SECRET"},
		},
	}

	var content strings.Builder
	for _, section := range configSections {
		content.WriteString(section.title + "\n")
		for _, key := range section.keys {
			fmt.Fprintf(&content, "%s=%v\n", key, viper.Get(key))
		}
		content.WriteString("\n")
	}

	return os.WriteFile(".env", []byte(content.String()), 0644)
}

func loadConfig() (*Env, error) {
	viper.SetConfigFile(".env")
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var env Env
	if err := viper.Unmarshal(&env); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	return &env, nil
}

func (e *Env) validate() error {
	var errs []string

	if e.AppEnv != AppEnvDev && e.AppEnv != AppEnvProd && e.AppEnv != AppEnvTest {
		errs = append(errs, "APP_ENV必须是 dev、prod 或 test")
	}

	if e.ContextTimeout <= 0 {
		errs = append(errs, "CONTEXT_TIMEOUT必须大于0")
	}

	if e.Port == "" {
		errs = append(errs, "PORT不能为空")
	}

	if e.DBType != DbTypeSqlite && e.DBType != DbTypePgSql && e.DBType != DbTypeMysql {
		errs = append(errs, "DB_TYPE必须是sqlite、postgres或mysql")
	}

	if (e.DBType == DbTypePgSql || e.DBType == DbTypeMysql) && e.DBDSN == "" {
		errs = append(errs, "使用PostgreSQL或MySQL时必须提供DB_DSN")
	}

	if e.AccessTokenExpiryMinute <= 0 {
		errs = append(errs, "ACCESS_TOKEN_EXPIRY_MINUTE必须大于0")
	}

	if e.RefreshTokenExpiryMinute <= 0 {
		errs = append(errs, "REFRESH_TOKEN_EXPIRY_MINUTE必须大于0")
	}

	if len(e.AccessTokenSecret) < 16 {
		errs = append(errs, "ACCESS_TOKEN_SECRET长度不能少于16位")
	}

	if len(e.RefreshTokenSecret) < 16 {
		errs = append(errs, "REFRESH_TOKEN_SECRET长度不能少于16位")
	}

	if e.AdminUsername == "" {
		errs = append(errs, "ADMIN_USERNAME不能为空")
	}

	if len(e.AdminPassword) < 3 {
		errs = append(errs, "ADMIN_PASSWORD长度不能少于3位")
	}

	if e.StorageType != StoreTypeDisk && e.StorageType != StoreTypeMinio {
		errs = append(errs, "STORAGE_TYPE必须是disk或minio")
	}

	if e.StorageType == StoreTypeDisk && e.StorageBasePath == "" {
		errs = append(errs, "使用本地存储时必须提供STORAGE_BASE_PATH")
	}

	if e.StorageType == StoreTypeMinio {
		if e.S3Address == "" {
			errs = append(errs, "使用MinIO时S3_ADDRESS不能为空")
		}
		if e.S3AccessKey == "" {
			errs = append(errs, "使用MinIO时S3_ACCESS_KEY不能为空")
		}
		if e.S3SecretKey == "" {
			errs = append(errs, "使用MinIO时S3_SECRET_KEY不能为空")
		}
		if e.S3Bucket == "" {
			errs = append(errs, "使用MinIO时S3_BUCKET不能为空")
		}
	}

	cacheType := e.CacheTypeValue()
	if cacheType != CacheTypeMem && cacheType != CacheTypeRedis {
		errs = append(errs, "CACHE_TYPE必须是mem或redis")
	}
	if cacheType == CacheTypeRedis && strings.TrimSpace(e.RedisAddress) == "" {
		errs = append(errs, "CACHE_TYPE=redis时REDIS_ADDRESS不能为空")
	}
	if cacheType == CacheTypeRedis && e.RedisDB < 0 {
		errs = append(errs, "REDIS_DB不能小于0")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}

func NewEnv() *Env {
	setDefaults()

	if _, err := os.Stat(".env"); err != nil {
		pkg.Log.Println("没有找到 .env 文件，使用默认配置并写入.env")
		if err := generateEnvFile(); err != nil {
			pkg.Log.Error("写入.env文件失败: " + err.Error())
			return &Env{}
		}
	}

	env, err := loadConfig()
	if err != nil {
		pkg.Log.Error(err.Error())
		return &Env{}
	}

	if err := env.validate(); err != nil {
		pkg.Log.Error("配置验证失败: " + err.Error())
	}

	if env.AppEnv == AppEnvDev {
		pkg.Log.Println("The App is running in development env")
	}

	return env
}
