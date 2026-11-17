package domain

const (
	StatusSuccess = "success"
	StatusFailed  = "failed"

	StatusActive   = "active"
	StatusInactive = "inactive"

	UserStatusActive    = StatusActive
	UserStatusInactive  = StatusInactive
	UserStatusInvited   = "invited"
	UserStatusSuspended = "suspended"
)

const (
	UserSourceLocal  = "shadmin"
	UserSourceGitHub = "github"
	UserSourceGoogle = "google"
)
