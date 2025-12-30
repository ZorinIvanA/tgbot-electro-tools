package fsm

import (
	"regexp"
	"strings"
)

// State represents FSM state
type State string

const (
	StateIdle                 State = "idle"
	StateUSHMNotStartingStep1 State = "ushm_not_starting_step1"
	StateUSHMNotStartingStep2 State = "ushm_not_starting_step2"
	StateAwaitingEmail        State = "awaiting_email"
	StateAwaitingEmailConsent State = "awaiting_email_consent"
	StateOfferingSiteLink     State = "offering_site_link"
)

// FSM represents the finite state machine
type FSM struct {
	currentState State
}

// NewFSM creates a new FSM instance
func NewFSM(initialState string) *FSM {
	if initialState == "" {
		initialState = string(StateIdle)
	}
	return &FSM{
		currentState: State(initialState),
	}
}

// GetState returns current state
func (f *FSM) GetState() State {
	return f.currentState
}

// SetState sets the current state
func (f *FSM) SetState(state State) {
	f.currentState = state
}

// ProcessMessage processes incoming message and returns response and new state
func (f *FSM) ProcessMessage(message string) (response string, newState State, handled bool) {
	messageLower := strings.ToLower(strings.TrimSpace(message))

	// Check for diagnostic triggers when in idle state
	if f.currentState == StateIdle {
		if containsUSHMTrigger(messageLower) {
			return GetUSHMStep1Response(), StateUSHMNotStartingStep1, true
		}
	}

	// Handle FSM states
	switch f.currentState {
	case StateUSHMNotStartingStep1:
		return GetUSHMStep2Response(), StateUSHMNotStartingStep2, true

	case StateUSHMNotStartingStep2:
		// After step 2, return to idle
		response := GetUSHMFinalResponse()
		return response, StateIdle, true

	case StateAwaitingEmail:
		// Validate email
		if IsValidEmail(message) {
			return "Спасибо! Разрешаете ли вы получать технические рекомендации и инструкции по эксплуатации на этот email? Это не реклама.", StateAwaitingEmailConsent, true
		}
		return "Пожалуйста, введите корректный email адрес.", StateAwaitingEmail, true

	case StateOfferingSiteLink:
		// This state is handled by bot logic with buttons
		return "", StateOfferingSiteLink, false

	case StateAwaitingEmailConsent:
		// This state is handled by bot logic with buttons
		return "", StateAwaitingEmailConsent, false

	default:
		return "", f.currentState, false
	}
}

// containsUSHMTrigger checks if message contains УШМ problem triggers
func containsUSHMTrigger(message string) bool {
	triggers := []string{
		"не включается",
		"не запускается",
		"молчит",
		"не жужжит",
		"не крутит",
	}

	for _, trigger := range triggers {
		if strings.Contains(message, trigger) {
			return true
		}
	}

	return false
}

// IsValidEmail validates email format
func IsValidEmail(email string) bool {
	// Simple email validation regex
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(strings.TrimSpace(email))
}

// GetStartMessage returns the start message
func GetStartMessage() string {
	return "Здравствуйте! Я — технический помощник по электроинструментам. Опишите проблему с вашим устройством."
}

// GetUSHMStep1Response returns first step of УШМ diagnostic
func GetUSHMStep1Response() string {
	return "Понял, проблема с запуском. Что именно происходит? Опишите, пожалуйста, подробнее: устройство совсем не реагирует на нажатие кнопки, или есть какие-то звуки, индикация?"
}

// GetUSHMStep2Response returns second step of УШМ diagnostic
func GetUSHMStep2Response() string {
	return "Давайте попробуем продиагностировать проблему:\n\n" +
		"1. Проверьте, нажимаете ли вы рычажок предохранителя (обычно находится на корпусе)\n" +
		"2. Убедитесь, что розетка работает (проверьте другим устройством)\n" +
		"3. Осмотрите кабель на наличие повреждений\n" +
		"4. Если есть кнопка блокировки шпинделя - убедитесь, что она не зажата\n\n" +
		"Проверьте эти моменты и напишите результат."
}

// GetUSHMFinalResponse returns final response for УШМ diagnostic
func GetUSHMFinalResponse() string {
	return "Если эти действия не помогли, возможно, требуется диагностика щёток, кнопки включения или обмотки двигателя. " +
		"В этом случае рекомендую обратиться в сервисный центр.\n\n" +
		"Чем ещё могу помочь?"
}

// GetSiteLinkOfferMessage returns the message offering site link
func GetSiteLinkOfferMessage() string {
	return "Хотите подробнее ознакомиться с инструкциями и рекомендациями по эксплуатации? Перейти на сайт?"
}

// GetEmailRequestMessage returns message requesting email
func GetEmailRequestMessage() string {
	return "Отлично! Пожалуйста, укажите ваш email адрес для получения полезной информации об эксплуатации электроинструментов."
}

// GetEmailConsentMessage returns message for email consent
func GetEmailConsentMessage() string {
	return "Разрешаете ли вы получать технические рекомендации и инструкции по эксплуатации на этот email? Это не реклама."
}

// GetEmailSavedMessage returns message after email is saved
func GetEmailSavedMessage(siteURL string) string {
	return "Спасибо! Информация сохранена.\n\nВот ссылка на полезные материалы: " + siteURL
}

// GetEmailDeclinedMessage returns message when user declines email consent
func GetEmailDeclinedMessage(siteURL string) string {
	return "Понял, не будем использовать ваш email.\n\nВот ссылка на полезные материалы: " + siteURL
}

// GetSiteLinkDeclinedMessage returns message when user declines site link
func GetSiteLinkDeclinedMessage() string {
	return "Хорошо, если что — обращайтесь! Всегда рад помочь."
}

// GetRateLimitMessage returns rate limit exceeded message
func GetRateLimitMessage() string {
	return "Пожалуйста, подождите немного. Вы отправляете сообщения слишком часто."
}
