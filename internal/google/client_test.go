package google

import (
	"testing"

	"github.com/startcodex/ldap-google-sync/internal/config"
)

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"user@example.com", "user@example.com"},
		{"USER@EXAMPLE.COM", "user@example.com"},
		{"User@Example.Com", "user@example.com"},
		{"  user@example.com  ", "user@example.com"},
		{" USER@EXAMPLE.COM ", "user@example.com"},
		{"", ""},
	}

	for _, tt := range tests {
		result := NormalizeEmail(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeEmail(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestNeedsUpdate(t *testing.T) {
	tests := []struct {
		name       string
		ldapUser   config.User
		googleUser config.User
		expected   bool
	}{
		{
			name: "identical users",
			ldapUser: config.User{
				FirstName:  "John",
				LastName:   "Doe",
				Phone:      "123456",
				Department: "Sales",
				Title:      "Manager",
			},
			googleUser: config.User{
				FirstName:  "John",
				LastName:   "Doe",
				Phone:      "123456",
				Department: "Sales",
				Title:      "Manager",
			},
			expected: false,
		},
		{
			name: "different first name",
			ldapUser: config.User{
				FirstName: "John",
				LastName:  "Doe",
			},
			googleUser: config.User{
				FirstName: "Johnny",
				LastName:  "Doe",
			},
			expected: true,
		},
		{
			name: "different last name",
			ldapUser: config.User{
				FirstName: "John",
				LastName:  "Doe",
			},
			googleUser: config.User{
				FirstName: "John",
				LastName:  "Smith",
			},
			expected: true,
		},
		{
			name: "different phone",
			ldapUser: config.User{
				FirstName: "John",
				LastName:  "Doe",
				Phone:     "123456",
			},
			googleUser: config.User{
				FirstName: "John",
				LastName:  "Doe",
				Phone:     "654321",
			},
			expected: true,
		},
		{
			name: "ldap has phone google doesnt",
			ldapUser: config.User{
				FirstName: "John",
				LastName:  "Doe",
				Phone:     "123456",
			},
			googleUser: config.User{
				FirstName: "John",
				LastName:  "Doe",
				Phone:     "",
			},
			expected: true,
		},
		{
			name: "google has phone ldap doesnt - no update needed",
			ldapUser: config.User{
				FirstName: "John",
				LastName:  "Doe",
				Phone:     "",
			},
			googleUser: config.User{
				FirstName: "John",
				LastName:  "Doe",
				Phone:     "123456",
			},
			expected: false,
		},
		{
			name: "different department",
			ldapUser: config.User{
				FirstName:  "John",
				LastName:   "Doe",
				Department: "Sales",
			},
			googleUser: config.User{
				FirstName:  "John",
				LastName:   "Doe",
				Department: "Marketing",
			},
			expected: true,
		},
		{
			name: "different title",
			ldapUser: config.User{
				FirstName: "John",
				LastName:  "Doe",
				Title:     "Manager",
			},
			googleUser: config.User{
				FirstName: "John",
				LastName:  "Doe",
				Title:     "Director",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NeedsUpdate(tt.ldapUser, tt.googleUser)
			if result != tt.expected {
				t.Errorf("NeedsUpdate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGroupNeedsUpdate(t *testing.T) {
	tests := []struct {
		name        string
		ldapGroup   config.Group
		googleGroup config.Group
		expected    bool
	}{
		{
			name: "identical groups",
			ldapGroup: config.Group{
				Name:        "Sales Team",
				Description: "The sales team",
			},
			googleGroup: config.Group{
				Name:        "Sales Team",
				Description: "The sales team",
			},
			expected: false,
		},
		{
			name: "different name",
			ldapGroup: config.Group{
				Name: "Sales Team",
			},
			googleGroup: config.Group{
				Name: "Marketing Team",
			},
			expected: true,
		},
		{
			name: "different description",
			ldapGroup: config.Group{
				Name:        "Sales Team",
				Description: "New description",
			},
			googleGroup: config.Group{
				Name:        "Sales Team",
				Description: "Old description",
			},
			expected: true,
		},
		{
			name: "ldap has description google doesnt",
			ldapGroup: config.Group{
				Name:        "Sales Team",
				Description: "Some description",
			},
			googleGroup: config.Group{
				Name:        "Sales Team",
				Description: "",
			},
			expected: true,
		},
		{
			name: "google has description ldap doesnt - no update",
			ldapGroup: config.Group{
				Name:        "Sales Team",
				Description: "",
			},
			googleGroup: config.Group{
				Name:        "Sales Team",
				Description: "Some description",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GroupNeedsUpdate(tt.ldapGroup, tt.googleGroup)
			if result != tt.expected {
				t.Errorf("GroupNeedsUpdate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGenerateRandomPassword(t *testing.T) {
	// Test password length
	lengths := []int{8, 12, 16, 24, 32}
	for _, length := range lengths {
		password := generateRandomPassword(length)
		if len(password) != length {
			t.Errorf("generateRandomPassword(%d) returned length %d", length, len(password))
		}
	}

	// Test uniqueness (generate multiple passwords, they should be different)
	passwords := make(map[string]bool)
	for i := 0; i < 100; i++ {
		p := generateRandomPassword(16)
		if passwords[p] {
			t.Error("generateRandomPassword generated duplicate password")
		}
		passwords[p] = true
	}

	// Test that password contains expected character types
	password := generateRandomPassword(100)
	hasLower := false
	hasUpper := false
	hasDigit := false
	hasSpecial := false

	for _, c := range password {
		if c >= 'a' && c <= 'z' {
			hasLower = true
		}
		if c >= 'A' && c <= 'Z' {
			hasUpper = true
		}
		if c >= '0' && c <= '9' {
			hasDigit = true
		}
		if c == '!' || c == '@' || c == '#' || c == '$' || c == '%' || c == '^' || c == '&' || c == '*' || c == '(' || c == ')' {
			hasSpecial = true
		}
	}

	// With 100 characters, we should have all types (statistically very likely)
	if !hasLower || !hasUpper || !hasDigit || !hasSpecial {
		t.Logf("Password might be missing character types (could be statistical anomaly)")
	}
}

func TestRandomInt64(t *testing.T) {
	// Test that it returns values within range
	for i := 0; i < 100; i++ {
		max := int64(100)
		result := randomInt64(max)
		if result < 0 || result >= max {
			t.Errorf("randomInt64(%d) = %d, out of range [0, %d)", max, result, max)
		}
	}

	// Test edge cases
	if result := randomInt64(0); result != 0 {
		t.Errorf("randomInt64(0) = %d, want 0", result)
	}

	if result := randomInt64(-1); result != 0 {
		t.Errorf("randomInt64(-1) = %d, want 0", result)
	}

	// Test with max=1 (should always return 0)
	for i := 0; i < 10; i++ {
		if result := randomInt64(1); result != 0 {
			t.Errorf("randomInt64(1) = %d, want 0", result)
		}
	}
}

func TestIsRetryableError(t *testing.T) {
	// Test with nil error
	if isRetryableError(nil) {
		t.Error("isRetryableError(nil) should return false")
	}

	// Test with non-googleapi error
	if isRetryableError(testError{}) {
		t.Error("isRetryableError with non-googleapi error should return false")
	}
}

func TestIsNotFoundError(t *testing.T) {
	// Test with nil error
	if isNotFoundError(nil) {
		t.Error("isNotFoundError(nil) should return false")
	}

	// Test with non-googleapi error
	if isNotFoundError(testError{}) {
		t.Error("isNotFoundError with non-googleapi error should return false")
	}
}

// testError is a simple error type for testing
type testError struct{}

func (e testError) Error() string {
	return "test error"
}
