package casbin

import (
	"fmt"
	"log"
	"shadmin/ent"
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	redisadapter "github.com/casbin/redis-adapter/v2"
	_ "github.com/mattn/go-sqlite3"
)

// Config 控制 Casbin 后端选择。RedisAddr 非空 → 走 Redis 适配器；否则为纯内存模式。
type Config struct {
	RedisAddr     string
	RedisPassword string
	RedisDB       int
}

var (
	enforcer *casbin.Enforcer
	once     sync.Once
)

// Manager 权限管理器接口 - 简化后只保留核心方法
type Manager interface {
	// CheckPermission 核心权限检查 - middleware使用
	CheckPermission(userID, object, action string) (bool, error)

	// AddPolicy 策略管理 - sync服务使用
	AddPolicy(roleID, object, action string) (bool, error)
	RemovePolicy(roleID, object, action string) (bool, error)
	GetAllPolicies() [][]string

	// AddRoleForUser 角色管理 - sync服务使用
	AddRoleForUser(userID, roleID string) (bool, error)
	DeleteRoleForUser(userID, roleID string) (bool, error)
	GetRolesForUser(userID string) []string
	GetAllRoles() [][]string

	// SavePolicy 系统管理 - sync服务使用
	SavePolicy() error
	LoadPolicy() error
	GetEnforcer() *casbin.Enforcer
}

// CasManager 权限管理器实现
type CasManager struct {
	enforcer  *casbin.Enforcer
	entClient *ent.Client
	logger    Logger
}

// Logger 日志接口
type Logger interface {
	Log(action, message string)
}

// defaultLogger 默认日志实现
type defaultLogger struct{}

func (l *defaultLogger) Log(action, message string) {
	log.Printf("[%s] %s", action, message)
}

// 配置常量
const (
	ModelConf = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && (r.obj == p.obj || p.obj == "*" || keyMatch2(r.obj, p.obj) ) && (r.act == p.act || p.act == "*")
`
	// 资源类型前缀常量 - 用于对象标识（保留以备将来扩展）
	MenuResourcePrefix   = "menu:"
	ButtonResourcePrefix = "button:"
)

// NewCasManager 创建权限管理器实例
func NewCasManager(entClient *ent.Client, cfg Config) Manager {
	return NewCasManagerWithLogger(entClient, cfg, &defaultLogger{})
}

// NewCasManagerWithLogger 创建带自定义日志的权限管理器实例
func NewCasManagerWithLogger(entClient *ent.Client, cfg Config, logger Logger) Manager {
	var err error

	once.Do(func() {
		err = initializeCasbin(entClient, cfg)
	})

	if err != nil {
		panic(fmt.Errorf("failed to initialize casbin manager: %w", err))
	}

	return &CasManager{
		enforcer:  enforcer,
		entClient: entClient,
		logger:    logger,
	}
}

// initializeCasbin 初始化Casbin组件
// 启用 Redis 时使用 casbin-redis-adapter（自行管理 redigo 连接）；
// 否则使用无适配器的纯内存 enforcer，策略由 SyncService 从 PG 同步入内存。
func initializeCasbin(entClient *ent.Client, cfg Config) error {
	// 创建模型
	m, err := model.NewModelFromString(ModelConf)
	if err != nil {
		return fmt.Errorf("failed to create casbin model: %w", err)
	}

	var adapter persist.Adapter
	if cfg.RedisAddr != "" {
		adapterKey := "casbin_rules"
		if cfg.RedisDB != 0 {
			adapterKey = fmt.Sprintf("casbin_rules:%d", cfg.RedisDB)
		}
		options := []redisadapter.Option{
			redisadapter.WithNetwork("tcp"),
			redisadapter.WithAddress(cfg.RedisAddr),
			redisadapter.WithPassword(cfg.RedisPassword),
			redisadapter.WithKey(adapterKey),
		}
		adapter = redisadapter.NewAdpaterWithOption(options...)
		enforcer, err = casbin.NewEnforcer(m, adapter)
	} else {
		// 纯内存模式：无适配器；AddPolicy 仅作用于内存，SavePolicy 需短路。
		enforcer, err = casbin.NewEnforcer(m)
	}
	if err != nil {
		return fmt.Errorf("failed to create casbin enforcer: %w", err)
	}

	enforcer.EnableLog(true)
	enforcer.EnableAutoSave(true)

	// 仅在有适配器时从后端加载策略；内存模式由 SyncService 填充。
	if adapter != nil {
		if err = enforcer.LoadPolicy(); err != nil {
			return fmt.Errorf("failed to load casbin policy: %w", err)
		}
	}

	return nil
}

// GetEnforcer 获取 enforcer 实例
func (m *CasManager) GetEnforcer() *casbin.Enforcer {
	return m.enforcer
}

// ========== 核心权限检查 ==========

// CheckPermission 检查权限
func (m *CasManager) CheckPermission(userID, object, action string) (bool, error) {
	m.logger.Log("CheckPermission", fmt.Sprintf("user:%s, object:%s, action:%s", userID, object, action))

	// 获取用户的所有角色
	roles := m.GetRolesForUser(userID)
	if len(roles) == 0 {
		return false, nil
	}

	// 检查用户的任何角色是否有权限
	for _, roleID := range roles {
		hasPermission, err := m.enforcer.Enforce(roleID, object, action)
		if err != nil {
			return false, err
		}
		if hasPermission {
			return true, nil
		}
	}

	return false, nil
}

// ========== 策略管理 ==========

// AddPolicy 添加权限策略
func (m *CasManager) AddPolicy(roleID, object, action string) (bool, error) {
	m.logger.Log("AddPolicy", fmt.Sprintf("%s, %s, %s", roleID, object, action))
	return m.enforcer.AddNamedPolicy("p", roleID, object, action)
}

// RemovePolicy 移除权限策略
func (m *CasManager) RemovePolicy(roleID, object, action string) (bool, error) {
	m.logger.Log("RemovePolicy", fmt.Sprintf("%s, %s, %s", roleID, object, action))
	return m.enforcer.RemoveNamedPolicy("p", roleID, object, action)
}

// GetAllPolicies 获取所有策略
func (m *CasManager) GetAllPolicies() [][]string {
	policies, _ := m.enforcer.GetNamedPolicy("p")
	return policies
}

// ========== 角色管理 ==========

// AddRoleForUser 为用户添加角色
func (m *CasManager) AddRoleForUser(userID, roleID string) (bool, error) {
	m.logger.Log("AddRoleForUser", fmt.Sprintf("%s, %s", userID, roleID))
	return m.enforcer.AddRoleForUser(userID, roleID)
}

// DeleteRoleForUser 删除用户角色
func (m *CasManager) DeleteRoleForUser(userID, roleID string) (bool, error) {
	m.logger.Log("DeleteRoleForUser", fmt.Sprintf("%s, %s", userID, roleID))
	return m.enforcer.DeleteRoleForUser(userID, roleID)
}

// GetRolesForUser 获取用户的角色列表
func (m *CasManager) GetRolesForUser(userID string) []string {
	roles, _ := m.enforcer.GetRolesForUser(userID)
	m.logger.Log("GetRolesForUser", fmt.Sprintf("%s -> %v", userID, roles))
	return roles
}

// ========== 系统管理 ==========

// GetAllRoles 获取所有角色映射
func (m *CasManager) GetAllRoles() [][]string {
	roles, _ := m.enforcer.GetNamedGroupingPolicy("g")
	return roles
}

// SavePolicy 保存策略到后端。
// 纯内存模式下 enforcer 无适配器，SavePolicy 会触发 nil 解引用 panic；
// 因此内存模式直接返回 nil（策略已由 AutoSave/AddPolicy 作用于内存）。
func (m *CasManager) SavePolicy() error {
	if m.enforcer == nil || m.enforcer.GetAdapter() == nil {
		return nil
	}
	return m.enforcer.SavePolicy()
}

// LoadPolicy 从数据库加载策略
func (m *CasManager) LoadPolicy() error {
	return m.enforcer.LoadPolicy()
}
