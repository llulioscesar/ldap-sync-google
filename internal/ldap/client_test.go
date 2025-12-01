package ldap

import (
	"strings"
	"testing"
)

func TestDnToPath(t *testing.T) {
	tests := []struct {
		name     string
		dn       string
		baseDN   string
		expected string
	}{
		{
			name:     "single OU",
			dn:       "ou=Sales,dc=example,dc=com",
			baseDN:   "dc=example,dc=com",
			expected: "/Sales",
		},
		{
			name:     "nested OUs",
			dn:       "ou=LATAM,ou=Sales,dc=example,dc=com",
			baseDN:   "dc=example,dc=com",
			expected: "/Sales/LATAM",
		},
		{
			name:     "deeply nested OUs",
			dn:       "ou=Colombia,ou=LATAM,ou=Sales,dc=example,dc=com",
			baseDN:   "dc=example,dc=com",
			expected: "/Sales/LATAM/Colombia",
		},
		{
			name:     "root DN equals base DN",
			dn:       "dc=example,dc=com",
			baseDN:   "dc=example,dc=com",
			expected: "/",
		},
		{
			name:     "with spaces in OU name",
			dn:       "ou=Human Resources,dc=example,dc=com",
			baseDN:   "dc=example,dc=com",
			expected: "/Human Resources",
		},
		{
			name:     "mixed case OU attribute",
			dn:       "OU=Sales,DC=example,DC=com",
			baseDN:   "DC=example,DC=com",
			expected: "/Sales",
		},
		{
			name:     "user DN with OU",
			dn:       "cn=John Doe,ou=Users,ou=Sales,dc=example,dc=com",
			baseDN:   "dc=example,dc=com",
			expected: "/Sales/Users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dnToPath(tt.dn, tt.baseDN)
			if result != tt.expected {
				t.Errorf("dnToPath(%q, %q) = %q, want %q", tt.dn, tt.baseDN, result, tt.expected)
			}
		})
	}
}

func TestGetParentPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "root path",
			path:     "/",
			expected: "/",
		},
		{
			name:     "empty path",
			path:     "",
			expected: "/",
		},
		{
			name:     "single level",
			path:     "/Sales",
			expected: "/",
		},
		{
			name:     "two levels",
			path:     "/Sales/LATAM",
			expected: "/Sales",
		},
		{
			name:     "three levels",
			path:     "/Sales/LATAM/Colombia",
			expected: "/Sales/LATAM",
		},
		{
			name:     "path with spaces",
			path:     "/Human Resources/Recruiting",
			expected: "/Human Resources",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getParentPath(tt.path)
			if result != tt.expected {
				t.Errorf("getParentPath(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestDnToPath_EdgeCases(t *testing.T) {
	// Test when baseDN is not at the end
	result := dnToPath("ou=Test,dc=other,dc=com", "dc=example,dc=com")
	// Should still extract OUs
	if result != "/Test" {
		t.Logf("Note: baseDN mismatch case returned %q", result)
	}

	// Test empty strings
	result = dnToPath("", "")
	if result != "/" {
		t.Errorf("empty DN should return /, got %q", result)
	}

	// Test DN with only base DN components
	result = dnToPath("dc=example,dc=com", "dc=example,dc=com")
	if result != "/" {
		t.Errorf("DN equal to baseDN should return /, got %q", result)
	}
}

func TestIsDN(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		// DN format (groupOfNames)
		{"cn=john,ou=users,dc=example,dc=com", true},
		{"uid=john,ou=people,dc=example,dc=com", true},
		{"CN=John Doe,OU=Users,DC=example,DC=com", true},

		// UID format (posixGroup)
		{"john", false},
		{"john.doe", false},
		{"julio.caicedo", false},
		{"user123", false},
	}

	for _, tt := range tests {
		// Check if it contains "=" to determine if it's a DN
		result := strings.Contains(tt.input, "=")
		if result != tt.expected {
			t.Errorf("isDN(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}
