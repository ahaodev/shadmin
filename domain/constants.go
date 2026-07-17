package domain

const (
	// StatusActive and StatusInactive are the shared enabled/disabled values used across entities.
	StatusActive   = "active"
	StatusInactive = "inactive"
)

// 用户状态定义
const (
	UserStatusActive    = StatusActive
	UserStatusInactive  = StatusInactive
	UserStatusInvited   = "invited"   // 已邀请但未激活
	UserStatusSuspended = "suspended" // 已暂停
)
