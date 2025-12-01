package config

import (
	"os"
	"testing"
)

func TestLoadFromEnv_Defaults(t *testing.T) {
	// Set required vars
	os.Setenv("LDAP_BASE_DN", "dc=test,dc=com")
	os.Setenv("GOOGLE_CREDENTIALS_FILE", "/tmp/creds.json")
	os.Setenv("GOOGLE_ADMIN_EMAIL", "admin@test.com")
	os.Setenv("GOOGLE_DOMAIN", "test.com")
	defer func() {
		os.Unsetenv("LDAP_BASE_DN")
		os.Unsetenv("GOOGLE_CREDENTIALS_FILE")
		os.Unsetenv("GOOGLE_ADMIN_EMAIL")
		os.Unsetenv("GOOGLE_DOMAIN")
	}()

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}

	// Check LDAP defaults
	if cfg.LDAP.Host != "localhost" {
		t.Errorf("expected LDAP.Host=localhost, got %s", cfg.LDAP.Host)
	}
	if cfg.LDAP.Port != 389 {
		t.Errorf("expected LDAP.Port=389, got %d", cfg.LDAP.Port)
	}
	if cfg.LDAP.UseTLS != false {
		t.Errorf("expected LDAP.UseTLS=false, got %v", cfg.LDAP.UseTLS)
	}
	if cfg.LDAP.UserFilter != "(objectClass=inetOrgPerson)" {
		t.Errorf("expected default UserFilter, got %s", cfg.LDAP.UserFilter)
	}
	if cfg.LDAP.GroupFilter != "(|(objectClass=groupOfNames)(objectClass=posixGroup))" {
		t.Errorf("expected default GroupFilter, got %s", cfg.LDAP.GroupFilter)
	}

	// Check user attribute defaults
	if cfg.LDAP.UserAttributes.UID != "uid" {
		t.Errorf("expected UID=uid, got %s", cfg.LDAP.UserAttributes.UID)
	}
	if cfg.LDAP.UserAttributes.Email != "mail" {
		t.Errorf("expected Email=mail, got %s", cfg.LDAP.UserAttributes.Email)
	}

	// Check group attribute defaults
	if cfg.LDAP.GroupAttributes.Name != "cn" {
		t.Errorf("expected GroupName=cn, got %s", cfg.LDAP.GroupAttributes.Name)
	}
	if cfg.LDAP.GroupAttributes.Member != "memberUid" {
		t.Errorf("expected GroupMember=memberUid, got %s", cfg.LDAP.GroupAttributes.Member)
	}

	// Check sync defaults
	if cfg.Sync.DryRun != true {
		t.Errorf("expected DryRun=true, got %v", cfg.Sync.DryRun)
	}
	if cfg.Sync.SyncUsers != true {
		t.Errorf("expected SyncUsers=true, got %v", cfg.Sync.SyncUsers)
	}
	if cfg.Sync.SyncGroups != false {
		t.Errorf("expected SyncGroups=false, got %v", cfg.Sync.SyncGroups)
	}
	if cfg.Sync.SyncOrgUnits != false {
		t.Errorf("expected SyncOrgUnits=false, got %v", cfg.Sync.SyncOrgUnits)
	}
	if cfg.Sync.DefaultOrgUnit != "/" {
		t.Errorf("expected DefaultOrgUnit=/, got %s", cfg.Sync.DefaultOrgUnit)
	}
}

func TestLoadFromEnv_CustomValues(t *testing.T) {
	// Set all vars
	os.Setenv("LDAP_HOST", "ldap.example.com")
	os.Setenv("LDAP_PORT", "636")
	os.Setenv("LDAP_USE_TLS", "true")
	os.Setenv("LDAP_BASE_DN", "dc=example,dc=com")
	os.Setenv("LDAP_USER_FILTER", "(objectClass=person)")
	os.Setenv("GOOGLE_CREDENTIALS_FILE", "/etc/creds.json")
	os.Setenv("GOOGLE_ADMIN_EMAIL", "admin@example.com")
	os.Setenv("GOOGLE_DOMAIN", "example.com")
	os.Setenv("SYNC_DRY_RUN", "false")
	os.Setenv("SYNC_GROUPS", "true")
	os.Setenv("SYNC_ORG_UNITS", "true")

	defer func() {
		os.Unsetenv("LDAP_HOST")
		os.Unsetenv("LDAP_PORT")
		os.Unsetenv("LDAP_USE_TLS")
		os.Unsetenv("LDAP_BASE_DN")
		os.Unsetenv("LDAP_USER_FILTER")
		os.Unsetenv("GOOGLE_CREDENTIALS_FILE")
		os.Unsetenv("GOOGLE_ADMIN_EMAIL")
		os.Unsetenv("GOOGLE_DOMAIN")
		os.Unsetenv("SYNC_DRY_RUN")
		os.Unsetenv("SYNC_GROUPS")
		os.Unsetenv("SYNC_ORG_UNITS")
	}()

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}

	if cfg.LDAP.Host != "ldap.example.com" {
		t.Errorf("expected LDAP.Host=ldap.example.com, got %s", cfg.LDAP.Host)
	}
	if cfg.LDAP.Port != 636 {
		t.Errorf("expected LDAP.Port=636, got %d", cfg.LDAP.Port)
	}
	if cfg.LDAP.UseTLS != true {
		t.Errorf("expected LDAP.UseTLS=true, got %v", cfg.LDAP.UseTLS)
	}
	if cfg.LDAP.UserFilter != "(objectClass=person)" {
		t.Errorf("expected custom UserFilter, got %s", cfg.LDAP.UserFilter)
	}
	if cfg.Sync.DryRun != false {
		t.Errorf("expected DryRun=false, got %v", cfg.Sync.DryRun)
	}
	if cfg.Sync.SyncGroups != true {
		t.Errorf("expected SyncGroups=true, got %v", cfg.Sync.SyncGroups)
	}
	if cfg.Sync.SyncOrgUnits != true {
		t.Errorf("expected SyncOrgUnits=true, got %v", cfg.Sync.SyncOrgUnits)
	}
}

func TestLoadFromEnv_MissingRequired(t *testing.T) {
	// Clear all env vars
	os.Unsetenv("LDAP_BASE_DN")
	os.Unsetenv("GOOGLE_CREDENTIALS_FILE")
	os.Unsetenv("GOOGLE_ADMIN_EMAIL")
	os.Unsetenv("GOOGLE_DOMAIN")

	_, err := LoadFromEnv()
	if err == nil {
		t.Error("expected error for missing required vars, got nil")
	}
}

func TestValidate_MissingLDAPBaseDN(t *testing.T) {
	cfg := &Config{
		LDAP: LDAPConfig{
			Host: "localhost",
		},
		Google: GoogleConfig{
			CredentialsFile: "/tmp/creds.json",
			AdminEmail:      "admin@test.com",
			Domain:          "test.com",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for missing LDAP_BASE_DN")
	}
}

func TestValidate_MissingGoogleConfig(t *testing.T) {
	cfg := &Config{
		LDAP: LDAPConfig{
			Host:   "localhost",
			BaseDN: "dc=test,dc=com",
		},
		Google: GoogleConfig{},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for missing Google config")
	}
}

func TestValidate_AllPresent(t *testing.T) {
	cfg := &Config{
		LDAP: LDAPConfig{
			Host:   "localhost",
			BaseDN: "dc=test,dc=com",
		},
		Google: GoogleConfig{
			CredentialsFile: "/tmp/creds.json",
			AdminEmail:      "admin@test.com",
			Domain:          "test.com",
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	os.Setenv("TEST_VAR", "custom_value")
	defer os.Unsetenv("TEST_VAR")

	if v := getEnvOrDefault("TEST_VAR", "default"); v != "custom_value" {
		t.Errorf("expected custom_value, got %s", v)
	}

	if v := getEnvOrDefault("NONEXISTENT_VAR", "default"); v != "default" {
		t.Errorf("expected default, got %s", v)
	}
}

func TestGetEnvAsInt(t *testing.T) {
	os.Setenv("TEST_INT", "123")
	defer os.Unsetenv("TEST_INT")

	if v := getEnvAsInt("TEST_INT", 0); v != 123 {
		t.Errorf("expected 123, got %d", v)
	}

	if v := getEnvAsInt("NONEXISTENT_INT", 456); v != 456 {
		t.Errorf("expected 456, got %d", v)
	}

	os.Setenv("TEST_INVALID_INT", "notanumber")
	defer os.Unsetenv("TEST_INVALID_INT")

	if v := getEnvAsInt("TEST_INVALID_INT", 789); v != 789 {
		t.Errorf("expected 789 for invalid int, got %d", v)
	}
}

func TestGetEnvAsBool(t *testing.T) {
	tests := []struct {
		envValue string
		expected bool
	}{
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"1", true},
		{"false", false},
		{"False", false},
		{"FALSE", false},
		{"0", false},
	}

	for _, tt := range tests {
		os.Setenv("TEST_BOOL", tt.envValue)
		if v := getEnvAsBool("TEST_BOOL", !tt.expected); v != tt.expected {
			t.Errorf("for %s: expected %v, got %v", tt.envValue, tt.expected, v)
		}
	}
	os.Unsetenv("TEST_BOOL")

	// Test default
	if v := getEnvAsBool("NONEXISTENT_BOOL", true); v != true {
		t.Errorf("expected true as default, got %v", v)
	}

	// Test invalid value falls back to default
	os.Setenv("TEST_INVALID_BOOL", "notabool")
	defer os.Unsetenv("TEST_INVALID_BOOL")
	if v := getEnvAsBool("TEST_INVALID_BOOL", true); v != true {
		t.Errorf("expected true for invalid bool, got %v", v)
	}
}
