package constants

const StatusSuccess = "success"
const StatusFailed = "failed"

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

// 用户来源定义：区分本地原生用户与第三方登录来源用户
const (
	UserSourceLocal  = "shadmin" // shadmin 本地原生用户
	UserSourceGitHub = "github"  // GitHub 第三方登录来源用户
	UserSourceGoogle = "google"  // Google 第三方登录来源用户
)
