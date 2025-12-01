package ldap

import "github.com/startcodex/ldap-google-sync/internal/config"

// LDAPClient defines the interface for LDAP operations
type LDAPClient interface {
	Connect() error
	Close()
	FetchUsers() ([]config.User, error)
	FetchGroups() ([]config.Group, error)
	FetchOrgUnits() ([]config.OrgUnit, error)
	ResolveMemberEmails(memberDNs []string) ([]string, error)
}

// Ensure Client implements LDAPClient
var _ LDAPClient = (*Client)(nil)
