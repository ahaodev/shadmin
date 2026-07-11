package casbin

import (
	"fmt"
	"shadmin/ent"
	"sync"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	entadapter "github.com/casbin/ent-adapter"
	adapterent "github.com/casbin/ent-adapter/ent"
	"github.com/casbin/redis-adapter/v3"
)

// Config 控制 Casbin 后端选择。RedisAddr 非空 → 走 Redis 适配器；否则为 Ent 适配器。
type Config struct {
	Debug         bool
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
	// RemoveFilteredPolicy 按字段过滤批量删除策略，fieldIndex=0 时即"删除某角色所有策略"
	RemoveFilteredPolicy(fieldIndex int, fieldValues ...string) (bool, error)
	GetAllPolicies() [][]string

	// AddRoleForUser 角色管理 - sync服务使用
	AddRoleForUser(userID, roleID string) (bool, error)
	DeleteRoleForUser(userID, roleID string) (bool, error)
	// DeleteRolesForUser 删除某用户的所有角色映射
	DeleteRolesForUser(userID string) (bool, error)
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
}

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
)

// NewCasManager 创建权限管理器实例
func NewCasManager(entClient *ent.Client, cfg Config) Manager {
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
	}
}

// initializeCasbin 初始化Casbin组件
func initializeCasbin(entClient *ent.Client, cfg Config) error {
	m, err := model.NewModelFromString(ModelConf)
	if err != nil {
		return err
	}

	adapter, err := newCasbinAdapter(entClient, cfg)
	if err != nil {
		return err
	}

	enforcer, err = casbin.NewEnforcer(m, adapter)
	if err != nil {
		return err
	}

	enforcer.EnableAutoSave(true)
	if cfg.Debug {
		enforcer.SetLogger(newCasbinLogger())
	}
	return nil
}

func newCasbinAdapter(entClient *ent.Client, cfg Config) (any, error) {
	if cfg.RedisAddr != "" {
		adapterKey := "casbin_rules"
		if cfg.RedisDB != 0 {
			adapterKey = fmt.Sprintf("casbin_rules:%d", cfg.RedisDB)
		}

		config := redisadapter.Config{
			Network:  "tcp",
			Address:  cfg.RedisAddr,
			Password: cfg.RedisPassword,
			Key:      adapterKey,
		}

		return redisadapter.NewAdapter(&config)
	}

	adapterClient := adapterent.NewClient(adapterent.Driver(entClient.Driver()))
	return entadapter.NewAdapterWithClient(adapterClient)
}

// GetEnforcer 获取 enforcer 实例
func (m *CasManager) GetEnforcer() *casbin.Enforcer {
	return m.enforcer
}

// ========== 核心权限检查 ==========

// CheckPermission 检查权限
func (m *CasManager) CheckPermission(userID, object, action string) (bool, error) {
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
	return m.enforcer.AddNamedPolicy("p", roleID, object, action)
}

// RemovePolicy 移除权限策略
func (m *CasManager) RemovePolicy(roleID, object, action string) (bool, error) {
	return m.enforcer.RemoveNamedPolicy("p", roleID, object, action)
}

// RemoveFilteredPolicy 按字段过滤批量删除策略（fieldIndex=0 即删某角色所有策略）
func (m *CasManager) RemoveFilteredPolicy(fieldIndex int, fieldValues ...string) (bool, error) {
	return m.enforcer.RemoveFilteredNamedPolicy("p", fieldIndex, fieldValues...)
}

// GetAllPolicies 获取所有策略
func (m *CasManager) GetAllPolicies() [][]string {
	policies, _ := m.enforcer.GetNamedPolicy("p")
	return policies
}

// ========== 角色管理 ==========

// AddRoleForUser 为用户添加角色
func (m *CasManager) AddRoleForUser(userID, roleID string) (bool, error) {
	return m.enforcer.AddRoleForUser(userID, roleID)
}

// DeleteRoleForUser 删除用户的单个角色
func (m *CasManager) DeleteRoleForUser(userID, roleID string) (bool, error) {
	return m.enforcer.DeleteRoleForUser(userID, roleID)
}

// DeleteRolesForUser 删除用户的所有角色映射
func (m *CasManager) DeleteRolesForUser(userID string) (bool, error) {
	return m.enforcer.DeleteRolesForUser(userID)
}

// GetRolesForUser 获取用户的角色列表
func (m *CasManager) GetRolesForUser(userID string) []string {
	roles, _ := m.enforcer.GetRolesForUser(userID)
	return roles
}

// ========== 系统管理 ==========

func (m *CasManager) GetAllRoles() [][]string {
	roles, _ := m.enforcer.GetNamedGroupingPolicy("g")
	return roles
}

func (m *CasManager) SavePolicy() error {
	if m.enforcer == nil || m.enforcer.GetAdapter() == nil {
		return nil
	}
	return m.enforcer.SavePolicy()
}

func (m *CasManager) LoadPolicy() error {
	return m.enforcer.LoadPolicy()
}
