package google

import "github.com/startcodex/ldap-google-sync/internal/config"

// GoogleClient defines the interface for Google Admin SDK operations
type GoogleClient interface {
	// Users
	FetchUsers() ([]config.User, error)
	CreateUser(user config.User, orgUnit string, generatePassword bool) error
	UpdateUser(user config.User) error
	SuspendUser(email string) error
	DeleteUser(email string) error

	// Groups
	FetchGroups() ([]config.Group, error)
	CreateGroup(group config.Group) error
	UpdateGroup(group config.Group) error
	DeleteGroup(email string) error
	FetchGroupMembers(groupEmail string) ([]string, error)
	AddGroupMember(groupEmail, memberEmail string) error
	RemoveGroupMember(groupEmail, memberEmail string) error

	// OrgUnits
	FetchOrgUnits() ([]config.OrgUnit, error)
	CreateOrgUnit(ou config.OrgUnit) error
	MoveUserToOrgUnit(email, orgUnitPath string) error
}

// Ensure Client implements GoogleClient
var _ GoogleClient = (*Client)(nil)
