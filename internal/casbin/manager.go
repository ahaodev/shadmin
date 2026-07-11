package casbin

import (
	"fmt"
	"sync"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
)

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
	enforcer *casbin.Enforcer
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

// initializeCasbin 初始化Casbin组件
func initializeCasbin(adapter any) error {
	m, err := model.NewModelFromString(ModelConf)
	if err != nil {
		return err
	}

	enforcer, err = casbin.NewEnforcer(m, adapter)
	if err != nil {
		return err
	}

	enforcer.EnableAutoSave(true)

	enforcer.SetLogger(newCasbinLogger())
	return nil
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
