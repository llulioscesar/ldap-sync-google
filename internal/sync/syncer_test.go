package sync

import (
	"context"
	"errors"
	"testing"

	"github.com/startcodex/ldap-google-sync/internal/config"
)

func TestShouldSkipUser(t *testing.T) {
	tests := []struct {
		name     string
		user     config.User
		expected bool
	}{
		{
			name:     "admin user",
			user:     config.User{Email: "admin@example.com"},
			expected: true,
		},
		{
			name:     "administrator user",
			user:     config.User{Email: "administrator@example.com"},
			expected: true,
		},
		{
			name:     "postmaster user",
			user:     config.User{Email: "postmaster@example.com"},
			expected: true,
		},
		{
			name:     "abuse user",
			user:     config.User{Email: "abuse@example.com"},
			expected: true,
		},
		{
			name:     "security user",
			user:     config.User{Email: "security@example.com"},
			expected: true,
		},
		{
			name:     "service account",
			user:     config.User{Email: "myapp@project.iam.gserviceaccount.com"},
			expected: true,
		},
		{
			name:     "regular user",
			user:     config.User{Email: "john.doe@example.com"},
			expected: false,
		},
		{
			name:     "user with admin in name",
			user:     config.User{Email: "john.admin.doe@example.com"},
			expected: false,
		},
		{
			name:     "uppercase admin",
			user:     config.User{Email: "ADMIN@example.com"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldSkipUser(tt.user)
			if result != tt.expected {
				t.Errorf("ShouldSkipUser(%v) = %v, want %v", tt.user.Email, result, tt.expected)
			}
		})
	}
}

func TestNewSyncer(t *testing.T) {
	cfg := &config.Config{}
	syncer, err := NewSyncer(cfg)
	if err != nil {
		t.Errorf("NewSyncer() error = %v", err)
	}
	if syncer == nil {
		t.Error("NewSyncer() returned nil")
	}
}

func TestNewSyncerWithClients(t *testing.T) {
	cfg := &config.Config{}
	ldapClient := &MockLDAPClient{}
	googleClient := &MockGoogleClient{}

	syncer := NewSyncerWithClients(cfg, ldapClient, googleClient)
	if syncer == nil {
		t.Error("NewSyncerWithClients() returned nil")
	}
	if syncer.cfg != cfg {
		t.Error("NewSyncerWithClients() cfg not set correctly")
	}
}

func TestSyncUsers_CreateNewUser(t *testing.T) {
	cfg := &config.Config{
		Sync: config.SyncConfig{
			SyncUsers:     true,
			CreateUsers:   true,
			DefaultOrgUnit: "/",
		},
	}

	ldapUsers := []config.User{
		{Email: "new@example.com", FirstName: "New", LastName: "User"},
	}
	googleUsers := []config.User{}

	var createdUser config.User
	var createdOrgUnit string

	ldapClient := &MockLDAPClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return ldapUsers, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	googleClient := &MockGoogleClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return googleUsers, nil
		},
		CreateUserFunc: func(user config.User, orgUnit string, generatePassword bool) error {
			createdUser = user
			createdOrgUnit = orgUnit
			return nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	syncer := NewSyncerWithClients(cfg, ldapClient, googleClient)
	stats, err := syncer.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if stats.UsersCreated != 1 {
		t.Errorf("UsersCreated = %d, want 1", stats.UsersCreated)
	}

	if createdUser.Email != "new@example.com" {
		t.Errorf("Created wrong user: %s", createdUser.Email)
	}

	if createdOrgUnit != "/" {
		t.Errorf("Created with wrong org unit: %s", createdOrgUnit)
	}
}

func TestSyncUsers_UpdateExistingUser(t *testing.T) {
	cfg := &config.Config{
		Sync: config.SyncConfig{
			SyncUsers:   true,
			UpdateUsers: true,
		},
	}

	ldapUsers := []config.User{
		{Email: "user@example.com", FirstName: "Updated", LastName: "Name"},
	}
	googleUsers := []config.User{
		{Email: "user@example.com", FirstName: "Old", LastName: "Name"},
	}

	var updatedUser config.User

	ldapClient := &MockLDAPClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return ldapUsers, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	googleClient := &MockGoogleClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return googleUsers, nil
		},
		UpdateUserFunc: func(user config.User) error {
			updatedUser = user
			return nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	syncer := NewSyncerWithClients(cfg, ldapClient, googleClient)
	stats, err := syncer.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if stats.UsersUpdated != 1 {
		t.Errorf("UsersUpdated = %d, want 1", stats.UsersUpdated)
	}

	if updatedUser.FirstName != "Updated" {
		t.Errorf("Updated wrong first name: %s", updatedUser.FirstName)
	}
}

func TestSyncUsers_SuspendMissingUser(t *testing.T) {
	cfg := &config.Config{
		Sync: config.SyncConfig{
			SyncUsers:           true,
			SuspendMissingUsers: true,
		},
	}

	ldapUsers := []config.User{}
	googleUsers := []config.User{
		{Email: "orphan@example.com", FirstName: "Orphan", LastName: "User"},
	}

	var suspendedEmail string

	ldapClient := &MockLDAPClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return ldapUsers, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	googleClient := &MockGoogleClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return googleUsers, nil
		},
		SuspendUserFunc: func(email string) error {
			suspendedEmail = email
			return nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	syncer := NewSyncerWithClients(cfg, ldapClient, googleClient)
	stats, err := syncer.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if stats.UsersSuspended != 1 {
		t.Errorf("UsersSuspended = %d, want 1", stats.UsersSuspended)
	}

	if suspendedEmail != "orphan@example.com" {
		t.Errorf("Suspended wrong user: %s", suspendedEmail)
	}
}

func TestSyncUsers_DeleteMissingUser(t *testing.T) {
	cfg := &config.Config{
		Sync: config.SyncConfig{
			SyncUsers:              true,
			SuspendMissingUsers:    true,
			DeleteInsteadOfSuspend: true,
		},
	}

	ldapUsers := []config.User{}
	googleUsers := []config.User{
		{Email: "orphan@example.com", FirstName: "Orphan", LastName: "User"},
	}

	var deletedEmail string

	ldapClient := &MockLDAPClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return ldapUsers, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	googleClient := &MockGoogleClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return googleUsers, nil
		},
		DeleteUserFunc: func(email string) error {
			deletedEmail = email
			return nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	syncer := NewSyncerWithClients(cfg, ldapClient, googleClient)
	stats, err := syncer.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if stats.UsersDeleted != 1 {
		t.Errorf("UsersDeleted = %d, want 1", stats.UsersDeleted)
	}

	if deletedEmail != "orphan@example.com" {
		t.Errorf("Deleted wrong user: %s", deletedEmail)
	}
}

func TestSyncUsers_DryRun(t *testing.T) {
	cfg := &config.Config{
		Sync: config.SyncConfig{
			SyncUsers:     true,
			CreateUsers:   true,
			DryRun:        true,
			DefaultOrgUnit: "/",
		},
	}

	ldapUsers := []config.User{
		{Email: "new@example.com", FirstName: "New", LastName: "User"},
	}
	googleUsers := []config.User{}

	createCalled := false

	ldapClient := &MockLDAPClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return ldapUsers, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	googleClient := &MockGoogleClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return googleUsers, nil
		},
		CreateUserFunc: func(user config.User, orgUnit string, generatePassword bool) error {
			createCalled = true
			return nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	syncer := NewSyncerWithClients(cfg, ldapClient, googleClient)
	stats, err := syncer.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if createCalled {
		t.Error("CreateUser was called in dry run mode")
	}

	if stats.UsersCreated != 1 {
		t.Errorf("UsersCreated = %d, want 1 (should count even in dry run)", stats.UsersCreated)
	}
}

func TestSyncGroups_CreateNewGroup(t *testing.T) {
	cfg := &config.Config{
		Sync: config.SyncConfig{
			SyncGroups:       true,
			CreateGroups:     true,
			GroupEmailSuffix: "@groups.example.com",
		},
	}

	ldapGroups := []config.Group{
		{Name: "sales", Email: "sales@groups.example.com", Description: "Sales Team"},
	}
	googleGroups := []config.Group{}

	var createdGroup config.Group

	ldapClient := &MockLDAPClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return nil, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return ldapGroups, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	googleClient := &MockGoogleClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return nil, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return googleGroups, nil
		},
		CreateGroupFunc: func(group config.Group) error {
			createdGroup = group
			return nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	syncer := NewSyncerWithClients(cfg, ldapClient, googleClient)
	stats, err := syncer.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if stats.GroupsCreated != 1 {
		t.Errorf("GroupsCreated = %d, want 1", stats.GroupsCreated)
	}

	if createdGroup.Name != "sales" {
		t.Errorf("Created wrong group: %s", createdGroup.Name)
	}
}

func TestSyncGroups_UpdateExistingGroup(t *testing.T) {
	cfg := &config.Config{
		Sync: config.SyncConfig{
			SyncGroups:   true,
			UpdateGroups: true,
		},
	}

	ldapGroups := []config.Group{
		{Name: "Sales Team Updated", Email: "sales@example.com", Description: "Updated description"},
	}
	googleGroups := []config.Group{
		{Name: "Sales Team", Email: "sales@example.com", Description: "Old description"},
	}

	var updatedGroup config.Group

	ldapClient := &MockLDAPClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return nil, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return ldapGroups, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	googleClient := &MockGoogleClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return nil, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return googleGroups, nil
		},
		UpdateGroupFunc: func(group config.Group) error {
			updatedGroup = group
			return nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	syncer := NewSyncerWithClients(cfg, ldapClient, googleClient)
	stats, err := syncer.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if stats.GroupsUpdated != 1 {
		t.Errorf("GroupsUpdated = %d, want 1", stats.GroupsUpdated)
	}

	if updatedGroup.Name != "Sales Team Updated" {
		t.Errorf("Updated wrong group name: %s", updatedGroup.Name)
	}
}

func TestSyncGroups_DeleteMissingGroup(t *testing.T) {
	cfg := &config.Config{
		Sync: config.SyncConfig{
			SyncGroups:          true,
			DeleteMissingGroups: true,
		},
	}

	ldapGroups := []config.Group{}
	googleGroups := []config.Group{
		{Name: "Orphan Group", Email: "orphan@example.com"},
	}

	var deletedEmail string

	ldapClient := &MockLDAPClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return nil, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return ldapGroups, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	googleClient := &MockGoogleClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return nil, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return googleGroups, nil
		},
		DeleteGroupFunc: func(email string) error {
			deletedEmail = email
			return nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	syncer := NewSyncerWithClients(cfg, ldapClient, googleClient)
	stats, err := syncer.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if stats.GroupsDeleted != 1 {
		t.Errorf("GroupsDeleted = %d, want 1", stats.GroupsDeleted)
	}

	if deletedEmail != "orphan@example.com" {
		t.Errorf("Deleted wrong group: %s", deletedEmail)
	}
}

func TestSyncGroups_SyncMembers(t *testing.T) {
	cfg := &config.Config{
		Sync: config.SyncConfig{
			SyncGroups:       true,
			SyncGroupMembers: true,
		},
	}

	ldapGroups := []config.Group{
		{
			Name:    "Sales Team",
			Email:   "sales@example.com",
			Members: []string{"cn=user1,dc=example,dc=com", "cn=user2,dc=example,dc=com"},
		},
	}
	googleGroups := []config.Group{
		{Name: "Sales Team", Email: "sales@example.com"},
	}

	var addedMembers []string
	var removedMembers []string

	ldapClient := &MockLDAPClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return nil, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return ldapGroups, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
		ResolveMemberEmailsFunc: func(memberDNs []string) ([]string, error) {
			return []string{"user1@example.com", "user2@example.com"}, nil
		},
	}

	googleClient := &MockGoogleClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return nil, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return googleGroups, nil
		},
		FetchGroupMembersFunc: func(groupEmail string) ([]string, error) {
			return []string{"user1@example.com", "user3@example.com"}, nil
		},
		AddGroupMemberFunc: func(groupEmail, memberEmail string) error {
			addedMembers = append(addedMembers, memberEmail)
			return nil
		},
		RemoveGroupMemberFunc: func(groupEmail, memberEmail string) error {
			removedMembers = append(removedMembers, memberEmail)
			return nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	syncer := NewSyncerWithClients(cfg, ldapClient, googleClient)
	stats, err := syncer.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if stats.MembersAdded != 1 {
		t.Errorf("MembersAdded = %d, want 1", stats.MembersAdded)
	}

	if stats.MembersRemoved != 1 {
		t.Errorf("MembersRemoved = %d, want 1", stats.MembersRemoved)
	}

	if len(addedMembers) != 1 || addedMembers[0] != "user2@example.com" {
		t.Errorf("Added wrong members: %v", addedMembers)
	}

	if len(removedMembers) != 1 || removedMembers[0] != "user3@example.com" {
		t.Errorf("Removed wrong members: %v", removedMembers)
	}
}

func TestSyncOrgUnits_CreateNewOrgUnit(t *testing.T) {
	cfg := &config.Config{
		Sync: config.SyncConfig{
			SyncOrgUnits:   true,
			CreateOrgUnits: true,
		},
	}

	ldapOrgUnits := []config.OrgUnit{
		{Name: "Sales", Path: "/Sales", ParentPath: "/"},
	}
	googleOrgUnits := []config.OrgUnit{}

	var createdOrgUnit config.OrgUnit

	ldapClient := &MockLDAPClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return nil, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return ldapOrgUnits, nil
		},
	}

	googleClient := &MockGoogleClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return nil, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return googleOrgUnits, nil
		},
		CreateOrgUnitFunc: func(ou config.OrgUnit) error {
			createdOrgUnit = ou
			return nil
		},
	}

	syncer := NewSyncerWithClients(cfg, ldapClient, googleClient)
	stats, err := syncer.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if stats.OrgUnitsCreated != 1 {
		t.Errorf("OrgUnitsCreated = %d, want 1", stats.OrgUnitsCreated)
	}

	if createdOrgUnit.Name != "Sales" {
		t.Errorf("Created wrong org unit: %s", createdOrgUnit.Name)
	}
}

func TestSyncOrgUnits_SkipRoot(t *testing.T) {
	cfg := &config.Config{
		Sync: config.SyncConfig{
			SyncOrgUnits:   true,
			CreateOrgUnits: true,
		},
	}

	ldapOrgUnits := []config.OrgUnit{
		{Name: "Root", Path: "/", ParentPath: ""},
	}
	googleOrgUnits := []config.OrgUnit{}

	createCalled := false

	ldapClient := &MockLDAPClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return nil, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return ldapOrgUnits, nil
		},
	}

	googleClient := &MockGoogleClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return nil, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return googleOrgUnits, nil
		},
		CreateOrgUnitFunc: func(ou config.OrgUnit) error {
			createCalled = true
			return nil
		},
	}

	syncer := NewSyncerWithClients(cfg, ldapClient, googleClient)
	stats, err := syncer.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if createCalled {
		t.Error("CreateOrgUnit was called for root path")
	}

	if stats.OrgUnitsCreated != 0 {
		t.Errorf("OrgUnitsCreated = %d, want 0", stats.OrgUnitsCreated)
	}
}

func TestSyncUsers_FetchLDAPError(t *testing.T) {
	cfg := &config.Config{
		Sync: config.SyncConfig{
			SyncUsers: true,
		},
	}

	ldapClient := &MockLDAPClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return nil, errors.New("LDAP connection failed")
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	googleClient := &MockGoogleClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return nil, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	syncer := NewSyncerWithClients(cfg, ldapClient, googleClient)
	stats, err := syncer.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() should not return error, got: %v", err)
	}

	if stats.Errors != 1 {
		t.Errorf("Errors = %d, want 1", stats.Errors)
	}
}

func TestSyncUsers_FetchGoogleError(t *testing.T) {
	cfg := &config.Config{
		Sync: config.SyncConfig{
			SyncUsers: true,
		},
	}

	ldapClient := &MockLDAPClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return []config.User{}, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	googleClient := &MockGoogleClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return nil, errors.New("Google API error")
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	syncer := NewSyncerWithClients(cfg, ldapClient, googleClient)
	stats, err := syncer.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() should not return error, got: %v", err)
	}

	if stats.Errors != 1 {
		t.Errorf("Errors = %d, want 1", stats.Errors)
	}
}

func TestSyncUsers_SkipAdminUser(t *testing.T) {
	cfg := &config.Config{
		Sync: config.SyncConfig{
			SyncUsers:           true,
			SuspendMissingUsers: true,
		},
	}

	ldapUsers := []config.User{}
	googleUsers := []config.User{
		{Email: "admin@example.com", FirstName: "Admin", LastName: "User"},
	}

	suspendCalled := false

	ldapClient := &MockLDAPClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return ldapUsers, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	googleClient := &MockGoogleClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return googleUsers, nil
		},
		SuspendUserFunc: func(email string) error {
			suspendCalled = true
			return nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	syncer := NewSyncerWithClients(cfg, ldapClient, googleClient)
	stats, err := syncer.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if suspendCalled {
		t.Error("SuspendUser was called for admin user")
	}

	if stats.UsersSkipped != 1 {
		t.Errorf("UsersSkipped = %d, want 1", stats.UsersSkipped)
	}
}

func TestSyncUsers_CreateError(t *testing.T) {
	cfg := &config.Config{
		Sync: config.SyncConfig{
			SyncUsers:     true,
			CreateUsers:   true,
			DefaultOrgUnit: "/",
		},
	}

	ldapUsers := []config.User{
		{Email: "new@example.com", FirstName: "New", LastName: "User"},
	}
	googleUsers := []config.User{}

	ldapClient := &MockLDAPClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return ldapUsers, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	googleClient := &MockGoogleClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return googleUsers, nil
		},
		CreateUserFunc: func(user config.User, orgUnit string, generatePassword bool) error {
			return errors.New("API error")
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	syncer := NewSyncerWithClients(cfg, ldapClient, googleClient)
	stats, err := syncer.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if stats.Errors < 1 {
		t.Errorf("Errors = %d, want >= 1", stats.Errors)
	}
}

func TestSyncUsers_UserWithOrgUnit(t *testing.T) {
	cfg := &config.Config{
		Sync: config.SyncConfig{
			SyncUsers:     true,
			CreateUsers:   true,
			DefaultOrgUnit: "/Default",
		},
	}

	ldapUsers := []config.User{
		{Email: "user@example.com", FirstName: "Test", LastName: "User", OrgUnit: "Sales"},
	}
	googleUsers := []config.User{}

	var createdOrgUnit string

	ldapClient := &MockLDAPClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return ldapUsers, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	googleClient := &MockGoogleClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return googleUsers, nil
		},
		CreateUserFunc: func(user config.User, orgUnit string, generatePassword bool) error {
			createdOrgUnit = orgUnit
			return nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	syncer := NewSyncerWithClients(cfg, ldapClient, googleClient)
	_, err := syncer.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if createdOrgUnit != "/Sales" {
		t.Errorf("Created with wrong org unit: %s, want /Sales", createdOrgUnit)
	}
}

func TestSyncGroups_GroupWithoutEmail(t *testing.T) {
	cfg := &config.Config{
		Sync: config.SyncConfig{
			SyncGroups:       true,
			CreateGroups:     true,
			GroupEmailSuffix: "@groups.example.com",
		},
	}

	ldapGroups := []config.Group{
		{Name: "sales", Email: "", Description: "Sales Team"},
	}
	googleGroups := []config.Group{}

	var createdGroup config.Group

	ldapClient := &MockLDAPClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return nil, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return ldapGroups, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	googleClient := &MockGoogleClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return nil, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return googleGroups, nil
		},
		CreateGroupFunc: func(group config.Group) error {
			createdGroup = group
			return nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	syncer := NewSyncerWithClients(cfg, ldapClient, googleClient)
	_, err := syncer.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if createdGroup.Email != "sales@groups.example.com" {
		t.Errorf("Created group with wrong email: %s, want sales@groups.example.com", createdGroup.Email)
	}
}

func TestStats_Duration(t *testing.T) {
	cfg := &config.Config{}

	ldapClient := &MockLDAPClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return nil, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	googleClient := &MockGoogleClient{
		FetchUsersFunc: func() ([]config.User, error) {
			return nil, nil
		},
		FetchGroupsFunc: func() ([]config.Group, error) {
			return nil, nil
		},
		FetchOrgUnitsFunc: func() ([]config.OrgUnit, error) {
			return nil, nil
		},
	}

	syncer := NewSyncerWithClients(cfg, ldapClient, googleClient)
	stats, err := syncer.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if stats.EndTime.Before(stats.StartTime) {
		t.Error("EndTime is before StartTime")
	}
}
