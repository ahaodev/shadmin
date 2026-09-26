package constants

import "shadmin/domain"

const AuthorizationStateID = "global"

const (
	StatusSuccess = domain.StatusSuccess
	StatusFailed  = domain.StatusFailed

	StatusActive   = domain.StatusActive
	StatusInactive = domain.StatusInactive

	UserStatusActive    = domain.UserStatusActive
	UserStatusInactive  = domain.UserStatusInactive
	UserStatusInvited   = domain.UserStatusInvited
	UserStatusSuspended = domain.UserStatusSuspended
)

// 用户来源定义：区分本地原生用户与第三方登录来源用户
const (
	UserSourceLocal  = "shadmin" // shadmin 本地原生用户
	UserSourceGitHub = "github"  // GitHub 第三方登录来源用户
	UserSourceGoogle = "google"  // Google 第三方登录来源用户
)
