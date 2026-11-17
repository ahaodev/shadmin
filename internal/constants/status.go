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

const (
	UserSourceLocal  = domain.UserSourceLocal
	UserSourceGitHub = domain.UserSourceGitHub
	UserSourceGoogle = domain.UserSourceGoogle
)
