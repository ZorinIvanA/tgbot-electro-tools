package bot

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/ZorinIvanA/tgbot-electro-tools/internal/fsm"
	"github.com/ZorinIvanA/tgbot-electro-tools/internal/storage"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot represents the Telegram bot
type Bot struct {
	api             *tgbotapi.BotAPI
	storage         storage.Storage
	fsm             *fsm.FSM
	rateLimitPerMin int
}

// NewBot creates a new bot instance
func NewBot(token string, storage storage.Storage, rateLimitPerMin int, openAIEnabled bool, openAIURL, openAIKey, openAIModel string) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API: %w", err)
	}

	log.Printf("Authorized on account %s", api.Self.UserName)

	// Initialize FSM
	fsmInstance := fsm.NewFSM(storage, openAIEnabled, openAIURL, openAIKey, openAIModel)

	return &Bot{
		api:             api,
		storage:         storage,
		fsm:             fsmInstance,
		rateLimitPerMin: rateLimitPerMin,
	}, nil
}

// Start starts the bot
func (b *Bot) Start() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			go b.handleMessage(update.Message)
		} else if update.CallbackQuery != nil {
			go b.handleCallbackQuery(update.CallbackQuery)
		}
	}

	return nil
}

// handleMessage handles incoming messages
func (b *Bot) handleMessage(message *tgbotapi.Message) {
	// Check rate limit
	allowed, err := b.storage.CheckRateLimit(message.From.ID, b.rateLimitPerMin)
	if err != nil {
		log.Printf("Error checking rate limit: %v", err)
		return
	}

	if !allowed {
		msg := tgbotapi.NewMessage(message.Chat.ID, fsm.GetRateLimitMessage())
		b.api.Send(msg)
		return
	}

	// Get or create user
	user, err := b.storage.GetOrCreateUser(message.From.ID)
	if err != nil {
		log.Printf("Error getting/creating user: %v", err)
		return
	}

	// Log incoming message
	if err := b.storage.LogMessage(message.From.ID, message.Text, "incoming"); err != nil {
		log.Printf("Error logging message: %v", err)
	}

	// Handle /start command
	if message.IsCommand() && message.Command() == "start" {
		b.handleStartCommand(message.Chat.ID, user)
		return
	}

	// Increment message count
	if err := b.storage.UpdateUserMessageCount(message.From.ID); err != nil {
		log.Printf("Error updating message count: %v", err)
	}

	// Reload user to get updated message count
	user, err = b.storage.GetUser(message.From.ID)
	if err != nil {
		log.Printf("Error reloading user: %v", err)
		return
	}

	// Get settings
	settings, err := b.storage.GetSettings()
	if err != nil {
		log.Printf("Error getting settings: %v", err)
		return
	}

	// Check if we should offer site link
	if user.MessageCount == settings.TriggerMessageCount && user.FSMState == string(fsm.StateIdle) {
		b.offerSiteLink(message.Chat.ID, user)
		return
	}

	// Process message through FSM
	// Note: FSM now handles its own state management via database
	response, buttons, handled, err := b.fsm.ProcessMessage(message.From.ID, message.Text)
	if err != nil {
		log.Printf("Error processing message through FSM: %v", err)
		return
	}

	// Send response if handled by FSM
	if handled && response != "" {
		msg := tgbotapi.NewMessage(message.Chat.ID, response)

		// Add inline keyboard if buttons are provided
		if len(buttons) > 0 {
			keyboard := b.createInlineKeyboard(buttons)
			msg.ReplyMarkup = keyboard
		}

		sentMsg, err := b.api.Send(msg)
		if err != nil {
			log.Printf("Error sending message: %v", err)
			return
		}

		// Log outgoing message
		if err := b.storage.LogMessage(message.From.ID, sentMsg.Text, "outgoing"); err != nil {
			log.Printf("Error logging outgoing message: %v", err)
		}
	} else if !handled {
		// If not handled, send generic response
		genericResponse := "Я вас понял. Если возникнут проблемы с электроинструментом, опишите их подробнее, и я постараюсь помочь!"
		msg := tgbotapi.NewMessage(message.Chat.ID, genericResponse)
		sentMsg, err := b.api.Send(msg)
		if err != nil {
			log.Printf("Error sending message: %v", err)
			return
		}

		// Log outgoing message
		if err := b.storage.LogMessage(message.From.ID, sentMsg.Text, "outgoing"); err != nil {
			log.Printf("Error logging outgoing message: %v", err)
		}
	}
}

// handleStartCommand handles /start command
func (b *Bot) handleStartCommand(chatID int64, user *storage.User) {
	// Reset user state to idle
	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateIdle)); err != nil {
		log.Printf("Error resetting FSM state: %v", err)
	}

	// Clear user session
	if err := b.storage.DeleteUserSession(user.TelegramID); err != nil {
		log.Printf("Error clearing user session: %v", err)
	}

	msg := tgbotapi.NewMessage(chatID, fsm.GetStartMessage())

	// Get scenarios buttons
	scenariosButtons, err := b.fsm.GetScenariosButtons()
	if err != nil {
		log.Printf("Error getting scenarios buttons: %v", err)
	} else if len(scenariosButtons) > 0 {
		keyboard := b.createInlineKeyboard(scenariosButtons)
		msg.ReplyMarkup = keyboard
	}

	sentMsg, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Error sending start message: %v", err)
		return
	}

	// Log outgoing message
	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message: %v", err)
	}
}

// offerSiteLink offers site link to the user
func (b *Bot) offerSiteLink(chatID int64, user *storage.User) {
	// Update FSM state
	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateOfferingSiteLink)); err != nil {
		log.Printf("Error updating FSM state: %v", err)
	}

	msg := tgbotapi.NewMessage(chatID, fsm.GetSiteLinkOfferMessage())

	// Add inline keyboard with Yes/No buttons
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Да", "site_link_yes"),
			tgbotapi.NewInlineKeyboardButtonData("Нет", "site_link_no"),
		),
	)
	msg.ReplyMarkup = keyboard

	sentMsg, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Error sending site link offer: %v", err)
		return
	}

	// Log outgoing message
	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message: %v", err)
	}
}

// handleCallbackQuery handles button callbacks
func (b *Bot) handleCallbackQuery(query *tgbotapi.CallbackQuery) {
	// Get user
	user, err := b.storage.GetUser(query.From.ID)
	if err != nil {
		log.Printf("Error getting user: %v", err)
		return
	}

	if user == nil {
		log.Printf("User not found for callback query")
		return
	}

	// Get settings
	settings, err := b.storage.GetSettings()
	if err != nil {
		log.Printf("Error getting settings: %v", err)
		return
	}

	// Answer callback query to remove loading state
	callback := tgbotapi.NewCallback(query.ID, "")
	b.api.Request(callback)

	switch query.Data {
	case "site_link_yes":
		b.handleSiteLinkYes(query, user)
	case "site_link_no":
		b.handleSiteLinkNo(query, user)
	case "email_consent_yes":
		b.handleEmailConsentYes(query, user, settings)
	case "email_consent_no":
		b.handleEmailConsentNo(query, user, settings)
	default:
		if strings.HasPrefix(query.Data, "email_confirm_") {
			b.handleEmailConfirm(query, user)
		} else if strings.HasPrefix(query.Data, "start_scenario_") {
			b.handleStartScenario(query, user)
		} else if strings.HasPrefix(query.Data, "option_") {
			b.handleOption(query, user)
		} else if strings.HasPrefix(query.Data, "goto_") {
			b.handleGoto(query, user)
		} else if strings.HasPrefix(query.Data, "action_") {
			b.handleAction(query, user)
		} else {
			log.Printf("Unknown callback data: %s", query.Data)
		}
	}
}

// handleSiteLinkYes handles user accepting site link offer
func (b *Bot) handleSiteLinkYes(query *tgbotapi.CallbackQuery, user *storage.User) {
	// User wants to visit site, request email
	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateAwaitingEmail)); err != nil {
		log.Printf("Error updating FSM state: %v", err)
	}

	msg := tgbotapi.NewMessage(query.Message.Chat.ID, fsm.GetEmailRequestMessage())
	sentMsg, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Error sending email request: %v", err)
		return
	}

	// Log outgoing message
	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message: %v", err)
	}
}

// handleSiteLinkNo handles user declining site link offer
func (b *Bot) handleSiteLinkNo(query *tgbotapi.CallbackQuery, user *storage.User) {
	// User declined site link
	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateIdle)); err != nil {
		log.Printf("Error updating FSM state: %v", err)
	}

	msg := tgbotapi.NewMessage(query.Message.Chat.ID, fsm.GetSiteLinkDeclinedMessage())
	sentMsg, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Error sending decline message: %v", err)
		return
	}

	// Log outgoing message
	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message: %v", err)
	}
}

// handleEmailConsentYes handles user granting email consent
func (b *Bot) handleEmailConsentYes(query *tgbotapi.CallbackQuery, user *storage.User, settings *storage.Settings) {
	// User granted email consent
	// Since we're in awaiting_email_consent state, the email was already validated
	// We just need to update consent
	if user.Email != "" {
		if err := b.storage.UpdateUserEmail(user.TelegramID, user.Email, true); err != nil {
			log.Printf("Error updating user email consent: %v", err)
		}
	}

	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateIdle)); err != nil {
		log.Printf("Error updating FSM state: %v", err)
	}

	msg := tgbotapi.NewMessage(query.Message.Chat.ID, fsm.GetEmailSavedMessage(settings.SiteURL))
	sentMsg, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Error sending confirmation: %v", err)
		return
	}

	// Log outgoing message
	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message: %v", err)
	}
}

// handleEmailConsentNo handles user declining email consent
func (b *Bot) handleEmailConsentNo(query *tgbotapi.CallbackQuery, user *storage.User, settings *storage.Settings) {
	// User declined email consent
	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateIdle)); err != nil {
		log.Printf("Error updating FSM state: %v", err)
	}

	msg := tgbotapi.NewMessage(query.Message.Chat.ID, fsm.GetEmailDeclinedMessage(settings.SiteURL))
	sentMsg, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Error sending decline message: %v", err)
		return
	}

	// Log outgoing message
	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message: %v", err)
	}
}

// handleEmailConfirm handles email confirmation buttons
func (b *Bot) handleEmailConfirm(query *tgbotapi.CallbackQuery, user *storage.User) {
	email := strings.TrimPrefix(query.Data, "email_confirm_")

	// Save email temporarily and show consent buttons
	if err := b.storage.UpdateUserEmail(user.TelegramID, email, false); err != nil {
		log.Printf("Error saving email: %v", err)
	}

	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateAwaitingEmailConsent)); err != nil {
		log.Printf("Error updating FSM state: %v", err)
	}

	msg := tgbotapi.NewMessage(query.Message.Chat.ID, fsm.GetEmailConsentMessage())

	// Add consent buttons
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Разрешаю", "email_consent_yes"),
			tgbotapi.NewInlineKeyboardButtonData("Нет, спасибо", "email_consent_no"),
		),
	)
	msg.ReplyMarkup = keyboard

	sentMsg, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Error sending consent request: %v", err)
		return
	}

	// Log outgoing message
	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message: %v", err)
	}
}

// handleStartScenario handles scenario start
func (b *Bot) handleStartScenario(query *tgbotapi.CallbackQuery, user *storage.User) {
	scenarioIDStr := strings.TrimPrefix(query.Data, "start_scenario_")
	scenarioID, err := strconv.Atoi(scenarioIDStr)
	if err != nil {
		log.Printf("Error parsing scenario ID: %v", err)
		return
	}

	// Get scenario
	scenario, err := b.storage.GetFSMScenario(scenarioID)
	if err != nil {
		log.Printf("Error getting scenario: %v", err)
		return
	}
	if scenario == nil {
		log.Printf("Scenario not found: %d", scenarioID)
		return
	}

	// Start scenario
	step, err := b.fsm.GetFirstStep(scenarioID)
	if err != nil {
		log.Printf("Error getting first step: %v", err)
		return
	}
	if step != nil {
		err = b.storage.UpdateUserSession(user.TelegramID, &scenarioID, &step.StepKey)
		if err != nil {
			log.Printf("Error updating session: %v", err)
			return
		}

		buttons := b.fsm.GenerateButtonsForStep(step, scenarioID)
		msg := tgbotapi.NewMessage(query.Message.Chat.ID, step.Message)
		if len(buttons) > 0 {
			keyboard := b.createInlineKeyboard(buttons)
			msg.ReplyMarkup = keyboard
		}

		sentMsg, err := b.api.Send(msg)
		if err != nil {
			log.Printf("Error sending scenario start message: %v", err)
			return
		}

		// Log outgoing message
		if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
			log.Printf("Error logging outgoing message: %v", err)
		}
	}
}

// handleOption handles option selection
func (b *Bot) handleOption(query *tgbotapi.CallbackQuery, user *storage.User) {
	parts := strings.Split(query.Data, "_")
	if len(parts) >= 4 {
		scenarioID, _ := strconv.Atoi(parts[1])
		stepKey := parts[2]
		optionNum := parts[3]

		// Find next step based on option
		steps, err := b.storage.GetFSMScenarioSteps(scenarioID)
		if err != nil {
			log.Printf("Error getting steps: %v", err)
			return
		}

		var nextStep *storage.FSMScenarioStep
		for _, step := range steps {
			if step.StepKey == stepKey+"_"+optionNum {
				nextStep = step
				break
			}
		}

		if nextStep != nil {
			err = b.storage.UpdateUserSession(user.TelegramID, &scenarioID, &nextStep.StepKey)
			if err != nil {
				log.Printf("Error updating session: %v", err)
				return
			}

			buttons := b.fsm.GenerateButtonsForStep(nextStep, scenarioID)
			msg := tgbotapi.NewMessage(query.Message.Chat.ID, nextStep.Message)
			if len(buttons) > 0 {
				keyboard := b.createInlineKeyboard(buttons)
				msg.ReplyMarkup = keyboard
			}

			sentMsg, err := b.api.Send(msg)
			if err != nil {
				log.Printf("Error sending option response: %v", err)
				return
			}

			// Log outgoing message
			if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
				log.Printf("Error logging outgoing message: %v", err)
			}
		}
	}
}

// handleGoto handles goto step
func (b *Bot) handleGoto(query *tgbotapi.CallbackQuery, user *storage.User) {
	parts := strings.Split(query.Data, "_")
	if len(parts) >= 3 {
		scenarioID, _ := strconv.Atoi(parts[1])
		stepKey := parts[2]

		step, err := b.storage.GetFSMScenarioStep(scenarioID, stepKey)
		if err != nil {
			log.Printf("Error getting step: %v", err)
			return
		}
		if step != nil {
			err = b.storage.UpdateUserSession(user.TelegramID, &scenarioID, &step.StepKey)
			if err != nil {
				log.Printf("Error updating session: %v", err)
				return
			}

			buttons := b.fsm.GenerateButtonsForStep(step, scenarioID)
			msg := tgbotapi.NewMessage(query.Message.Chat.ID, step.Message)
			if len(buttons) > 0 {
				keyboard := b.createInlineKeyboard(buttons)
				msg.ReplyMarkup = keyboard
			}

			sentMsg, err := b.api.Send(msg)
			if err != nil {
				log.Printf("Error sending goto response: %v", err)
				return
			}

			// Log outgoing message
			if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
				log.Printf("Error logging outgoing message: %v", err)
			}
		}
	}
}

// ShouldOfferSiteLink checks if bot should offer site link based on message count
func ShouldOfferSiteLink(messageCount int, triggerCount int, currentState string) bool {
	return messageCount == triggerCount && currentState == string(fsm.StateIdle)
}

// Stop stops the bot
func (b *Bot) Stop() {
	b.api.StopReceivingUpdates()
}

// GetUsername returns bot username
func (b *Bot) GetUsername() string {
	return b.api.Self.UserName
}

// createInlineKeyboard creates inline keyboard from buttons
func (b *Bot) createInlineKeyboard(buttons []fsm.Button) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	// Group buttons into rows of 2
	for i := 0; i < len(buttons); i += 2 {
		var row []tgbotapi.InlineKeyboardButton
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(buttons[i].Text, buttons[i].CallbackData))
		if i+1 < len(buttons) {
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(buttons[i+1].Text, buttons[i+1].CallbackData))
		}
		rows = append(rows, row)
	}

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// handleAction handles action button clicks
func (b *Bot) handleAction(query *tgbotapi.CallbackQuery, user *storage.User) {
	parts := strings.Split(query.Data, "_")
	if len(parts) >= 3 {
		scenarioID, _ := strconv.Atoi(parts[1])
		actionKey := parts[2]

		step, err := b.storage.GetFSMScenarioStep(scenarioID, actionKey)
		if err != nil {
			log.Printf("Error getting action step: %v", err)
			return
		}
		if step != nil {
			err = b.storage.UpdateUserSession(user.TelegramID, &scenarioID, &step.StepKey)
			if err != nil {
				log.Printf("Error updating session: %v", err)
				return
			}

			buttons := b.fsm.GenerateButtonsForStep(step, scenarioID)
			msg := tgbotapi.NewMessage(query.Message.Chat.ID, step.Message)
			if len(buttons) > 0 {
				keyboard := b.createInlineKeyboard(buttons)
				msg.ReplyMarkup = keyboard
			}

			sentMsg, err := b.api.Send(msg)
			if err != nil {
				log.Printf("Error sending action response: %v", err)
				return
			}

			// Log outgoing message
			if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
				log.Printf("Error logging outgoing message: %v", err)
			}
		}
	}
}

// GetUserIDFromString converts string to user ID
func GetUserIDFromString(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
