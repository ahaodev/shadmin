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

// User sources identify local and external identity providers.
const (
	UserSourceLocal  = "shadmin"
	UserSourceGitHub = "github"
	UserSourceGoogle = "google"
)
