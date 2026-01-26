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
	log.Printf("DEBUG: handleMessage called for user %d with text: '%s'", message.From.ID, message.Text)

	// Check rate limit
	allowed, err := b.storage.CheckRateLimit(message.From.ID, b.rateLimitPerMin)
	if err != nil {
		log.Printf("Error checking rate limit for user %d: %v", message.From.ID, err)
		return
	}

	if !allowed {
		log.Printf("Rate limit exceeded for user %d", message.From.ID)
		msg := tgbotapi.NewMessage(message.Chat.ID, fsm.GetRateLimitMessage())
		b.api.Send(msg)
		return
	}

	// Get or create user
	user, err := b.storage.GetOrCreateUser(message.From.ID)
	if err != nil {
		log.Printf("Error getting/creating user %d: %v", message.From.ID, err)
		return
	}
	log.Printf("DEBUG: GetOrCreateUser for user %d: MessageCount=%d, FSMState=%s", user.TelegramID, user.MessageCount, user.FSMState)

	// Log incoming message
	if err := b.storage.LogMessage(message.From.ID, message.Text, "incoming"); err != nil {
		log.Printf("Error logging incoming message for user %d: %v", message.From.ID, err)
	}

	// Handle /start command
	if message.IsCommand() && message.Command() == "start" {
		log.Printf("DEBUG: Handling /start command for user %d", message.From.ID)
		b.handleStartCommand(message.Chat.ID, user)
		return
	}

	// Increment message count
	log.Printf("DEBUG: Attempting to increment message count for user %d", message.From.ID)
	if err := b.storage.UpdateUserMessageCount(message.From.ID); err != nil {
		log.Printf("Error updating message count for user %d: %v", message.From.ID, err)
	} else {
		log.Printf("DEBUG: Successfully called UpdateUserMessageCount for user %d", message.From.ID)
	}

	// Reload user to get updated message count
	log.Printf("DEBUG: Reloading user %d to get updated message count", message.From.ID)
	user, err = b.storage.GetUser(message.From.ID)
	if err != nil {
		log.Printf("Error reloading user %d: %v", message.From.ID, err)
		return
	}
	if user != nil {
		log.Printf("DEBUG: Reloaded user %d: MessageCount=%d, FSMState=%s", user.TelegramID, user.MessageCount, user.FSMState)
	} else {
		log.Printf("ERROR: Failed to reload user %d, got nil", message.From.ID)
		return
	}


	// Get settings
	settings, err := b.storage.GetSettings()
	if err != nil {
		log.Printf("Error getting settings for user %d: %v", message.From.ID, err)
		return
	}
	log.Printf("DEBUG: Settings for user %d: TriggerMessageCount=%d", message.From.ID, settings.TriggerMessageCount)

	// Check if we should offer site link
	log.Printf("DEBUG: Checking if user %d should be offered site link. MessageCount: %d, TriggerMessageCount: %d, FSMState: %s", user.TelegramID, user.MessageCount, settings.TriggerMessageCount, user.FSMState)
	if user.MessageCount == settings.TriggerMessageCount && user.FSMState == string(fsm.StateIdle) {
		log.Printf("DEBUG: Offering site link to user %d", user.TelegramID)
		b.offerSiteLink(message.Chat.ID, user)
		return
	}

	// Process message through FSM
	// Note: FSM now handles its own state management via database
	log.Printf("DEBUG: Processing message '%s' for user %d through FSM", message.Text, message.From.ID)
	response, buttons, handled, err := b.fsm.ProcessMessage(message.From.ID, message.Text)
	if err != nil {
		log.Printf("Error processing message through FSM for user %d: %v", message.From.ID, err)
		return
	}
	log.Printf("DEBUG: FSM processing for user %d: Handled=%v, Response: '%s'", message.From.ID, handled, response)

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
			log.Printf("Error sending message for user %d: %v", message.From.ID, err)
			return
		}

		// Log outgoing message
		if err := b.storage.LogMessage(message.From.ID, sentMsg.Text, "outgoing"); err != nil {
			log.Printf("Error logging outgoing message for user %d: %v", message.From.ID, err)
		}
	} else if !handled {
		log.Printf("DEBUG: FSM did not handle message for user %d, sending generic response", message.From.ID)
		// If not handled, send generic response
		genericResponse := "Я вас понял. Если возникнут проблемы с электроинструментом, опишите их подробнее, и я постараюсь помочь!"
		msg := tgbotapi.NewMessage(message.Chat.ID, genericResponse)
		sentMsg, err := b.api.Send(msg)
		if err != nil {
			log.Printf("Error sending generic response for user %d: %v", message.From.ID, err)
			return
		}

		// Log outgoing message
		if err := b.storage.LogMessage(message.From.ID, sentMsg.Text, "outgoing"); err != nil {
			log.Printf("Error logging generic response for user %d: %v", message.From.ID, err)
		}
	}
	log.Printf("DEBUG: handleMessage finished for user %d", message.From.ID)
}

// handleStartCommand handles /start command
func (b *Bot) handleStartCommand(chatID int64, user *storage.User) {
	log.Printf("DEBUG: handleStartCommand called for user %d", user.TelegramID)
	// Reset user state to idle
	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateIdle)); err != nil {
		log.Printf("Error resetting FSM state for user %d: %v", user.TelegramID, err)
	}

	// Clear user session
	if err := b.storage.DeleteUserSession(user.TelegramID); err != nil {
		log.Printf("Error clearing user session for user %d: %v", user.TelegramID, err)
	}

	msg := tgbotapi.NewMessage(chatID, fsm.GetStartMessage())

	// Get scenarios buttons
	scenariosButtons, err := b.fsm.GetScenariosButtons()
	if err != nil {
		log.Printf("Error getting scenarios buttons for user %d: %v", user.TelegramID, err)
	} else if len(scenariosButtons) > 0 {
		keyboard := b.createInlineKeyboard(scenariosButtons)
		msg.ReplyMarkup = keyboard
	}

	sentMsg, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Error sending start message for user %d: %v", user.TelegramID, err)
		return
	}

	// Log outgoing message
	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
	}
	log.Printf("DEBUG: handleStartCommand finished for user %d", user.TelegramID)
}

// offerSiteLink offers site link to the user
func (b *Bot) offerSiteLink(chatID int64, user *storage.User) {
	log.Printf("DEBUG: offerSiteLink called for user %d", user.TelegramID)
	// Update FSM state
	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateOfferingSiteLink)); err != nil {
		log.Printf("Error updating FSM state to OfferingSiteLink for user %d: %v", user.TelegramID, err)
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
		log.Printf("Error sending site link offer for user %d: %v", user.TelegramID, err)
		return
	}

	// Log outgoing message
	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
	}
	log.Printf("DEBUG: offerSiteLink finished for user %d", user.TelegramID)
}

// handleCallbackQuery handles button callbacks
func (b *Bot) handleCallbackQuery(query *tgbotapi.CallbackQuery) {
	log.Printf("DEBUG: handleCallbackQuery called for user %d with data: %s", query.From.ID, query.Data)
	// Get user
	user, err := b.storage.GetUser(query.From.ID)
	if err != nil {
		log.Printf("Error getting user %d for callback query: %v", query.From.ID, err)
		return
	}

	if user == nil {
		log.Printf("User %d not found for callback query", query.From.ID)
		return
	}
	log.Printf("DEBUG: User %d found for callback. MessageCount=%d, FSMState=%s", user.TelegramID, user.MessageCount, user.FSMState)


	// Get settings
	settings, err := b.storage.GetSettings()
	if err != nil {
		log.Printf("Error getting settings for user %d during callback: %v", query.From.ID, err)
		return
	}
	log.Printf("DEBUG: Settings for user %d during callback: TriggerMessageCount=%d", query.From.ID, settings.TriggerMessageCount)


	// Answer callback query to remove loading state
	callback := tgbotapi.NewCallback(query.ID, "")
	b.api.Request(callback)

	switch query.Data {
	case "site_link_yes":
		log.Printf("DEBUG: Handling site_link_yes for user %d", query.From.ID)
		b.handleSiteLinkYes(query, user)
	case "site_link_no":
		log.Printf("DEBUG: Handling site_link_no for user %d", query.From.ID)
		b.handleSiteLinkNo(query, user)
	case "site_link_yes_post":
		log.Printf("DEBUG: Handling site_link_yes_post for user %d", query.From.ID)
		b.handleSiteLinkYesPost(query, user, settings)
	case "site_link_no_post":
		log.Printf("DEBUG: Handling site_link_no_post for user %d", query.From.ID)
		b.handleSiteLinkNoPost(query, user, settings)
	case "email_consent_yes":
		log.Printf("DEBUG: Handling email_consent_yes for user %d", query.From.ID)
		b.handleEmailConsentYes(query, user, settings)
	case "email_consent_no":
		log.Printf("DEBUG: Handling email_consent_no for user %d", query.From.ID)
		b.handleEmailConsentNo(query, user, settings)
	default:
		if strings.HasPrefix(query.Data, "email_confirm_") {
			log.Printf("DEBUG: Handling email_confirm_ for user %d", query.From.ID)
			b.handleEmailConfirm(query, user)
		} else if strings.HasPrefix(query.Data, "start_scenario_") {
			log.Printf("DEBUG: Handling start_scenario_ for user %d", query.From.ID)
			b.handleStartScenario(query, user)
		} else if strings.HasPrefix(query.Data, "option_") {
			log.Printf("DEBUG: Handling option_ for user %d", query.From.ID)
			b.handleOption(query, user)
		} else if strings.HasPrefix(query.Data, "goto_") {
			log.Printf("DEBUG: Handling goto_ for user %d", query.From.ID)
			b.handleGoto(query, user)
		} else if strings.HasPrefix(query.Data, "action_") {
			log.Printf("DEBUG: Handling action_ for user %d", query.From.ID)
			b.handleAction(query, user)
		} else if strings.HasPrefix(query.Data, "back_") {
			log.Printf("DEBUG: Handling back_ for user %d", query.From.ID)
			b.handleBack(query, user)
		} else {
			log.Printf("Unknown callback data for user %d: %s", query.From.ID, query.Data)
		}
	}
	log.Printf("DEBUG: handleCallbackQuery finished for user %d", query.From.ID)
}

// handleSiteLinkYes handles user accepting site link offer
func (b *Bot) handleSiteLinkYes(query *tgbotapi.CallbackQuery, user *storage.User) {
	log.Printf("DEBUG: handleSiteLinkYes called for user %d", user.TelegramID)
	// User wants to visit site, request email
	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateAwaitingEmail)); err != nil {
		log.Printf("Error updating FSM state to AwaitingEmail for user %d: %v", user.TelegramID, err)
	}

	msg := tgbotapi.NewMessage(query.Message.Chat.ID, fsm.GetEmailRequestMessage())
	sentMsg, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Error sending email request for user %d: %v", user.TelegramID, err)
		return
	}

	// Log outgoing message
	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
	}
	log.Printf("DEBUG: handleSiteLinkYes finished for user %d", user.TelegramID)
}

// handleSiteLinkNo handles user declining site link offer
func (b *Bot) handleSiteLinkNo(query *tgbotapi.CallbackQuery, user *storage.User) {
	log.Printf("DEBUG: handleSiteLinkNo called for user %d. Current MessageCount: %d", user.TelegramID, user.MessageCount)
	// User declined site link, reset message count and FSM state
	if err := b.storage.ResetUserMessageCount(user.TelegramID); err != nil {
		log.Printf("Error resetting user message count for user %d: %v", user.TelegramID, err)
	} else {
		log.Printf("DEBUG: Called ResetUserMessageCount for user %d", user.TelegramID)
	}

	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateIdle)); err != nil {
		log.Printf("Error updating FSM state to Idle for user %d: %v", user.TelegramID, err)
	}

	msg := tgbotapi.NewMessage(query.Message.Chat.ID, fsm.GetSiteLinkDeclinedMessage())
	sentMsg, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Error sending decline message for user %d: %v", user.TelegramID, err)
		return
	}

	// Log outgoing message
	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
	}
	log.Printf("DEBUG: handleSiteLinkNo finished for user %d", user.TelegramID)
}

// handleSiteLinkYesPost handles user accepting site link after email
func (b *Bot) handleSiteLinkYesPost(query *tgbotapi.CallbackQuery, user *storage.User, settings *storage.Settings) {
	log.Printf("DEBUG: handleSiteLinkYesPost called for user %d. Current MessageCount: %d", user.TelegramID, user.MessageCount)
	// Reset message count as per requirement
	if err := b.storage.ResetUserMessageCount(user.TelegramID); err != nil {
		log.Printf("Error resetting user message count for user %d: %v", user.TelegramID, err)
	} else {
		log.Printf("DEBUG: Called ResetUserMessageCount for user %d", user.TelegramID)
	}

	// Update FSM state to show the post
	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateOfferingSitePost)); err != nil {
		log.Printf("Error updating FSM state to OfferingSitePost for user %d: %v", user.TelegramID, err)
	}

	msg := tgbotapi.NewMessage(query.Message.Chat.ID, fsm.GetSiteLinkOfferPost(settings.SiteURL))
	// No buttons needed for the post, just text and "Назад" which is part of the message text.
	// If a separate back button is desired, it would be here.
	// For now, relying on the "Назад" in the message text as per original request.

	sentMsg, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Error sending site link post for user %d: %v", user.TelegramID, err)
		return
	}

	// Log outgoing message
	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
	}
	log.Printf("DEBUG: handleSiteLinkYesPost finished for user %d", user.TelegramID)
}

// handleSiteLinkNoPost handles user declining site link after seeing the post
func (b *Bot) handleSiteLinkNoPost(query *tgbotapi.CallbackQuery, user *storage.User, settings *storage.Settings) {
	log.Printf("DEBUG: handleSiteLinkNoPost called for user %d. Current MessageCount: %d", user.TelegramID, user.MessageCount)
	// Reset message count as per requirement (explicitly set to 0)
	if err := b.storage.ResetUserMessageCount(user.TelegramID); err != nil {
		log.Printf("Error resetting user message count for user %d: %v", user.TelegramID, err)
	} else {
		log.Printf("DEBUG: Called ResetUserMessageCount for user %d", user.TelegramID)
	}

	// Update FSM state back to idle
	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateIdle)); err != nil {
		log.Printf("Error updating FSM state to Idle for user %d: %v", user.TelegramID, err)
	}

	// Inform user they can continue conversation
	msg := tgbotapi.NewMessage(query.Message.Chat.ID, "Можете продолжать консультацию. Если возникнут другие вопросы по электроинструменту, напишите мне.")
	sentMsg, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Error sending message after site link post decline for user %d: %v", user.TelegramID, err)
		return
	}

	// Log outgoing message
	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
	}
	log.Printf("DEBUG: handleSiteLinkNoPost finished for user %d", user.TelegramID)
}

// handleEmailConsentYes handles user granting email consent
func (b *Bot) handleEmailConsentYes(query *tgbotapi.CallbackQuery, user *storage.User, settings *storage.Settings) {
	log.Printf("DEBUG: handleEmailConsentYes called for user %d", user.TelegramID)
	// User granted email consent
	// Since we're in awaiting_email_consent state, the email was already validated
	// We just need to update consent
	if user.Email != "" {
		if err := b.storage.UpdateUserEmail(user.TelegramID, user.Email, true); err != nil {
			log.Printf("Error updating user email consent for user %d: %v", user.TelegramID, err)
		}
	}

	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateIdle)); err != nil {
		log.Printf("Error updating FSM state to Idle for user %d: %v", user.TelegramID, err)
	}

	msg := tgbotapi.NewMessage(query.Message.Chat.ID, fsm.GetEmailSavedMessage(settings.SiteURL))
	sentMsg, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Error sending confirmation for user %d: %v", user.TelegramID, err)
		return
	}

	// Log outgoing message
	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
	}
	log.Printf("DEBUG: handleEmailConsentYes finished for user %d", user.TelegramID)
}

// handleEmailConsentNo handles user declining email consent
func (b *Bot) handleEmailConsentNo(query *tgbotapi.CallbackQuery, user *storage.User, settings *storage.Settings) {
	log.Printf("DEBUG: handleEmailConsentNo called for user %d", user.TelegramID)
	// User declined email consent
	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateIdle)); err != nil {
		log.Printf("Error updating FSM state to Idle for user %d: %v", user.TelegramID, err)
	}

	msg := tgbotapi.NewMessage(query.Message.Chat.ID, fsm.GetEmailDeclinedMessage(settings.SiteURL))
	sentMsg, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Error sending decline message for user %d: %v", user.TelegramID, err)
		return
	}

	// Log outgoing message
	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
	}
	log.Printf("DEBUG: handleEmailConsentNo finished for user %d", user.TelegramID)
}

// handleEmailConfirm handles email confirmation buttons
func (b *Bot) handleEmailConfirm(query *tgbotapi.CallbackQuery, user *storage.User) {
	log.Printf("DEBUG: handleEmailConfirm called for user %d with data: %s", query.From.ID, query.Data)
	email := strings.TrimPrefix(query.Data, "email_confirm_")
	log.Printf("DEBUG: Parsed email for user %d: %s", user.TelegramID, email)


	// Save email temporarily and show consent buttons
	if err := b.storage.UpdateUserEmail(user.TelegramID, email, false); err != nil {
		log.Printf("Error saving email for user %d: %v", user.TelegramID, err)
	}

	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateAwaitingEmailConsent)); err != nil {
		log.Printf("Error updating FSM state to AwaitingEmailConsent for user %d: %v", user.TelegramID, err)
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
		log.Printf("Error sending consent request for user %d: %v", user.TelegramID, err)
		return
	}

	// Log outgoing message
	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
	}
	log.Printf("DEBUG: handleEmailConfirm finished for user %d", user.TelegramID)
}

// handleStartScenario handles scenario start
func (b *Bot) handleStartScenario(query *tgbotapi.CallbackQuery, user *storage.User) {
	log.Printf("DEBUG: handleStartScenario called for user %d with data: %s", query.From.ID, query.Data)
	scenarioIDStr := strings.TrimPrefix(query.Data, "start_scenario_")
	scenarioID, err := strconv.Atoi(scenarioIDStr)
	if err != nil {
		log.Printf("Error parsing scenario ID for user %d: %v", query.From.ID, err)
		return
	}
	log.Printf("DEBUG: Parsed scenarioID for user %d: %d", user.TelegramID, scenarioID)

	// Get scenario
	scenario, err := b.storage.GetFSMScenario(scenarioID)
	if err != nil {
		log.Printf("Error getting scenario %d for user %d: %v", scenarioID, user.TelegramID, err)
		return
	}
	if scenario == nil {
		log.Printf("Scenario %d not found for user %d", scenarioID, user.TelegramID)
		return
	}

	// Start scenario
	step, err := b.fsm.GetFirstStep(scenarioID)
	if err != nil {
		log.Printf("Error getting first step for scenario %d, user %d: %v", scenarioID, user.TelegramID, err)
		return
	}
	if step != nil {
		log.Printf("DEBUG: Updating session for user %d to scenario %d, step %s", user.TelegramID, scenarioID, step.StepKey)
		err = b.storage.UpdateUserSession(user.TelegramID, &scenarioID, &step.StepKey)
		if err != nil {
			log.Printf("Error updating session for user %d: %v", user.TelegramID, err)
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
			log.Printf("Error sending scenario start message for user %d: %v", user.TelegramID, err)
			return
		}

		// Log outgoing message
		if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
			log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
		}
	}
	log.Printf("DEBUG: handleStartScenario finished for user %d", user.TelegramID)
}

// handleOption handles option selection
func (b *Bot) handleOption(query *tgbotapi.CallbackQuery, user *storage.User) {
	log.Printf("DEBUG: handleOption called for user %d with data: %s", query.From.ID, query.Data)
	parts := strings.Split(query.Data, "_")
	if len(parts) >= 4 {
		scenarioID, _ := strconv.Atoi(parts[1])
		stepKey := parts[2]
		optionNum := parts[3]

		log.Printf("DEBUG: Parsed option for user %d: scenarioID=%d, stepKey=%s, optionNum=%s", user.TelegramID, scenarioID, stepKey, optionNum)

		// Find next step based on option
		steps, err := b.storage.GetFSMScenarioSteps(scenarioID)
		if err != nil {
			log.Printf("Error getting steps for scenario %d, user %d: %v", scenarioID, user.TelegramID, err)
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
			log.Printf("DEBUG: Updating session for user %d to scenario %d, step %s", user.TelegramID, scenarioID, nextStep.StepKey)
			err = b.storage.UpdateUserSession(user.TelegramID, &scenarioID, &nextStep.StepKey)
			if err != nil {
				log.Printf("Error updating session for user %d: %v", user.TelegramID, err)
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
				log.Printf("Error sending option response for user %d: %v", user.TelegramID, err)
				return
			}

			// Log outgoing message
			if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
				log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
			}
		} else {
			log.Printf("Next step not found for user %d: scenarioID=%d, stepKey=%s_%s", user.TelegramID, scenarioID, stepKey, optionNum)
		}
	} else {
		log.Printf("Invalid option data for user %d: %s", user.TelegramID, query.Data)
	}
	log.Printf("DEBUG: handleOption finished for user %d", user.TelegramID)
}

// handleGoto handles goto step
func (b *Bot) handleGoto(query *tgbotapi.CallbackQuery, user *storage.User) {
	// Format: goto_<scenarioID>_<stepKey>
	// stepKey can contain underscores, so we need to parse carefully
	parts := strings.SplitN(query.Data, "_", 3)
	log.Printf("DEBUG: handleGoto called for user %d with data: %s", query.From.ID, query.Data)
	if len(parts) >= 3 {
		scenarioID, _ := strconv.Atoi(parts[1])
		stepKey := parts[2]

		log.Printf("DEBUG: handleGoto parsed for user %d: scenarioID=%d, stepKey=%s", user.TelegramID, scenarioID, stepKey)

		step, err := b.storage.GetFSMScenarioStep(scenarioID, stepKey)
		if err != nil {
			log.Printf("Error getting step %s for scenario %d, user %d: %v", stepKey, scenarioID, user.TelegramID, err)
			return
		}
		if step != nil {
			log.Printf("DEBUG: Updating session for user %d to scenario %d, step %s", user.TelegramID, scenarioID, step.StepKey)
			err = b.storage.UpdateUserSession(user.TelegramID, &scenarioID, &step.StepKey)
			if err != nil {
				log.Printf("Error updating session for user %d: %v", user.TelegramID, err)
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
				log.Printf("Error sending goto response for user %d: %v", user.TelegramID, err)
				return
			}

			// Log outgoing message
			if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
				log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
			}
		} else {
			log.Printf("Step not found for user %d: scenarioID=%d, stepKey=%s", user.TelegramID, scenarioID, stepKey)
		}
	} else {
		log.Printf("Invalid goto data for user %d: %s", user.TelegramID, query.Data)
	}
	log.Printf("DEBUG: handleGoto finished for user %d", user.TelegramID)
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
	log.Printf("DEBUG: handleAction called for user %d with data: %s", query.From.ID, query.Data)
	parts := strings.Split(query.Data, "_")
	if len(parts) >= 3 {
		scenarioID, _ := strconv.Atoi(parts[1])
		actionKey := parts[2]

		log.Printf("DEBUG: Parsed action for user %d: scenarioID=%d, actionKey=%s", user.TelegramID, scenarioID, actionKey)


		step, err := b.storage.GetFSMScenarioStep(scenarioID, actionKey)
		if err != nil {
			log.Printf("Error getting action step %s for scenario %d, user %d: %v", actionKey, scenarioID, user.TelegramID, err)
			return
		}
		if step != nil {
			log.Printf("DEBUG: Updating session for user %d to scenario %d, step %s", user.TelegramID, scenarioID, step.StepKey)
			err = b.storage.UpdateUserSession(user.TelegramID, &scenarioID, &step.StepKey)
			if err != nil {
				log.Printf("Error updating session for user %d: %v", user.TelegramID, err)
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
				log.Printf("Error sending action response for user %d: %v", user.TelegramID, err)
				return
			}

			// Log outgoing message
			if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
				log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
			}
		}
	}
	log.Printf("DEBUG: handleAction finished for user %d", user.TelegramID)
}

// handleBack handles back button navigation
func (b *Bot) handleBack(query *tgbotapi.CallbackQuery, user *storage.User) {
	log.Printf("DEBUG: handleBack called for user %d with data: %s", query.From.ID, query.Data)
	parts := strings.Split(query.Data, "_")
	if len(parts) >= 3 {
		scenarioID, _ := strconv.Atoi(parts[1])
		currentStepKey := parts[2]

		log.Printf("DEBUG: Parsed back for user %d: scenarioID=%d, currentStepKey=%s", user.TelegramID, scenarioID, currentStepKey)

		// If we're on the root step, go back to scenario selection
		if currentStepKey == "root" {
			log.Printf("DEBUG: User %d on root step, clearing session to return to scenario selection", user.TelegramID)
			// Clear session to return to scenario selection
			if err := b.storage.DeleteUserSession(user.TelegramID); err != nil {
				log.Printf("Error clearing session for user %d: %v", user.TelegramID, err)
				return
			}

			// Show scenario selection
			msg := tgbotapi.NewMessage(query.Message.Chat.ID, fsm.GetStartMessage())
			scenariosButtons, err := b.fsm.GetScenariosButtons()
			if err != nil {
				log.Printf("Error getting scenarios buttons for user %d: %v", user.TelegramID, err)
				return
			}
			if len(scenariosButtons) > 0 {
				keyboard := b.createInlineKeyboard(scenariosButtons)
				msg.ReplyMarkup = keyboard
			}

			sentMsg, err := b.api.Send(msg)
			if err != nil {
				log.Printf("Error sending scenario selection for user %d: %v", user.TelegramID, err)
				return
			}

			// Log outgoing message
			if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
				log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
			}
			log.Printf("DEBUG: handleBack finished for user %d (root case)", user.TelegramID)
			return
		}

		// Get the previous step key
		previousStepKey := b.fsm.GetPreviousStepKey(currentStepKey)
		log.Printf("DEBUG: Previous step key for user %d from %s is %s", user.TelegramID, currentStepKey, previousStepKey)

		step, err := b.storage.GetFSMScenarioStep(scenarioID, previousStepKey)
		if err != nil {
			log.Printf("Error getting previous step %s for scenario %d, user %d: %v", previousStepKey, scenarioID, user.TelegramID, err)
			return
		}
		if step != nil {
			log.Printf("DEBUG: Updating session for user %d to scenario %d, step %s", user.TelegramID, scenarioID, step.StepKey)
			err = b.storage.UpdateUserSession(user.TelegramID, &scenarioID, &step.StepKey)
			if err != nil {
				log.Printf("Error updating session for user %d: %v", user.TelegramID, err)
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
				log.Printf("Error sending back navigation response for user %d: %v", user.TelegramID, err)
				return
			}

			// Log outgoing message
			if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
				log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
			}
		} else {
			log.Printf("Previous step not found for user %d: scenarioID=%d, previousStepKey=%s", user.TelegramID, scenarioID, previousStepKey)
		}
	} else {
		log.Printf("Invalid back data for user %d: %s", user.TelegramID, query.Data)
	}
	log.Printf("DEBUG: handleBack finished for user %d", user.TelegramID)
}

// GetUserIDFromString converts string to user ID
func GetUserIDFromString(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}