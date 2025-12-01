package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration for the sync process
type Config struct {
	LDAP   LDAPConfig
	Google GoogleConfig
	Sync   SyncConfig
}

// LDAPConfig holds LDAP connection settings
type LDAPConfig struct {
	Host           string
	Port           int
	UseTLS         bool
	BindDN         string
	BindPassword   string
	BaseDN         string
	UserFilter     string
	GroupFilter    string
	GroupBaseDN    string
	UserAttributes  LDAPUserAttributes
	GroupAttributes LDAPGroupAttributes
}

// LDAPUserAttributes maps LDAP attributes to user fields
type LDAPUserAttributes struct {
	UID        string
	Email      string
	FirstName  string
	LastName   string
	Phone      string
	Department string
	Title      string
	OrgUnit    string // For mapping users to OUs
}

// LDAPGroupAttributes maps LDAP attributes to group fields
type LDAPGroupAttributes struct {
	Name        string
	Email       string
	Description string
	Member      string
}

// GoogleConfig holds Google Workspace settings
type GoogleConfig struct {
	CredentialsFile string
	AdminEmail      string
	Domain          string
}

// SyncConfig holds synchronization settings
type SyncConfig struct {
	DryRun                 bool
	// Users
	SyncUsers              bool
	CreateUsers            bool
	UpdateUsers            bool
	SuspendMissingUsers    bool
	DeleteInsteadOfSuspend bool
	DefaultOrgUnit         string
	// Groups
	SyncGroups             bool
	CreateGroups           bool
	UpdateGroups           bool
	DeleteMissingGroups    bool
	SyncGroupMembers       bool
	GroupEmailSuffix       string // e.g., "@groups.example.com"
	// OrgUnits
	SyncOrgUnits           bool
	CreateOrgUnits         bool
}

// User represents a synchronized user
type User struct {
	UID        string
	Email      string
	FirstName  string
	LastName   string
	Phone      string
	Department string
	Title      string
	OrgUnit    string
	Source     string // "ldap" or "google"
}

// Group represents a synchronized group
type Group struct {
	Name        string
	Email       string
	Description string
	Members     []string // List of member emails
	Source      string   // "ldap" or "google"
}

// OrgUnit represents an organizational unit
type OrgUnit struct {
	Name        string
	Path        string // Full path like "/Sales/LATAM"
	ParentPath  string
	Description string
	Source      string // "ldap" or "google"
}

// LoadFromEnv loads configuration from environment variables
func LoadFromEnv() (*Config, error) {
	cfg := &Config{}

	// LDAP Config
	cfg.LDAP.Host = getEnvOrDefault("LDAP_HOST", "localhost")
	cfg.LDAP.Port = getEnvAsInt("LDAP_PORT", 389)
	cfg.LDAP.UseTLS = getEnvAsBool("LDAP_USE_TLS", false)
	cfg.LDAP.BindDN = os.Getenv("LDAP_BIND_DN")
	cfg.LDAP.BindPassword = os.Getenv("LDAP_BIND_PASSWORD")
	cfg.LDAP.BaseDN = os.Getenv("LDAP_BASE_DN")
	cfg.LDAP.UserFilter = getEnvOrDefault("LDAP_USER_FILTER", "(objectClass=inetOrgPerson)")
	cfg.LDAP.GroupFilter = getEnvOrDefault("LDAP_GROUP_FILTER", "(|(objectClass=groupOfNames)(objectClass=posixGroup))")
	cfg.LDAP.GroupBaseDN = getEnvOrDefault("LDAP_GROUP_BASE_DN", os.Getenv("LDAP_BASE_DN"))

	// LDAP User Attribute Mapping
	cfg.LDAP.UserAttributes.UID = getEnvOrDefault("LDAP_ATTR_UID", "uid")
	cfg.LDAP.UserAttributes.Email = getEnvOrDefault("LDAP_ATTR_EMAIL", "mail")
	cfg.LDAP.UserAttributes.FirstName = getEnvOrDefault("LDAP_ATTR_FIRSTNAME", "givenName")
	cfg.LDAP.UserAttributes.LastName = getEnvOrDefault("LDAP_ATTR_LASTNAME", "sn")
	cfg.LDAP.UserAttributes.Phone = getEnvOrDefault("LDAP_ATTR_PHONE", "telephoneNumber")
	cfg.LDAP.UserAttributes.Department = getEnvOrDefault("LDAP_ATTR_DEPARTMENT", "departmentNumber")
	cfg.LDAP.UserAttributes.Title = getEnvOrDefault("LDAP_ATTR_TITLE", "title")
	cfg.LDAP.UserAttributes.OrgUnit = getEnvOrDefault("LDAP_ATTR_ORG_UNIT", "ou")

	// LDAP Group Attribute Mapping
	cfg.LDAP.GroupAttributes.Name = getEnvOrDefault("LDAP_ATTR_GROUP_NAME", "cn")
	cfg.LDAP.GroupAttributes.Email = getEnvOrDefault("LDAP_ATTR_GROUP_EMAIL", "mail")
	cfg.LDAP.GroupAttributes.Description = getEnvOrDefault("LDAP_ATTR_GROUP_DESC", "description")
	cfg.LDAP.GroupAttributes.Member = getEnvOrDefault("LDAP_ATTR_GROUP_MEMBER", "memberUid")

	// Google Config
	cfg.Google.CredentialsFile = os.Getenv("GOOGLE_CREDENTIALS_FILE")
	cfg.Google.AdminEmail = os.Getenv("GOOGLE_ADMIN_EMAIL")
	cfg.Google.Domain = os.Getenv("GOOGLE_DOMAIN")

	// Sync Config - Users
	cfg.Sync.DryRun = getEnvAsBool("SYNC_DRY_RUN", true)
	cfg.Sync.SyncUsers = getEnvAsBool("SYNC_USERS", true)
	cfg.Sync.CreateUsers = getEnvAsBool("SYNC_CREATE_USERS", true)
	cfg.Sync.UpdateUsers = getEnvAsBool("SYNC_UPDATE_USERS", true)
	cfg.Sync.SuspendMissingUsers = getEnvAsBool("SYNC_SUSPEND_MISSING_USERS", false)
	cfg.Sync.DeleteInsteadOfSuspend = getEnvAsBool("SYNC_DELETE_INSTEAD_OF_SUSPEND", false)
	cfg.Sync.DefaultOrgUnit = getEnvOrDefault("SYNC_DEFAULT_ORG_UNIT", "/")

	// Sync Config - Groups
	cfg.Sync.SyncGroups = getEnvAsBool("SYNC_GROUPS", false)
	cfg.Sync.CreateGroups = getEnvAsBool("SYNC_CREATE_GROUPS", true)
	cfg.Sync.UpdateGroups = getEnvAsBool("SYNC_UPDATE_GROUPS", true)
	cfg.Sync.DeleteMissingGroups = getEnvAsBool("SYNC_DELETE_MISSING_GROUPS", false)
	cfg.Sync.SyncGroupMembers = getEnvAsBool("SYNC_GROUP_MEMBERS", true)
	cfg.Sync.GroupEmailSuffix = getEnvOrDefault("SYNC_GROUP_EMAIL_SUFFIX", "@"+os.Getenv("GOOGLE_DOMAIN"))

	// Sync Config - OrgUnits
	cfg.Sync.SyncOrgUnits = getEnvAsBool("SYNC_ORG_UNITS", false)
	cfg.Sync.CreateOrgUnits = getEnvAsBool("SYNC_CREATE_ORG_UNITS", true)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks if all required configuration is present
func (c *Config) Validate() error {
	var missing []string

	if c.LDAP.Host == "" {
		missing = append(missing, "LDAP_HOST")
	}
	if c.LDAP.BaseDN == "" {
		missing = append(missing, "LDAP_BASE_DN")
	}
	if c.Google.CredentialsFile == "" {
		missing = append(missing, "GOOGLE_CREDENTIALS_FILE")
	}
	if c.Google.AdminEmail == "" {
		missing = append(missing, "GOOGLE_ADMIN_EMAIL")
	}
	if c.Google.Domain == "" {
		missing = append(missing, "GOOGLE_DOMAIN")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}
