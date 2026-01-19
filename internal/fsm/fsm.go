package fsm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/ZorinIvanA/tgbot-electro-tools/internal/storage"
)

// Button represents an inline keyboard button
type Button struct {
	Text         string
	CallbackData string
}

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
	storage       storage.Storage
	openAIEnabled bool
	openAIURL     string
	openAIKey     string
	openAIModel   string
}

// NewFSM creates a new FSM instance
func NewFSM(storage storage.Storage, openAIEnabled bool, openAIURL, openAIKey, openAIModel string) *FSM {
	return &FSM{
		storage:       storage,
		openAIEnabled: openAIEnabled,
		openAIURL:     openAIURL,
		openAIKey:     openAIKey,
		openAIModel:   openAIModel,
	}
}

// ProcessMessage processes incoming message and returns response, buttons and whether it was handled
func (f *FSM) ProcessMessage(userID int64, message string) (response string, buttons []Button, handled bool, err error) {
	// Get user's current session
	session, err := f.storage.GetUserSession(userID)
	if err != nil {
		return "", nil, false, fmt.Errorf("failed to get user session: %w", err)
	}

	// If no active session, check for triggers or use AI if enabled
	if session == nil || session.ScenarioID == nil {
		if f.openAIEnabled {
			// Try to use AI for question recognition
			scenario, err := f.recognizeScenarioWithAI(message)
			if err != nil {
				// Log error but continue with keyword matching
				fmt.Printf("AI recognition failed: %v\n", err)
			} else if scenario != nil {
				// Start scenario
				step, err := f.GetFirstStep(scenario.ID)
				if err != nil {
					return "", nil, false, fmt.Errorf("failed to get first step: %w", err)
				}
				if step != nil {
					err = f.storage.UpdateUserSession(userID, &scenario.ID, &step.StepKey)
					if err != nil {
						return "", nil, false, fmt.Errorf("failed to update session: %w", err)
					}
					buttons = f.GenerateButtonsForStep(step, scenario.ID)
					return step.Message, buttons, true, nil
				}
			}
		}

		// Fallback to keyword matching
		scenario, err := f.storage.GetFSMScenarioByTrigger(message)
		if err != nil {
			return "", nil, false, fmt.Errorf("failed to check triggers: %w", err)
		}
		if scenario != nil {
			step, err := f.GetFirstStep(scenario.ID)
			if err != nil {
				return "", nil, false, fmt.Errorf("failed to get first step: %w", err)
			}
			if step != nil {
				err = f.storage.UpdateUserSession(userID, &scenario.ID, &step.StepKey)
				if err != nil {
					return "", nil, false, fmt.Errorf("failed to update session: %w", err)
				}
				buttons = f.GenerateButtonsForStep(step, scenario.ID)
				return step.Message, buttons, true, nil
			}
		}

		// No scenario triggered
		return "", nil, false, nil
	}

	// Continue existing scenario
	step, err := f.storage.GetFSMScenarioStep(*session.ScenarioID, *session.CurrentStepKey)
	if err != nil {
		return "", nil, false, fmt.Errorf("failed to get current step: %w", err)
	}
	if step == nil {
		// Invalid step, clear session
		f.storage.DeleteUserSession(userID)
		return "", nil, false, nil
	}

	// If this is a final step, clear session and return response
	if step.IsFinal {
		f.storage.DeleteUserSession(userID)
		return step.Message, nil, true, nil
	}

	// Move to next step
	if step.NextStepKey == nil {
		// No next step, clear session
		f.storage.DeleteUserSession(userID)
		return step.Message, nil, true, nil
	}

	nextStep, err := f.storage.GetFSMScenarioStep(*session.ScenarioID, *step.NextStepKey)
	if err != nil {
		return "", nil, false, fmt.Errorf("failed to get next step: %w", err)
	}
	if nextStep == nil {
		// Invalid next step, clear session
		f.storage.DeleteUserSession(userID)
		return step.Message, nil, true, nil
	}

	// Update session with next step
	err = f.storage.UpdateUserSession(userID, session.ScenarioID, &nextStep.StepKey)
	if err != nil {
		return "", nil, false, fmt.Errorf("failed to update session: %w", err)
	}

	buttons = f.GenerateButtonsForStep(nextStep, *session.ScenarioID)
	return nextStep.Message, buttons, true, nil
}

// recognizeScenarioWithAI uses OpenAI-compatible API to recognize the scenario
func (f *FSM) recognizeScenarioWithAI(message string) (*storage.FSMScenario, error) {
	if !f.openAIEnabled || f.openAIKey == "" {
		return nil, nil
	}

	// Get all scenarios
	scenarios, err := f.storage.GetFSMScenarios()
	if err != nil {
		return nil, err
	}

	if len(scenarios) == 0 {
		return nil, nil
	}

	// Prepare scenario descriptions for AI
	var scenarioDescriptions []string
	for _, scenario := range scenarios {
		keywords := strings.Join(scenario.TriggerKeywords, ", ")
		scenarioDescriptions = append(scenarioDescriptions, fmt.Sprintf("%s (%s): %s", scenario.Name, keywords, scenario.Description))
	}

	prompt := fmt.Sprintf(`Пользователь описал проблему с электроинструментом: "%s"

Тебе доступны следующие сценарии диагностики. Выбери ОДИН наиболее подходящий сценарий из списка ниже:

%s

Если проблема не подходит ни под один сценарий, верни {"scenario": null}

Если подходит, верни ТОЛЬКО название сценария в формате JSON: {"scenario": "scenario_name"}`, message, strings.Join(scenarioDescriptions, "\n"))

	requestBody := map[string]interface{}{
		"model": f.openAIModel,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens": 100,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", f.openAIURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+f.openAIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		return nil, err
	}

	if len(apiResponse.Choices) == 0 {
		return nil, nil
	}

	content := strings.TrimSpace(apiResponse.Choices[0].Message.Content)

	var result struct {
		Scenario *string `json:"scenario"`
	}

	err = json.Unmarshal([]byte(content), &result)
	if err != nil {
		return nil, err
	}

	if result.Scenario == nil {
		return nil, nil
	}

	// Find matching scenario
	for _, scenario := range scenarios {
		if scenario.Name == *result.Scenario {
			return scenario, nil
		}
	}

	return nil, nil
}

// GetFirstStep returns the first step of a scenario
func (f *FSM) GetFirstStep(scenarioID int) (*storage.FSMScenarioStep, error) {
	steps, err := f.storage.GetFSMScenarioSteps(scenarioID)
	if err != nil {
		return nil, err
	}

	if len(steps) == 0 {
		return nil, nil
	}

	// Return the first step (assuming steps are ordered by ID)
	return steps[0], nil
}

// GenerateButtonsForStep generates buttons for a given step
func (f *FSM) GenerateButtonsForStep(step *storage.FSMScenarioStep, scenarioID int) []Button {
	// Parse buttons from message content
	buttons := f.parseButtonsFromMessage(step.Message, scenarioID, step.StepKey)

	// If no buttons parsed, check if there are next steps to show as options
	if len(buttons) == 0 {
		buttons = f.getNextStepButtons(scenarioID, step.StepKey)
	}

	return buttons
}

// parseButtonsFromMessage parses buttons from message text
func (f *FSM) parseButtonsFromMessage(message string, scenarioID int, stepKey string) []Button {
	var buttons []Button

	// Look for numbered options like "1. Option", "2. Option"
	lines := strings.Split(message, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "1.") || strings.HasPrefix(line, "2.") || strings.HasPrefix(line, "3.") {
			// Extract option text
			parts := strings.SplitN(line, ".", 2)
			if len(parts) == 2 {
				text := strings.TrimSpace(parts[1])
				callbackData := fmt.Sprintf("option_%d_%s_%s", scenarioID, stepKey, parts[0])
				buttons = append(buttons, Button{
					Text:         text,
					CallbackData: callbackData,
				})
			}
		}
	}

	return buttons
}

// getNextStepButtons gets buttons for next possible steps
func (f *FSM) getNextStepButtons(scenarioID int, currentStepKey string) []Button {
	steps, err := f.storage.GetFSMScenarioSteps(scenarioID)
	if err != nil {
		return nil
	}

	var buttons []Button
	for _, step := range steps {
		if step.StepKey != currentStepKey && step.StepKey != "step1" { // Don't include current or first step
			buttons = append(buttons, Button{
				Text:         step.Message[:min(50, len(step.Message))], // Truncate long messages
				CallbackData: fmt.Sprintf("goto_%d_%s", scenarioID, step.StepKey),
			})
		}
	}

	return buttons
}

// min returns minimum of two ints
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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

// GetScenariosButtons returns buttons for all available scenarios
func (f *FSM) GetScenariosButtons() ([]Button, error) {
	scenarios, err := f.storage.GetFSMScenarios()
	if err != nil {
		return nil, err
	}

	var buttons []Button
	for _, scenario := range scenarios {
		buttons = append(buttons, Button{
			Text:         scenario.Name,
			CallbackData: fmt.Sprintf("start_scenario_%d", scenario.ID),
		})
	}

	return buttons, nil
}
