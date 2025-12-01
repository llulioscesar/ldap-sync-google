package sync

import (
	"github.com/startcodex/ldap-google-sync/internal/config"
)

// MockLDAPClient implements ldap.LDAPClient for testing
type MockLDAPClient struct {
	ConnectFunc            func() error
	CloseFunc              func()
	FetchUsersFunc         func() ([]config.User, error)
	FetchGroupsFunc        func() ([]config.Group, error)
	FetchOrgUnitsFunc      func() ([]config.OrgUnit, error)
	ResolveMemberEmailsFunc func(memberDNs []string) ([]string, error)
}

func (m *MockLDAPClient) Connect() error {
	if m.ConnectFunc != nil {
		return m.ConnectFunc()
	}
	return nil
}

func (m *MockLDAPClient) Close() {
	if m.CloseFunc != nil {
		m.CloseFunc()
	}
}

func (m *MockLDAPClient) FetchUsers() ([]config.User, error) {
	if m.FetchUsersFunc != nil {
		return m.FetchUsersFunc()
	}
	return nil, nil
}

func (m *MockLDAPClient) FetchGroups() ([]config.Group, error) {
	if m.FetchGroupsFunc != nil {
		return m.FetchGroupsFunc()
	}
	return nil, nil
}

func (m *MockLDAPClient) FetchOrgUnits() ([]config.OrgUnit, error) {
	if m.FetchOrgUnitsFunc != nil {
		return m.FetchOrgUnitsFunc()
	}
	return nil, nil
}

func (m *MockLDAPClient) ResolveMemberEmails(memberDNs []string) ([]string, error) {
	if m.ResolveMemberEmailsFunc != nil {
		return m.ResolveMemberEmailsFunc(memberDNs)
	}
	return nil, nil
}

// MockGoogleClient implements google.GoogleClient for testing
type MockGoogleClient struct {
	// Users
	FetchUsersFunc   func() ([]config.User, error)
	CreateUserFunc   func(user config.User, orgUnit string, generatePassword bool) error
	UpdateUserFunc   func(user config.User) error
	SuspendUserFunc  func(email string) error
	DeleteUserFunc   func(email string) error

	// Groups
	FetchGroupsFunc       func() ([]config.Group, error)
	CreateGroupFunc       func(group config.Group) error
	UpdateGroupFunc       func(group config.Group) error
	DeleteGroupFunc       func(email string) error
	FetchGroupMembersFunc func(groupEmail string) ([]string, error)
	AddGroupMemberFunc    func(groupEmail, memberEmail string) error
	RemoveGroupMemberFunc func(groupEmail, memberEmail string) error

	// OrgUnits
	FetchOrgUnitsFunc     func() ([]config.OrgUnit, error)
	CreateOrgUnitFunc     func(ou config.OrgUnit) error
	MoveUserToOrgUnitFunc func(email, orgUnitPath string) error
}

// Users
func (m *MockGoogleClient) FetchUsers() ([]config.User, error) {
	if m.FetchUsersFunc != nil {
		return m.FetchUsersFunc()
	}
	return nil, nil
}

func (m *MockGoogleClient) CreateUser(user config.User, orgUnit string, generatePassword bool) error {
	if m.CreateUserFunc != nil {
		return m.CreateUserFunc(user, orgUnit, generatePassword)
	}
	return nil
}

func (m *MockGoogleClient) UpdateUser(user config.User) error {
	if m.UpdateUserFunc != nil {
		return m.UpdateUserFunc(user)
	}
	return nil
}

func (m *MockGoogleClient) SuspendUser(email string) error {
	if m.SuspendUserFunc != nil {
		return m.SuspendUserFunc(email)
	}
	return nil
}

func (m *MockGoogleClient) DeleteUser(email string) error {
	if m.DeleteUserFunc != nil {
		return m.DeleteUserFunc(email)
	}
	return nil
}

// Groups
func (m *MockGoogleClient) FetchGroups() ([]config.Group, error) {
	if m.FetchGroupsFunc != nil {
		return m.FetchGroupsFunc()
	}
	return nil, nil
}

func (m *MockGoogleClient) CreateGroup(group config.Group) error {
	if m.CreateGroupFunc != nil {
		return m.CreateGroupFunc(group)
	}
	return nil
}

func (m *MockGoogleClient) UpdateGroup(group config.Group) error {
	if m.UpdateGroupFunc != nil {
		return m.UpdateGroupFunc(group)
	}
	return nil
}

func (m *MockGoogleClient) DeleteGroup(email string) error {
	if m.DeleteGroupFunc != nil {
		return m.DeleteGroupFunc(email)
	}
	return nil
}

func (m *MockGoogleClient) FetchGroupMembers(groupEmail string) ([]string, error) {
	if m.FetchGroupMembersFunc != nil {
		return m.FetchGroupMembersFunc(groupEmail)
	}
	return nil, nil
}

func (m *MockGoogleClient) AddGroupMember(groupEmail, memberEmail string) error {
	if m.AddGroupMemberFunc != nil {
		return m.AddGroupMemberFunc(groupEmail, memberEmail)
	}
	return nil
}

func (m *MockGoogleClient) RemoveGroupMember(groupEmail, memberEmail string) error {
	if m.RemoveGroupMemberFunc != nil {
		return m.RemoveGroupMemberFunc(groupEmail, memberEmail)
	}
	return nil
}

// OrgUnits
func (m *MockGoogleClient) FetchOrgUnits() ([]config.OrgUnit, error) {
	if m.FetchOrgUnitsFunc != nil {
		return m.FetchOrgUnitsFunc()
	}
	return nil, nil
}

func (m *MockGoogleClient) CreateOrgUnit(ou config.OrgUnit) error {
	if m.CreateOrgUnitFunc != nil {
		return m.CreateOrgUnitFunc(ou)
	}
	return nil
}

func (m *MockGoogleClient) MoveUserToOrgUnit(email, orgUnitPath string) error {
	if m.MoveUserToOrgUnitFunc != nil {
		return m.MoveUserToOrgUnitFunc(email, orgUnitPath)
	}
	return nil
}
