package casbin

import (
	"fmt"
	"sync"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
)

// enforcer 用 SyncedEnforcer：策略写入（hook 触发的后台同步 / 定时调度器）
var (
	enforcer *casbin.SyncedEnforcer
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
	// GetFilteredPolicies 按字段过滤读取 p 规则副本，是 RemoveFilteredPolicy 的读侧对应物。
	// 同步单个角色时用它只取该角色的规则，避免为每个角色深拷贝整个策略集。
	GetFilteredPolicies(fieldIndex int, fieldValues ...string) [][]string

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
}

// CasManager 权限管理器实现
type CasManager struct {
	enforcer *casbin.SyncedEnforcer
}

// NewCasManager 创建权限管理器实例
func NewCasManager(adapter any) Manager {
	var err error

	once.Do(func() {
		err = initializeCasbin(adapter)
	})

	if err != nil {
		panic(fmt.Errorf("failed to initialize casbin manager: %w", err))
	}

	return &CasManager{
		enforcer: enforcer,
	}
}

// newEnforcer 构建一个独立的 enforcer。生产路径经 initializeCasbin 赋值给包级
// 单例；测试用它构造彼此隔离的实例，避免共享状态相互污染。
func newEnforcer(adapter any) (*casbin.SyncedEnforcer, error) {
	m, err := model.NewModelFromString(ModelConf)
	if err != nil {
		return nil, err
	}

	e, err := casbin.NewSyncedEnforcer(m, adapter)
	if err != nil {
		return nil, err
	}

	e.EnableAutoSave(true)
	e.SetLogger(newCasbinLogger())
	return e, nil
}

// initializeCasbin 初始化Casbin组件
func initializeCasbin(adapter any) error {
	e, err := newEnforcer(adapter)
	if err != nil {
		return err
	}
	enforcer = e
	return nil
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

// GetAllPolicies 获取所有策略的副本。
func (m *CasManager) GetAllPolicies() [][]string {
	lock := m.enforcer.GetLock()
	lock.RLock()
	defer lock.RUnlock()

	// 直接用内嵌的 Enforcer，避免 SyncedEnforcer 再次加读锁造成重入。
	policies, _ := m.enforcer.Enforcer.GetNamedPolicy("p")
	return cloneRules(policies)
}

// GetFilteredPolicies 按字段过滤获取 p 规则副本（fieldIndex=0 即取某角色全部策略）。
func (m *CasManager) GetFilteredPolicies(fieldIndex int, fieldValues ...string) [][]string {
	lock := m.enforcer.GetLock()
	lock.RLock()
	defer lock.RUnlock()

	policies, _ := m.enforcer.Enforcer.GetFilteredNamedPolicy("p", fieldIndex, fieldValues...)
	return cloneRules(policies)
}

// cloneRules 深拷贝策略行，隔断与 casbin 内部切片的共享。
func cloneRules(rules [][]string) [][]string {
	if rules == nil {
		return nil
	}

	cloned := make([][]string, 0, len(rules))
	for _, rule := range rules {
		cloned = append(cloned, append([]string(nil), rule...))
	}
	return cloned
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
	lock := m.enforcer.GetLock()
	lock.RLock()
	defer lock.RUnlock()

	roles, _ := m.enforcer.Enforcer.GetNamedGroupingPolicy("g")
	return cloneRules(roles)
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
