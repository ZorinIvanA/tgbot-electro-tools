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
	rateLimitPerMin int
}

// NewBot creates a new bot instance
func NewBot(token string, storage storage.Storage, rateLimitPerMin int) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API: %w", err)
	}

	log.Printf("Authorized on account %s", api.Self.UserName)

	return &Bot{
		api:             api,
		storage:         storage,
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
	stateMachine := fsm.NewFSM(user.FSMState)
	response, newState, handled := stateMachine.ProcessMessage(message.Text)

	// Update FSM state if changed
	if newState != stateMachine.GetState() {
		if err := b.storage.UpdateUserFSMState(message.From.ID, string(newState)); err != nil {
			log.Printf("Error updating FSM state: %v", err)
		}
	}

	// Send response if handled by FSM
	if handled && response != "" {
		msg := tgbotapi.NewMessage(message.Chat.ID, response)
		sentMsg, err := b.api.Send(msg)
		if err != nil {
			log.Printf("Error sending message: %v", err)
			return
		}

		// Log outgoing message
		if err := b.storage.LogMessage(message.From.ID, sentMsg.Text, "outgoing"); err != nil {
			log.Printf("Error logging outgoing message: %v", err)
		}
	} else if !handled && user.FSMState == string(fsm.StateIdle) {
		// If not handled and in idle state, send generic response
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

	msg := tgbotapi.NewMessage(chatID, fsm.GetStartMessage())
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

	case "site_link_no":
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

	case "email_consent_yes":
		// User granted email consent
		// Get the email from awaiting_email_consent state context
		// In real implementation, we'd need to store the email temporarily
		// For now, we'll save the email that was entered

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

	case "email_consent_no":
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

	default:
		// Handle email confirmation buttons with email embedded in callback data
		if strings.HasPrefix(query.Data, "email_confirm_") {
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

// GetUserIDFromString converts string to user ID
func GetUserIDFromString(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
