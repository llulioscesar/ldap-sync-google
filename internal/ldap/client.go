package ldap

import (
	"crypto/tls"
	"fmt"
	"log"
	"strings"

	"github.com/go-ldap/ldap/v3"
	"github.com/startcodex/ldap-google-sync/internal/config"
)

// Client wraps LDAP connection and operations
type Client struct {
	conn *ldap.Conn
	cfg  config.LDAPConfig
}

// NewClient creates a new LDAP client
func NewClient(cfg config.LDAPConfig) (*Client, error) {
	return &Client{cfg: cfg}, nil
}

// Connect establishes connection to LDAP server
func (c *Client) Connect() error {
	var err error
	address := fmt.Sprintf("%s:%d", c.cfg.Host, c.cfg.Port)

	if c.cfg.UseTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: false,
			ServerName:         c.cfg.Host,
		}
		c.conn, err = ldap.DialTLS("tcp", address, tlsConfig)
	} else {
		c.conn, err = ldap.Dial("tcp", address)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to LDAP server: %w", err)
	}

	if c.cfg.BindDN != "" {
		err = c.conn.Bind(c.cfg.BindDN, c.cfg.BindPassword)
		if err != nil {
			return fmt.Errorf("failed to bind to LDAP: %w", err)
		}
	}

	log.Printf("Successfully connected to LDAP server at %s", address)
	return nil
}

// Close closes the LDAP connection
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

// FetchUsers retrieves all users from LDAP matching the filter
func (c *Client) FetchUsers() ([]config.User, error) {
	attrs := c.cfg.UserAttributes
	searchAttrs := []string{
		attrs.UID,
		attrs.Email,
		attrs.FirstName,
		attrs.LastName,
		attrs.Phone,
		attrs.Department,
		attrs.Title,
		attrs.OrgUnit,
	}

	searchRequest := ldap.NewSearchRequest(
		c.cfg.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		c.cfg.UserFilter,
		searchAttrs,
		nil,
	)

	result, err := c.conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("LDAP user search failed: %w", err)
	}

	var users []config.User
	for _, entry := range result.Entries {
		user := config.User{
			UID:        entry.GetAttributeValue(attrs.UID),
			Email:      entry.GetAttributeValue(attrs.Email),
			FirstName:  entry.GetAttributeValue(attrs.FirstName),
			LastName:   entry.GetAttributeValue(attrs.LastName),
			Phone:      entry.GetAttributeValue(attrs.Phone),
			Department: entry.GetAttributeValue(attrs.Department),
			Title:      entry.GetAttributeValue(attrs.Title),
			OrgUnit:    entry.GetAttributeValue(attrs.OrgUnit),
			Source:     "ldap",
		}

		if user.Email == "" {
			log.Printf("Skipping LDAP user %s: no email address", user.UID)
			continue
		}

		users = append(users, user)
	}

	log.Printf("Found %d users in LDAP", len(users))
	return users, nil
}

// FetchGroups retrieves all groups from LDAP matching the filter
func (c *Client) FetchGroups() ([]config.Group, error) {
	attrs := c.cfg.GroupAttributes
	searchAttrs := []string{
		attrs.Name,
		attrs.Email,
		attrs.Description,
		attrs.Member,
	}

	baseDN := c.cfg.GroupBaseDN
	if baseDN == "" {
		baseDN = c.cfg.BaseDN
	}

	searchRequest := ldap.NewSearchRequest(
		baseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		c.cfg.GroupFilter,
		searchAttrs,
		nil,
	)

	result, err := c.conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("LDAP group search failed: %w", err)
	}

	var groups []config.Group
	for _, entry := range result.Entries {
		group := config.Group{
			Name:        entry.GetAttributeValue(attrs.Name),
			Email:       entry.GetAttributeValue(attrs.Email),
			Description: entry.GetAttributeValue(attrs.Description),
			Members:     entry.GetAttributeValues(attrs.Member),
			Source:      "ldap",
		}

		if group.Name == "" {
			continue
		}

		groups = append(groups, group)
	}

	log.Printf("Found %d groups in LDAP", len(groups))
	return groups, nil
}

// FetchOrgUnits extracts organizational units from LDAP structure
func (c *Client) FetchOrgUnits() ([]config.OrgUnit, error) {
	searchRequest := ldap.NewSearchRequest(
		c.cfg.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		"(objectClass=organizationalUnit)",
		[]string{"ou", "description"},
		nil,
	)

	result, err := c.conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("LDAP OU search failed: %w", err)
	}

	var orgUnits []config.OrgUnit
	for _, entry := range result.Entries {
		ou := config.OrgUnit{
			Name:        entry.GetAttributeValue("ou"),
			Description: entry.GetAttributeValue("description"),
			Source:      "ldap",
		}

		// Convert DN to path: "ou=Sales,ou=Departments,dc=example,dc=com" -> "/Departments/Sales"
		ou.Path = dnToPath(entry.DN, c.cfg.BaseDN)
		ou.ParentPath = getParentPath(ou.Path)

		if ou.Name == "" {
			continue
		}

		orgUnits = append(orgUnits, ou)
	}

	log.Printf("Found %d organizational units in LDAP", len(orgUnits))
	return orgUnits, nil
}

// ResolveMemberEmails converts member identifiers to email addresses
// Supports both DN format (groupOfNames) and UID format (posixGroup)
func (c *Client) ResolveMemberEmails(members []string) ([]string, error) {
	var emails []string

	for _, member := range members {
		email, err := c.resolveMemberEmail(member)
		if err != nil {
			log.Printf("Warning: could not resolve member %s: %v", member, err)
			continue
		}
		if email != "" {
			emails = append(emails, email)
		}
	}

	return emails, nil
}

// resolveMemberEmail resolves a single member identifier to an email
func (c *Client) resolveMemberEmail(member string) (string, error) {
	// Check if it's a DN (contains "=") or a UID (simple username)
	if strings.Contains(member, "=") {
		// It's a DN (groupOfNames style: cn=john,ou=users,dc=example,dc=com)
		return c.resolveEmailByDN(member)
	}
	// It's a UID (posixGroup style: john)
	return c.resolveEmailByUID(member)
}

// resolveEmailByDN looks up email by DN (for groupOfNames)
func (c *Client) resolveEmailByDN(dn string) (string, error) {
	searchRequest := ldap.NewSearchRequest(
		dn,
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		1,
		0,
		false,
		"(objectClass=*)",
		[]string{c.cfg.UserAttributes.Email},
		nil,
	)

	result, err := c.conn.Search(searchRequest)
	if err != nil {
		return "", err
	}

	if len(result.Entries) > 0 {
		return result.Entries[0].GetAttributeValue(c.cfg.UserAttributes.Email), nil
	}

	return "", nil
}

// resolveEmailByUID looks up email by UID (for posixGroup)
func (c *Client) resolveEmailByUID(uid string) (string, error) {
	// Search for user with matching UID attribute
	filter := fmt.Sprintf("(&%s(%s=%s))", c.cfg.UserFilter, c.cfg.UserAttributes.UID, ldap.EscapeFilter(uid))

	searchRequest := ldap.NewSearchRequest(
		c.cfg.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1,
		0,
		false,
		filter,
		[]string{c.cfg.UserAttributes.Email},
		nil,
	)

	result, err := c.conn.Search(searchRequest)
	if err != nil {
		return "", err
	}

	if len(result.Entries) > 0 {
		return result.Entries[0].GetAttributeValue(c.cfg.UserAttributes.Email), nil
	}

	return "", nil
}

// StartTLS upgrades the connection to TLS
func (c *Client) StartTLS() error {
	if c.conn == nil {
		return fmt.Errorf("no active LDAP connection")
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         c.cfg.Host,
	}

	err := c.conn.StartTLS(tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to start TLS: %w", err)
	}

	return nil
}

// dnToPath converts an LDAP DN to a Google Workspace OU path
// e.g., "ou=Sales,ou=Departments,dc=example,dc=com" with baseDN "dc=example,dc=com"
// becomes "/Departments/Sales"
func dnToPath(dn, baseDN string) string {
	// Remove base DN from the end
	relativeDN := strings.TrimSuffix(dn, ","+baseDN)
	if relativeDN == dn {
		// baseDN not found, might be the root
		relativeDN = strings.TrimSuffix(dn, baseDN)
	}

	// Split by comma and extract OU values
	parts := strings.Split(relativeDN, ",")
	var pathParts []string

	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if strings.HasPrefix(strings.ToLower(part), "ou=") {
			pathParts = append(pathParts, part[3:])
		}
	}

	if len(pathParts) == 0 {
		return "/"
	}

	return "/" + strings.Join(pathParts, "/")
}

// getParentPath returns the parent path of an OU path
func getParentPath(path string) string {
	if path == "/" || path == "" {
		return "/"
	}

	lastSlash := strings.LastIndex(path, "/")
	if lastSlash <= 0 {
		return "/"
	}

	return path[:lastSlash]
}
