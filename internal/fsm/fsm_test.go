package fsm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFSM(t *testing.T) {
	fsm := NewFSM("")
	assert.Equal(t, State(StateIdle), fsm.GetState())

	fsm2 := NewFSM("idle")
	assert.Equal(t, State(StateIdle), fsm2.GetState())
}

func TestFSM_ProcessMessage(t *testing.T) {
	tests := []struct {
		name            string
		initialState    State
		message         string
		expectedState   State
		expectedResp    string
		expectedHandled bool
	}{
		{
			name:            "idle state with no trigger",
			initialState:    StateIdle,
			message:         "hello",
			expectedState:   StateIdle,
			expectedResp:    "",
			expectedHandled: false,
		},
		{
			name:            "idle state with УШМ trigger",
			initialState:    StateIdle,
			message:         "моя ушм не включается",
			expectedState:   StateUSHMNotStartingStep1,
			expectedResp:    GetUSHMStep1Response(),
			expectedHandled: true,
		},
		{
			name:            "ушм step1 to step2",
			initialState:    StateUSHMNotStartingStep1,
			message:         "да",
			expectedState:   StateUSHMNotStartingStep2,
			expectedResp:    GetUSHMStep2Response(),
			expectedHandled: true,
		},
		{
			name:            "ушм step2 to idle",
			initialState:    StateUSHMNotStartingStep2,
			message:         "проверил",
			expectedState:   StateIdle,
			expectedResp:    GetUSHMFinalResponse(),
			expectedHandled: true,
		},
		{
			name:            "awaiting email with valid email",
			initialState:    StateAwaitingEmail,
			message:         "user@example.com",
			expectedState:   StateAwaitingEmailConsent,
			expectedResp:    "Спасибо! Разрешаете ли вы получать технические рекомендации и инструкции по эксплуатации на этот email? Это не реклама.",
			expectedHandled: true,
		},
		{
			name:            "awaiting email with invalid email",
			initialState:    StateAwaitingEmail,
			message:         "invalid-email",
			expectedState:   StateAwaitingEmail,
			expectedResp:    "Пожалуйста, введите корректный email адрес.",
			expectedHandled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsm := NewFSM(string(tt.initialState))
			resp, newState, handled := fsm.ProcessMessage(tt.message)

			assert.Equal(t, tt.expectedState, newState)
			assert.Equal(t, tt.expectedResp, resp)
			assert.Equal(t, tt.expectedHandled, handled)
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

func TestFSM_SetState(t *testing.T) {
	fsm := NewFSM("")
	fsm.SetState(StateAwaitingEmail)
	assert.Equal(t, StateAwaitingEmail, fsm.GetState())
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
