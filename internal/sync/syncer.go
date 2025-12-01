package sync

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/startcodex/ldap-google-sync/internal/config"
	"github.com/startcodex/ldap-google-sync/internal/google"
	"github.com/startcodex/ldap-google-sync/internal/ldap"
)

// Stats holds synchronization statistics
type Stats struct {
	StartTime time.Time
	EndTime   time.Time
	// Users
	LDAPUsers      int
	GoogleUsers    int
	UsersCreated   int
	UsersUpdated   int
	UsersSuspended int
	UsersDeleted   int
	UsersSkipped   int
	// Groups
	LDAPGroups     int
	GoogleGroups   int
	GroupsCreated  int
	GroupsUpdated  int
	GroupsDeleted  int
	GroupsSkipped  int
	MembersAdded   int
	MembersRemoved int
	// OrgUnits
	LDAPOrgUnits    int
	GoogleOrgUnits  int
	OrgUnitsCreated int
	// Errors
	Errors        int
	ErrorMessages []string
}

// Syncer orchestrates the synchronization between LDAP and Google
type Syncer struct {
	cfg          *config.Config
	ldapClient   ldap.LDAPClient
	googleClient google.GoogleClient
	stats        Stats
}

// NewSyncer creates a new Syncer instance
func NewSyncer(cfg *config.Config) (*Syncer, error) {
	return &Syncer{
		cfg: cfg,
	}, nil
}

// NewSyncerWithClients creates a Syncer with injected clients (for testing)
func NewSyncerWithClients(cfg *config.Config, ldapClient ldap.LDAPClient, googleClient google.GoogleClient) *Syncer {
	return &Syncer{
		cfg:          cfg,
		ldapClient:   ldapClient,
		googleClient: googleClient,
	}
}

// Run executes the full synchronization process
func (s *Syncer) Run(ctx context.Context) (*Stats, error) {
	s.stats = Stats{StartTime: time.Now()}

	log.Println("========================================")
	log.Println("LDAP to Google Workspace Sync Starting")
	log.Println("========================================")

	if s.cfg.Sync.DryRun {
		log.Println("*** DRY RUN MODE - No changes will be applied ***")
	}

	// If clients not injected, create them
	if s.ldapClient == nil {
		log.Println("\n[1/5] Connecting to LDAP...")
		ldapClient, err := ldap.NewClient(s.cfg.LDAP)
		if err != nil {
			return nil, fmt.Errorf("failed to create LDAP client: %w", err)
		}
		s.ldapClient = ldapClient

		if err := s.ldapClient.Connect(); err != nil {
			return nil, fmt.Errorf("failed to connect to LDAP: %w", err)
		}
		defer s.ldapClient.Close()
	}

	if s.googleClient == nil {
		log.Println("\n[2/5] Connecting to Google Admin SDK...")
		googleClient, err := google.NewClient(ctx, s.cfg.Google)
		if err != nil {
			return nil, fmt.Errorf("failed to create Google client: %w", err)
		}
		s.googleClient = googleClient
	}

	// Sync OrgUnits first (they need to exist before users can be placed in them)
	if s.cfg.Sync.SyncOrgUnits {
		log.Println("\n[3/5] Synchronizing Organizational Units...")
		if err := s.syncOrgUnits(); err != nil {
			s.recordError(fmt.Sprintf("OrgUnit sync failed: %v", err))
		}
	} else {
		log.Println("\n[3/5] Skipping Organizational Units (disabled)")
	}

	// Sync Users
	if s.cfg.Sync.SyncUsers {
		log.Println("\n[4/5] Synchronizing Users...")
		if err := s.syncUsers(); err != nil {
			s.recordError(fmt.Sprintf("User sync failed: %v", err))
		}
	} else {
		log.Println("\n[4/5] Skipping Users (disabled)")
	}

	// Sync Groups
	if s.cfg.Sync.SyncGroups {
		log.Println("\n[5/5] Synchronizing Groups...")
		if err := s.syncGroups(); err != nil {
			s.recordError(fmt.Sprintf("Group sync failed: %v", err))
		}
	} else {
		log.Println("\n[5/5] Skipping Groups (disabled)")
	}

	s.stats.EndTime = time.Now()
	s.printStats()

	return &s.stats, nil
}

// RunWithClients executes sync with pre-configured clients (for testing)
func (s *Syncer) RunWithClients(ctx context.Context) (*Stats, error) {
	return s.Run(ctx)
}

// ============================================================================
// USER SYNCHRONIZATION
// ============================================================================

func (s *Syncer) syncUsers() error {
	ldapUsers, err := s.ldapClient.FetchUsers()
	if err != nil {
		return fmt.Errorf("failed to fetch LDAP users: %w", err)
	}
	s.stats.LDAPUsers = len(ldapUsers)

	googleUsers, err := s.googleClient.FetchUsers()
	if err != nil {
		return fmt.Errorf("failed to fetch Google users: %w", err)
	}
	s.stats.GoogleUsers = len(googleUsers)

	// Create maps for quick lookup
	ldapMap := make(map[string]config.User)
	for _, u := range ldapUsers {
		ldapMap[google.NormalizeEmail(u.Email)] = u
	}

	googleMap := make(map[string]config.User)
	for _, u := range googleUsers {
		googleMap[google.NormalizeEmail(u.Email)] = u
	}

	// Process LDAP users (create or update in Google)
	for email, ldapUser := range ldapMap {
		if googleUser, exists := googleMap[email]; exists {
			if s.cfg.Sync.UpdateUsers && google.NeedsUpdate(ldapUser, googleUser) {
				if err := s.updateUser(ldapUser); err != nil {
					s.recordError(fmt.Sprintf("Failed to update user %s: %v", email, err))
				}
			} else {
				s.stats.UsersSkipped++
			}
		} else {
			if s.cfg.Sync.CreateUsers {
				orgUnit := s.cfg.Sync.DefaultOrgUnit
				if ldapUser.OrgUnit != "" {
					orgUnit = "/" + ldapUser.OrgUnit
				}
				if err := s.createUser(ldapUser, orgUnit); err != nil {
					s.recordError(fmt.Sprintf("Failed to create user %s: %v", email, err))
				}
			} else {
				s.stats.UsersSkipped++
				log.Printf("Skipping creation of user %s (CreateUsers disabled)", email)
			}
		}
	}

	// Process Google users not in LDAP (suspend or delete)
	if s.cfg.Sync.SuspendMissingUsers {
		for email, googleUser := range googleMap {
			if _, exists := ldapMap[email]; !exists {
				if ShouldSkipUser(googleUser) {
					log.Printf("Skipping admin/service account: %s", email)
					s.stats.UsersSkipped++
					continue
				}

				if s.cfg.Sync.DeleteInsteadOfSuspend {
					if err := s.deleteUser(email); err != nil {
						s.recordError(fmt.Sprintf("Failed to delete user %s: %v", email, err))
					}
				} else {
					if err := s.suspendUser(email); err != nil {
						s.recordError(fmt.Sprintf("Failed to suspend user %s: %v", email, err))
					}
				}
			}
		}
	}

	return nil
}

func (s *Syncer) createUser(user config.User, orgUnit string) error {
	log.Printf("Creating user: %s (%s %s)", user.Email, user.FirstName, user.LastName)

	if s.cfg.Sync.DryRun {
		log.Printf("  [DRY RUN] Would create user %s", user.Email)
		s.stats.UsersCreated++
		return nil
	}

	err := s.googleClient.CreateUser(user, orgUnit, true)
	if err != nil {
		s.stats.Errors++
		return err
	}

	s.stats.UsersCreated++
	return nil
}

func (s *Syncer) updateUser(user config.User) error {
	log.Printf("Updating user: %s", user.Email)

	if s.cfg.Sync.DryRun {
		log.Printf("  [DRY RUN] Would update user %s", user.Email)
		s.stats.UsersUpdated++
		return nil
	}

	err := s.googleClient.UpdateUser(user)
	if err != nil {
		s.stats.Errors++
		return err
	}

	s.stats.UsersUpdated++
	return nil
}

func (s *Syncer) suspendUser(email string) error {
	log.Printf("Suspending user: %s (not found in LDAP)", email)

	if s.cfg.Sync.DryRun {
		log.Printf("  [DRY RUN] Would suspend user %s", email)
		s.stats.UsersSuspended++
		return nil
	}

	err := s.googleClient.SuspendUser(email)
	if err != nil {
		s.stats.Errors++
		return err
	}

	s.stats.UsersSuspended++
	return nil
}

func (s *Syncer) deleteUser(email string) error {
	log.Printf("Deleting user: %s (not found in LDAP)", email)

	if s.cfg.Sync.DryRun {
		log.Printf("  [DRY RUN] Would delete user %s", email)
		s.stats.UsersDeleted++
		return nil
	}

	err := s.googleClient.DeleteUser(email)
	if err != nil {
		s.stats.Errors++
		return err
	}

	s.stats.UsersDeleted++
	return nil
}

// ShouldSkipUser checks if a user should be skipped from suspension/deletion
func ShouldSkipUser(user config.User) bool {
	email := strings.ToLower(user.Email)

	adminPatterns := []string{
		"admin@", "administrator@", "postmaster@", "abuse@", "security@",
	}

	for _, pattern := range adminPatterns {
		if strings.HasPrefix(email, pattern) {
			return true
		}
	}

	if strings.Contains(email, "gserviceaccount.com") {
		return true
	}

	return false
}

// ============================================================================
// GROUP SYNCHRONIZATION
// ============================================================================

func (s *Syncer) syncGroups() error {
	ldapGroups, err := s.ldapClient.FetchGroups()
	if err != nil {
		return fmt.Errorf("failed to fetch LDAP groups: %w", err)
	}
	s.stats.LDAPGroups = len(ldapGroups)

	googleGroups, err := s.googleClient.FetchGroups()
	if err != nil {
		return fmt.Errorf("failed to fetch Google groups: %w", err)
	}
	s.stats.GoogleGroups = len(googleGroups)

	// Create map of Google groups by email
	googleMap := make(map[string]config.Group)
	for _, g := range googleGroups {
		googleMap[google.NormalizeEmail(g.Email)] = g
	}

	// Process LDAP groups
	for _, ldapGroup := range ldapGroups {
		groupEmail := ldapGroup.Email
		if groupEmail == "" {
			groupEmail = strings.ToLower(ldapGroup.Name) + s.cfg.Sync.GroupEmailSuffix
		}
		groupEmail = google.NormalizeEmail(groupEmail)
		ldapGroup.Email = groupEmail

		if googleGroup, exists := googleMap[groupEmail]; exists {
			if s.cfg.Sync.UpdateGroups && google.GroupNeedsUpdate(ldapGroup, googleGroup) {
				if err := s.updateGroup(ldapGroup); err != nil {
					s.recordError(fmt.Sprintf("Failed to update group %s: %v", groupEmail, err))
				}
			} else {
				s.stats.GroupsSkipped++
			}

			if s.cfg.Sync.SyncGroupMembers {
				if err := s.syncGroupMembers(ldapGroup); err != nil {
					s.recordError(fmt.Sprintf("Failed to sync members for group %s: %v", groupEmail, err))
				}
			}
		} else {
			if s.cfg.Sync.CreateGroups {
				if err := s.createGroup(ldapGroup); err != nil {
					s.recordError(fmt.Sprintf("Failed to create group %s: %v", groupEmail, err))
				} else if s.cfg.Sync.SyncGroupMembers {
					if err := s.syncGroupMembers(ldapGroup); err != nil {
						s.recordError(fmt.Sprintf("Failed to sync members for group %s: %v", groupEmail, err))
					}
				}
			} else {
				s.stats.GroupsSkipped++
			}
		}
	}

	// Delete Google groups not in LDAP
	if s.cfg.Sync.DeleteMissingGroups {
		ldapEmails := make(map[string]bool)
		for _, g := range ldapGroups {
			email := g.Email
			if email == "" {
				email = strings.ToLower(g.Name) + s.cfg.Sync.GroupEmailSuffix
			}
			ldapEmails[google.NormalizeEmail(email)] = true
		}

		for email := range googleMap {
			if !ldapEmails[email] {
				if err := s.deleteGroup(email); err != nil {
					s.recordError(fmt.Sprintf("Failed to delete group %s: %v", email, err))
				}
			}
		}
	}

	return nil
}

func (s *Syncer) createGroup(group config.Group) error {
	log.Printf("Creating group: %s (%s)", group.Email, group.Name)

	if s.cfg.Sync.DryRun {
		log.Printf("  [DRY RUN] Would create group %s", group.Email)
		s.stats.GroupsCreated++
		return nil
	}

	err := s.googleClient.CreateGroup(group)
	if err != nil {
		s.stats.Errors++
		return err
	}

	s.stats.GroupsCreated++
	return nil
}

func (s *Syncer) updateGroup(group config.Group) error {
	log.Printf("Updating group: %s", group.Email)

	if s.cfg.Sync.DryRun {
		log.Printf("  [DRY RUN] Would update group %s", group.Email)
		s.stats.GroupsUpdated++
		return nil
	}

	err := s.googleClient.UpdateGroup(group)
	if err != nil {
		s.stats.Errors++
		return err
	}

	s.stats.GroupsUpdated++
	return nil
}

func (s *Syncer) deleteGroup(email string) error {
	log.Printf("Deleting group: %s (not found in LDAP)", email)

	if s.cfg.Sync.DryRun {
		log.Printf("  [DRY RUN] Would delete group %s", email)
		s.stats.GroupsDeleted++
		return nil
	}

	err := s.googleClient.DeleteGroup(email)
	if err != nil {
		s.stats.Errors++
		return err
	}

	s.stats.GroupsDeleted++
	return nil
}

func (s *Syncer) syncGroupMembers(group config.Group) error {
	ldapMemberEmails, err := s.ldapClient.ResolveMemberEmails(group.Members)
	if err != nil {
		return fmt.Errorf("failed to resolve member emails: %w", err)
	}

	googleMembers, err := s.googleClient.FetchGroupMembers(group.Email)
	if err != nil {
		return fmt.Errorf("failed to fetch Google group members: %w", err)
	}

	ldapSet := make(map[string]bool)
	for _, email := range ldapMemberEmails {
		ldapSet[google.NormalizeEmail(email)] = true
	}

	googleSet := make(map[string]bool)
	for _, email := range googleMembers {
		googleSet[google.NormalizeEmail(email)] = true
	}

	for email := range ldapSet {
		if !googleSet[email] {
			if err := s.addGroupMember(group.Email, email); err != nil {
				s.recordError(fmt.Sprintf("Failed to add member %s to group %s: %v", email, group.Email, err))
			}
		}
	}

	for email := range googleSet {
		if !ldapSet[email] {
			if err := s.removeGroupMember(group.Email, email); err != nil {
				s.recordError(fmt.Sprintf("Failed to remove member %s from group %s: %v", email, group.Email, err))
			}
		}
	}

	return nil
}

func (s *Syncer) addGroupMember(groupEmail, memberEmail string) error {
	log.Printf("Adding member %s to group %s", memberEmail, groupEmail)

	if s.cfg.Sync.DryRun {
		log.Printf("  [DRY RUN] Would add member %s to group %s", memberEmail, groupEmail)
		s.stats.MembersAdded++
		return nil
	}

	err := s.googleClient.AddGroupMember(groupEmail, memberEmail)
	if err != nil {
		s.stats.Errors++
		return err
	}

	s.stats.MembersAdded++
	return nil
}

func (s *Syncer) removeGroupMember(groupEmail, memberEmail string) error {
	log.Printf("Removing member %s from group %s", memberEmail, groupEmail)

	if s.cfg.Sync.DryRun {
		log.Printf("  [DRY RUN] Would remove member %s from group %s", memberEmail, groupEmail)
		s.stats.MembersRemoved++
		return nil
	}

	err := s.googleClient.RemoveGroupMember(groupEmail, memberEmail)
	if err != nil {
		s.stats.Errors++
		return err
	}

	s.stats.MembersRemoved++
	return nil
}

// ============================================================================
// ORG UNIT SYNCHRONIZATION
// ============================================================================

func (s *Syncer) syncOrgUnits() error {
	ldapOrgUnits, err := s.ldapClient.FetchOrgUnits()
	if err != nil {
		return fmt.Errorf("failed to fetch LDAP org units: %w", err)
	}
	s.stats.LDAPOrgUnits = len(ldapOrgUnits)

	googleOrgUnits, err := s.googleClient.FetchOrgUnits()
	if err != nil {
		return fmt.Errorf("failed to fetch Google org units: %w", err)
	}
	s.stats.GoogleOrgUnits = len(googleOrgUnits)

	googlePaths := make(map[string]bool)
	for _, ou := range googleOrgUnits {
		googlePaths[ou.Path] = true
	}

	sort.Slice(ldapOrgUnits, func(i, j int) bool {
		return strings.Count(ldapOrgUnits[i].Path, "/") < strings.Count(ldapOrgUnits[j].Path, "/")
	})

	if s.cfg.Sync.CreateOrgUnits {
		for _, ou := range ldapOrgUnits {
			if ou.Path == "/" {
				continue
			}

			if !googlePaths[ou.Path] {
				if err := s.createOrgUnit(ou); err != nil {
					s.recordError(fmt.Sprintf("Failed to create org unit %s: %v", ou.Path, err))
				} else {
					googlePaths[ou.Path] = true
				}
			}
		}
	}

	return nil
}

func (s *Syncer) createOrgUnit(ou config.OrgUnit) error {
	log.Printf("Creating org unit: %s", ou.Path)

	if s.cfg.Sync.DryRun {
		log.Printf("  [DRY RUN] Would create org unit %s", ou.Path)
		s.stats.OrgUnitsCreated++
		return nil
	}

	err := s.googleClient.CreateOrgUnit(ou)
	if err != nil {
		s.stats.Errors++
		return err
	}

	s.stats.OrgUnitsCreated++
	return nil
}

// ============================================================================
// HELPERS
// ============================================================================

func (s *Syncer) recordError(msg string) {
	s.stats.Errors++
	s.stats.ErrorMessages = append(s.stats.ErrorMessages, msg)
	log.Printf("ERROR: %s", msg)
}

func (s *Syncer) printStats() {
	duration := s.stats.EndTime.Sub(s.stats.StartTime)

	log.Println("\n========================================")
	log.Println("Synchronization Complete")
	log.Println("========================================")
	log.Printf("Duration: %v", duration.Round(time.Second))

	log.Println("\n--- Users ---")
	log.Printf("LDAP Users:     %d", s.stats.LDAPUsers)
	log.Printf("Google Users:   %d", s.stats.GoogleUsers)
	log.Printf("Created:        %d", s.stats.UsersCreated)
	log.Printf("Updated:        %d", s.stats.UsersUpdated)
	log.Printf("Suspended:      %d", s.stats.UsersSuspended)
	log.Printf("Deleted:        %d", s.stats.UsersDeleted)
	log.Printf("Skipped:        %d", s.stats.UsersSkipped)

	if s.cfg.Sync.SyncGroups {
		log.Println("\n--- Groups ---")
		log.Printf("LDAP Groups:    %d", s.stats.LDAPGroups)
		log.Printf("Google Groups:  %d", s.stats.GoogleGroups)
		log.Printf("Created:        %d", s.stats.GroupsCreated)
		log.Printf("Updated:        %d", s.stats.GroupsUpdated)
		log.Printf("Deleted:        %d", s.stats.GroupsDeleted)
		log.Printf("Skipped:        %d", s.stats.GroupsSkipped)
		log.Printf("Members Added:  %d", s.stats.MembersAdded)
		log.Printf("Members Removed:%d", s.stats.MembersRemoved)
	}

	if s.cfg.Sync.SyncOrgUnits {
		log.Println("\n--- Org Units ---")
		log.Printf("LDAP OUs:       %d", s.stats.LDAPOrgUnits)
		log.Printf("Google OUs:     %d", s.stats.GoogleOrgUnits)
		log.Printf("Created:        %d", s.stats.OrgUnitsCreated)
	}

	log.Println("\n--- Summary ---")
	log.Printf("Total Errors:   %d", s.stats.Errors)

	if len(s.stats.ErrorMessages) > 0 {
		log.Println("\nError Details:")
		for _, msg := range s.stats.ErrorMessages {
			log.Printf("  - %s", msg)
		}
	}

	log.Println("========================================")
}
