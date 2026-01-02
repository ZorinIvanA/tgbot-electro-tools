package fsm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Note: Tests are disabled since FSM now uses database-driven scenarios
// TODO: Implement proper integration tests with database setup

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		email    string
		expected bool
	}{
		{"user@example.com", true},
		{"test.email+tag@example.co.uk", true},
		{"user@localhost", false},
		{"", false},
		{"invalid-email", false},
		{"@example.com", false},
		{"user@", false},
		{"user@.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			result := IsValidEmail(tt.email)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsUSHMTrigger(t *testing.T) {
	tests := []struct {
		message  string
		expected bool
	}{
		{"не включается", true},
		{"не запускается", true},
		{"молчит", true},
		{"не жужжит", true},
		{"не крутит", true},
		{"УШМ не работает", false},
		{"hello world", false},
		{"инструмент сломан", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			result := containsUSHMTrigger(tt.message)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetMessages(t *testing.T) {
	// Test that messages are not empty
	assert.NotEmpty(t, GetStartMessage())
	assert.NotEmpty(t, GetUSHMStep1Response())
	assert.NotEmpty(t, GetUSHMStep2Response())
	assert.NotEmpty(t, GetUSHMFinalResponse())
	assert.NotEmpty(t, GetSiteLinkOfferMessage())
	assert.NotEmpty(t, GetEmailRequestMessage())
	assert.NotEmpty(t, GetEmailConsentMessage())
	assert.NotEmpty(t, GetEmailSavedMessage("https://example.com"))
	assert.NotEmpty(t, GetEmailDeclinedMessage("https://example.com"))
	assert.NotEmpty(t, GetSiteLinkDeclinedMessage())
	assert.NotEmpty(t, GetRateLimitMessage())
}
