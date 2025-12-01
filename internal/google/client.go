package google

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	"github.com/startcodex/ldap-google-sync/internal/config"
)

// Client wraps Google Admin SDK operations
type Client struct {
	service *admin.Service
	cfg     config.GoogleConfig
	ctx     context.Context
}

// Backoff configuration
const (
	maxRetries     = 5
	initialBackoff = 1 * time.Second
	maxBackoff     = 60 * time.Second
)

// NewClient creates a new Google Admin SDK client
func NewClient(ctx context.Context, cfg config.GoogleConfig) (*Client, error) {
	data, err := readFileFromOS(cfg.CredentialsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	jwtConfig, err := google.JWTConfigFromJSON(data,
		admin.AdminDirectoryUserScope,
		admin.AdminDirectoryGroupScope,
		admin.AdminDirectoryOrgunitScope,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	jwtConfig.Subject = cfg.AdminEmail

	service, err := admin.NewService(ctx, option.WithHTTPClient(jwtConfig.Client(ctx)))
	if err != nil {
		return nil, fmt.Errorf("failed to create admin service: %w", err)
	}

	log.Printf("Successfully connected to Google Admin SDK for domain %s", cfg.Domain)
	return &Client{
		service: service,
		cfg:     cfg,
		ctx:     ctx,
	}, nil
}

// ============================================================================
// USER OPERATIONS
// ============================================================================

// FetchUsers retrieves all users from Google Workspace
func (c *Client) FetchUsers() ([]config.User, error) {
	var users []config.User
	pageToken := ""

	for {
		call := c.service.Users.List().
			Domain(c.cfg.Domain).
			MaxResults(500).
			OrderBy("email")

		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		result, err := c.executeWithRetry(func() (interface{}, error) {
			return call.Do()
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list users: %w", err)
		}

		response := result.(*admin.Users)
		for _, u := range response.Users {
			user := config.User{
				UID:       u.Id,
				Email:     u.PrimaryEmail,
				FirstName: u.Name.GivenName,
				LastName:  u.Name.FamilyName,
				OrgUnit:   u.OrgUnitPath,
				Source:    "google",
			}

			if phones, ok := u.Phones.([]*admin.UserPhone); ok && len(phones) > 0 {
				user.Phone = phones[0].Value
			}

			if orgs, ok := u.Organizations.([]interface{}); ok && len(orgs) > 0 {
				if org, ok := orgs[0].(map[string]interface{}); ok {
					if dept, exists := org["department"]; exists {
						user.Department = fmt.Sprintf("%v", dept)
					}
					if title, exists := org["title"]; exists {
						user.Title = fmt.Sprintf("%v", title)
					}
				}
			}

			users = append(users, user)
		}

		pageToken = response.NextPageToken
		if pageToken == "" {
			break
		}
	}

	log.Printf("Found %d users in Google Workspace", len(users))
	return users, nil
}

// CreateUser creates a new user in Google Workspace
func (c *Client) CreateUser(user config.User, orgUnit string, generatePassword bool) error {
	password := ""
	if generatePassword {
		password = generateRandomPassword(16)
	}

	newUser := &admin.User{
		PrimaryEmail:              user.Email,
		Password:                  password,
		ChangePasswordAtNextLogin: true,
		OrgUnitPath:               orgUnit,
		Name: &admin.UserName{
			GivenName:  user.FirstName,
			FamilyName: user.LastName,
		},
	}

	if user.Phone != "" {
		newUser.Phones = []*admin.UserPhone{
			{Value: user.Phone, Type: "work", Primary: true},
		}
	}

	if user.Department != "" || user.Title != "" {
		org := make(map[string]interface{})
		org["primary"] = true
		if user.Department != "" {
			org["department"] = user.Department
		}
		if user.Title != "" {
			org["title"] = user.Title
		}
		newUser.Organizations = []interface{}{org}
	}

	_, err := c.executeWithRetry(func() (interface{}, error) {
		return c.service.Users.Insert(newUser).Do()
	})
	if err != nil {
		return fmt.Errorf("failed to create user %s: %w", user.Email, err)
	}

	log.Printf("Created user: %s", user.Email)
	return nil
}

// UpdateUser updates an existing user in Google Workspace
func (c *Client) UpdateUser(user config.User) error {
	updateUser := &admin.User{
		Name: &admin.UserName{
			GivenName:  user.FirstName,
			FamilyName: user.LastName,
		},
	}

	if user.Phone != "" {
		updateUser.Phones = []*admin.UserPhone{
			{Value: user.Phone, Type: "work", Primary: true},
		}
	}

	if user.Department != "" || user.Title != "" {
		org := make(map[string]interface{})
		org["primary"] = true
		if user.Department != "" {
			org["department"] = user.Department
		}
		if user.Title != "" {
			org["title"] = user.Title
		}
		updateUser.Organizations = []interface{}{org}
	}

	_, err := c.executeWithRetry(func() (interface{}, error) {
		return c.service.Users.Update(user.Email, updateUser).Do()
	})
	if err != nil {
		return fmt.Errorf("failed to update user %s: %w", user.Email, err)
	}

	log.Printf("Updated user: %s", user.Email)
	return nil
}

// SuspendUser suspends a user in Google Workspace
func (c *Client) SuspendUser(email string) error {
	updateUser := &admin.User{
		Suspended:       true,
		ForceSendFields: []string{"Suspended"},
	}

	_, err := c.executeWithRetry(func() (interface{}, error) {
		return c.service.Users.Update(email, updateUser).Do()
	})
	if err != nil {
		return fmt.Errorf("failed to suspend user %s: %w", email, err)
	}

	log.Printf("Suspended user: %s", email)
	return nil
}

// DeleteUser deletes a user from Google Workspace
func (c *Client) DeleteUser(email string) error {
	_, err := c.executeWithRetry(func() (interface{}, error) {
		return nil, c.service.Users.Delete(email).Do()
	})
	if err != nil {
		return fmt.Errorf("failed to delete user %s: %w", email, err)
	}

	log.Printf("Deleted user: %s", email)
	return nil
}

// ============================================================================
// GROUP OPERATIONS
// ============================================================================

// FetchGroups retrieves all groups from Google Workspace
func (c *Client) FetchGroups() ([]config.Group, error) {
	var groups []config.Group
	pageToken := ""

	for {
		call := c.service.Groups.List().
			Domain(c.cfg.Domain).
			MaxResults(200)

		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		result, err := c.executeWithRetry(func() (interface{}, error) {
			return call.Do()
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list groups: %w", err)
		}

		response := result.(*admin.Groups)
		for _, g := range response.Groups {
			group := config.Group{
				Name:        g.Name,
				Email:       g.Email,
				Description: g.Description,
				Source:      "google",
			}
			groups = append(groups, group)
		}

		pageToken = response.NextPageToken
		if pageToken == "" {
			break
		}
	}

	log.Printf("Found %d groups in Google Workspace", len(groups))
	return groups, nil
}

// CreateGroup creates a new group in Google Workspace
func (c *Client) CreateGroup(group config.Group) error {
	newGroup := &admin.Group{
		Name:        group.Name,
		Email:       group.Email,
		Description: group.Description,
	}

	_, err := c.executeWithRetry(func() (interface{}, error) {
		return c.service.Groups.Insert(newGroup).Do()
	})
	if err != nil {
		return fmt.Errorf("failed to create group %s: %w", group.Email, err)
	}

	log.Printf("Created group: %s", group.Email)
	return nil
}

// UpdateGroup updates an existing group in Google Workspace
func (c *Client) UpdateGroup(group config.Group) error {
	updateGroup := &admin.Group{
		Name:        group.Name,
		Description: group.Description,
	}

	_, err := c.executeWithRetry(func() (interface{}, error) {
		return c.service.Groups.Update(group.Email, updateGroup).Do()
	})
	if err != nil {
		return fmt.Errorf("failed to update group %s: %w", group.Email, err)
	}

	log.Printf("Updated group: %s", group.Email)
	return nil
}

// DeleteGroup deletes a group from Google Workspace
func (c *Client) DeleteGroup(email string) error {
	_, err := c.executeWithRetry(func() (interface{}, error) {
		return nil, c.service.Groups.Delete(email).Do()
	})
	if err != nil {
		return fmt.Errorf("failed to delete group %s: %w", email, err)
	}

	log.Printf("Deleted group: %s", email)
	return nil
}

// FetchGroupMembers retrieves all members of a group
func (c *Client) FetchGroupMembers(groupEmail string) ([]string, error) {
	var members []string
	pageToken := ""

	for {
		call := c.service.Members.List(groupEmail).MaxResults(200)

		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		result, err := c.executeWithRetry(func() (interface{}, error) {
			return call.Do()
		})
		if err != nil {
			if isNotFoundError(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to list members for group %s: %w", groupEmail, err)
		}

		response := result.(*admin.Members)
		for _, m := range response.Members {
			members = append(members, m.Email)
		}

		pageToken = response.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return members, nil
}

// AddGroupMember adds a member to a group
func (c *Client) AddGroupMember(groupEmail, memberEmail string) error {
	member := &admin.Member{
		Email: memberEmail,
		Role:  "MEMBER",
	}

	_, err := c.executeWithRetry(func() (interface{}, error) {
		return c.service.Members.Insert(groupEmail, member).Do()
	})
	if err != nil {
		// Ignore if already a member
		if apiErr, ok := err.(*googleapi.Error); ok && apiErr.Code == 409 {
			return nil
		}
		return fmt.Errorf("failed to add member %s to group %s: %w", memberEmail, groupEmail, err)
	}

	log.Printf("Added member %s to group %s", memberEmail, groupEmail)
	return nil
}

// RemoveGroupMember removes a member from a group
func (c *Client) RemoveGroupMember(groupEmail, memberEmail string) error {
	_, err := c.executeWithRetry(func() (interface{}, error) {
		return nil, c.service.Members.Delete(groupEmail, memberEmail).Do()
	})
	if err != nil {
		if isNotFoundError(err) {
			return nil
		}
		return fmt.Errorf("failed to remove member %s from group %s: %w", memberEmail, groupEmail, err)
	}

	log.Printf("Removed member %s from group %s", memberEmail, groupEmail)
	return nil
}

// ============================================================================
// ORG UNIT OPERATIONS
// ============================================================================

// FetchOrgUnits retrieves all organizational units from Google Workspace
func (c *Client) FetchOrgUnits() ([]config.OrgUnit, error) {
	result, err := c.executeWithRetry(func() (interface{}, error) {
		return c.service.Orgunits.List("my_customer").Type("all").Do()
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list org units: %w", err)
	}

	response := result.(*admin.OrgUnits)
	var orgUnits []config.OrgUnit

	for _, ou := range response.OrganizationUnits {
		orgUnit := config.OrgUnit{
			Name:        ou.Name,
			Path:        ou.OrgUnitPath,
			ParentPath:  ou.ParentOrgUnitPath,
			Description: ou.Description,
			Source:      "google",
		}
		orgUnits = append(orgUnits, orgUnit)
	}

	log.Printf("Found %d organizational units in Google Workspace", len(orgUnits))
	return orgUnits, nil
}

// CreateOrgUnit creates a new organizational unit in Google Workspace
func (c *Client) CreateOrgUnit(ou config.OrgUnit) error {
	newOU := &admin.OrgUnit{
		Name:              ou.Name,
		ParentOrgUnitPath: ou.ParentPath,
		Description:       ou.Description,
	}

	_, err := c.executeWithRetry(func() (interface{}, error) {
		return c.service.Orgunits.Insert("my_customer", newOU).Do()
	})
	if err != nil {
		// Ignore if already exists
		if apiErr, ok := err.(*googleapi.Error); ok && apiErr.Code == 409 {
			log.Printf("Org unit already exists: %s", ou.Path)
			return nil
		}
		return fmt.Errorf("failed to create org unit %s: %w", ou.Path, err)
	}

	log.Printf("Created org unit: %s", ou.Path)
	return nil
}

// MoveUserToOrgUnit moves a user to an organizational unit
func (c *Client) MoveUserToOrgUnit(email, orgUnitPath string) error {
	updateUser := &admin.User{
		OrgUnitPath: orgUnitPath,
	}

	_, err := c.executeWithRetry(func() (interface{}, error) {
		return c.service.Users.Update(email, updateUser).Do()
	})
	if err != nil {
		return fmt.Errorf("failed to move user %s to OU %s: %w", email, orgUnitPath, err)
	}

	log.Printf("Moved user %s to OU: %s", email, orgUnitPath)
	return nil
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// executeWithRetry executes a function with exponential backoff
func (c *Client) executeWithRetry(fn func() (interface{}, error)) (interface{}, error) {
	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		result, err := fn()
		if err == nil {
			return result, nil
		}

		if !isRetryableError(err) {
			return nil, err
		}

		lastErr = err

		if attempt < maxRetries {
			jitter := time.Duration(randomInt64(int64(backoff / 2)))
			sleepTime := backoff + jitter

			if sleepTime > maxBackoff {
				sleepTime = maxBackoff
			}

			log.Printf("Rate limited, retrying in %v (attempt %d/%d): %v",
				sleepTime, attempt+1, maxRetries, err)

			time.Sleep(sleepTime)
			backoff *= 2
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

func isRetryableError(err error) bool {
	apiErr, ok := err.(*googleapi.Error)
	if !ok {
		return false
	}
	return apiErr.Code == 429 ||
		(apiErr.Code == 403 && containsRateLimitReason(apiErr)) ||
		apiErr.Code >= 500
}

func containsRateLimitReason(err *googleapi.Error) bool {
	for _, e := range err.Errors {
		if e.Reason == "rateLimitExceeded" ||
			e.Reason == "userRateLimitExceeded" ||
			e.Reason == "quotaExceeded" {
			return true
		}
	}
	return false
}

func isNotFoundError(err error) bool {
	apiErr, ok := err.(*googleapi.Error)
	if !ok {
		return false
	}
	return apiErr.Code == 404
}

func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()"
	b := make([]byte, length)
	for i := range b {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[idx.Int64()]
	}
	return string(b)
}

func randomInt64(max int64) int64 {
	if max <= 0 {
		return 0
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(max))
	return n.Int64()
}

// NeedsUpdate compares LDAP user with Google user to determine if update is needed
func NeedsUpdate(ldapUser, googleUser config.User) bool {
	return ldapUser.FirstName != googleUser.FirstName ||
		ldapUser.LastName != googleUser.LastName ||
		(ldapUser.Phone != "" && ldapUser.Phone != googleUser.Phone) ||
		(ldapUser.Department != "" && ldapUser.Department != googleUser.Department) ||
		(ldapUser.Title != "" && ldapUser.Title != googleUser.Title)
}

// GroupNeedsUpdate compares LDAP group with Google group
func GroupNeedsUpdate(ldapGroup, googleGroup config.Group) bool {
	return ldapGroup.Name != googleGroup.Name ||
		(ldapGroup.Description != "" && ldapGroup.Description != googleGroup.Description)
}

// NormalizeEmail normalizes an email address for comparison
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
