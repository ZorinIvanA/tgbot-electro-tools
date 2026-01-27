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

type Bot struct {
	api             *tgbotapi.BotAPI
	storage         storage.Storage
	fsm             *fsm.FSM
	rateLimitPerMin int
}

func NewBot(token string, storage storage.Storage, rateLimitPerMin int, openAIEnabled bool, openAIURL, openAIKey, openAIModel string) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API: %w", err)
	}

	log.Printf("Authorized on account %s", api.Self.UserName)

	fsmInstance := fsm.NewFSM(storage, openAIEnabled, openAIURL, openAIKey, openAIModel)

	return &Bot{
		api:             api,
		storage:         storage,
		fsm:             fsmInstance,
		rateLimitPerMin: rateLimitPerMin,
	}, nil
}

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

func (b *Bot) handleMessage(message *tgbotapi.Message) {
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

	user, err := b.storage.GetOrCreateUser(message.From.ID)
	if err != nil {
		log.Printf("Error getting/creating user %d: %v", message.From.ID, err)
		return
	}

	if message.IsCommand() && message.Command() == "start" {
		b.handleStartCommand(message.Chat.ID, user)
		return
	}

	b.processMessage(message, user)
}

func (b *Bot) handleStartCommand(chatID int64, user *storage.User) {
	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateIdle)); err != nil {
		log.Printf("Error resetting FSM state for user %d: %v", user.TelegramID, err)
	}

	if err := b.storage.DeleteUserSession(user.TelegramID); err != nil {
		log.Printf("Error clearing user session for user %d: %v", user.TelegramID, err)
	}

	if err := b.storage.ResetUserMessageCount(user.TelegramID); err != nil {
		log.Printf("Error resetting message count for user %d: %v", user.TelegramID, err)
	}

	msg := tgbotapi.NewMessage(chatID, fsm.GetStartMessage())

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

	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
	}
}

func (b *Bot) offerSiteLink(chatID int64, user *storage.User) {
	log.Printf("offerSiteLink called for user %d", user.TelegramID)
	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateOfferingSiteLink)); err != nil {
		log.Printf("Error updating FSM state to OfferingSiteLink for user %d: %v", user.TelegramID, err)
	}

	msg := tgbotapi.NewMessage(chatID, fsm.GetSiteLinkOfferMessage())

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

	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
	}
	log.Printf("offerSiteLink finished for user %d", user.TelegramID)
}

func (b *Bot) handleCallbackQuery(query *tgbotapi.CallbackQuery) {
	allowed, err := b.storage.CheckRateLimit(query.From.ID, b.rateLimitPerMin)
	if err != nil {
		log.Printf("Error checking rate limit for user %d: %v", query.From.ID, err)
		return
	}

	if !allowed {
		log.Printf("Rate limit exceeded for user %d", query.From.ID)
		return
	}

	user, err := b.storage.GetUser(query.From.ID)
	if err != nil {
		log.Printf("Error getting user %d for callback query: %v", query.From.ID, err)
		return
	}

	if user == nil {
		log.Printf("User %d not found for callback query", query.From.ID)
		return
	}

	b.processCallbackQuery(query, user)
}

func (b *Bot) handleSiteLinkYes(query *tgbotapi.CallbackQuery, user *storage.User) {
	log.Printf("handleSiteLinkYes called for user %d", user.TelegramID)
	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateAwaitingEmail)); err != nil {
		log.Printf("Error updating FSM state to AwaitingEmail for user %d: %v", user.TelegramID, err)
	}

	msg := tgbotapi.NewMessage(query.Message.Chat.ID, fsm.GetEmailRequestMessage())
	sentMsg, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Error sending email request for user %d: %v", user.TelegramID, err)
		return
	}

	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
	}
	log.Printf("handleSiteLinkYes finished for user %d", user.TelegramID)
}

func (b *Bot) handleSiteLinkNo(query *tgbotapi.CallbackQuery, user *storage.User) {
	log.Printf("handleSiteLinkNo called for user %d. Current MessageCount: %d", user.TelegramID, user.MessageCount)
	if err := b.storage.ResetUserMessageCount(user.TelegramID); err != nil {
		log.Printf("Error resetting user message count for user %d: %v", user.TelegramID, err)
	} else {
		log.Printf("Called ResetUserMessageCount for user %d", user.TelegramID)
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

	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
	}
	log.Printf("handleSiteLinkNo finished for user %d", user.TelegramID)
}

func (b *Bot) handleSiteLinkYesPost(query *tgbotapi.CallbackQuery, user *storage.User, settings *storage.Settings) {
	log.Printf("handleSiteLinkYesPost called for user %d. Current MessageCount: %d", user.TelegramID, user.MessageCount)
	if err := b.storage.ResetUserMessageCount(user.TelegramID); err != nil {
		log.Printf("Error resetting user message count for user %d: %v", user.TelegramID, err)
	} else {
		log.Printf("Called ResetUserMessageCount for user %d", user.TelegramID)
	}

	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateOfferingSitePost)); err != nil {
		log.Printf("Error updating FSM state to OfferingSitePost for user %d: %v", user.TelegramID, err)
	}

	msg := tgbotapi.NewMessage(query.Message.Chat.ID, fsm.GetSiteLinkOfferPost(settings.SiteURL))

	sentMsg, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Error sending site link post for user %d: %v", user.TelegramID, err)
		return
	}

	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
	}
	log.Printf("handleSiteLinkYesPost finished for user %d", user.TelegramID)
}

func (b *Bot) handleSiteLinkNoPost(query *tgbotapi.CallbackQuery, user *storage.User, settings *storage.Settings) {
	log.Printf("handleSiteLinkNoPost called for user %d. Current MessageCount: %d", user.TelegramID, user.MessageCount)
	if err := b.storage.ResetUserMessageCount(user.TelegramID); err != nil {
		log.Printf("Error resetting user message count for user %d: %v", user.TelegramID, err)
	} else {
		log.Printf("Called ResetUserMessageCount for user %d", user.TelegramID)
	}

	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateIdle)); err != nil {
		log.Printf("Error updating FSM state to Idle for user %d: %v", user.TelegramID, err)
	}

	msg := tgbotapi.NewMessage(query.Message.Chat.ID, "Можете продолжать консультацию. Если возникнут другие вопросы по электроинструменту, напишите мне.")
	sentMsg, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Error sending message after site link post decline for user %d: %v", user.TelegramID, err)
		return
	}

	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
	}
	log.Printf("handleSiteLinkNoPost finished for user %d", user.TelegramID)
}

func (b *Bot) handleEmailConsentYes(query *tgbotapi.CallbackQuery, user *storage.User, settings *storage.Settings) {
	log.Printf("handleEmailConsentYes called for user %d", user.TelegramID)
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

	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
	}
	log.Printf("handleEmailConsentYes finished for user %d", user.TelegramID)
}

func (b *Bot) handleEmailConsentNo(query *tgbotapi.CallbackQuery, user *storage.User, settings *storage.Settings) {
	log.Printf("handleEmailConsentNo called for user %d", user.TelegramID)
	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateIdle)); err != nil {
		log.Printf("Error updating FSM state to Idle for user %d: %v", user.TelegramID, err)
	}

	msg := tgbotapi.NewMessage(query.Message.Chat.ID, fsm.GetEmailDeclinedMessage(settings.SiteURL))
	sentMsg, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Error sending decline message for user %d: %v", user.TelegramID, err)
		return
	}

	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
	}
	log.Printf("handleEmailConsentNo finished for user %d", user.TelegramID)
}

func (b *Bot) handleEmailConfirm(query *tgbotapi.CallbackQuery, user *storage.User) {
	log.Printf("handleEmailConfirm called for user %d with data: %s", query.From.ID, query.Data)
	email := strings.TrimPrefix(query.Data, "email_confirm_")
	log.Printf("Parsed email for user %d: %s", user.TelegramID, email)

	if err := b.storage.UpdateUserEmail(user.TelegramID, email, false); err != nil {
		log.Printf("Error saving email for user %d: %v", user.TelegramID, err)
	}

	if err := b.storage.UpdateUserFSMState(user.TelegramID, string(fsm.StateAwaitingEmailConsent)); err != nil {
		log.Printf("Error updating FSM state to AwaitingEmailConsent for user %d: %v", user.TelegramID, err)
	}

	msg := tgbotapi.NewMessage(query.Message.Chat.ID, fsm.GetEmailConsentMessage())

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

	if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
		log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
	}
	log.Printf("handleEmailConfirm finished for user %d", user.TelegramID)
}

func (b *Bot) handleStartScenario(query *tgbotapi.CallbackQuery, user *storage.User) {
	log.Printf("handleStartScenario called for user %d with data: %s", query.From.ID, query.Data)
	scenarioIDStr := strings.TrimPrefix(query.Data, "start_scenario_")
	scenarioID, err := strconv.Atoi(scenarioIDStr)
	if err != nil {
		log.Printf("Error parsing scenario ID for user %d: %v", query.From.ID, err)
		return
	}
	log.Printf("Parsed scenarioID for user %d: %d", user.TelegramID, scenarioID)

	scenario, err := b.storage.GetFSMScenario(scenarioID)
	if err != nil {
		log.Printf("Error getting scenario %d for user %d: %v", scenarioID, user.TelegramID, err)
		return
	}
	if scenario == nil {
		log.Printf("Scenario %d not found for user %d", scenarioID, user.TelegramID)
		return
	}

	step, err := b.fsm.GetFirstStep(scenarioID)
	if err != nil {
		log.Printf("Error getting first step for scenario %d, user %d: %v", scenarioID, user.TelegramID, err)
		return
	}
	if step != nil {
		log.Printf("Updating session for user %d to scenario %d, step %s", user.TelegramID, scenarioID, step.StepKey)
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

		if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
			log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
		}
	}
	log.Printf("handleStartScenario finished for user %d", user.TelegramID)
}

func (b *Bot) handleOption(query *tgbotapi.CallbackQuery, user *storage.User) {
	log.Printf("handleOption called for user %d with data: %s", query.From.ID, query.Data)
	parts := strings.Split(query.Data, "_")
	if len(parts) >= 4 {
		scenarioID, _ := strconv.Atoi(parts[1])
		stepKey := parts[2]
		optionNum := parts[3]

		log.Printf("Parsed option for user %d: scenarioID=%d, stepKey=%s, optionNum=%s", user.TelegramID, scenarioID, stepKey, optionNum)

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
			log.Printf("Updating session for user %d to scenario %d, step %s", user.TelegramID, scenarioID, nextStep.StepKey)
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

			if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
				log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
			}
		} else {
			log.Printf("Next step not found for user %d: scenarioID=%d, stepKey=%s_%s", user.TelegramID, scenarioID, stepKey, optionNum)
		}
	} else {
		log.Printf("Invalid option data for user %d: %s", user.TelegramID, query.Data)
	}
	log.Printf("handleOption finished for user %d", user.TelegramID)
}

func (b *Bot) handleGoto(query *tgbotapi.CallbackQuery, user *storage.User) {
	parts := strings.SplitN(query.Data, "_", 3)
	log.Printf("handleGoto called for user %d with data: %s", query.From.ID, query.Data)
	if len(parts) >= 3 {
		scenarioID, _ := strconv.Atoi(parts[1])
		stepKey := parts[2]

		log.Printf("handleGoto parsed for user %d: scenarioID=%d, stepKey=%s", user.TelegramID, scenarioID, stepKey)

		step, err := b.storage.GetFSMScenarioStep(scenarioID, stepKey)
		if err != nil {
			log.Printf("Error getting step %s for scenario %d, user %d: %v", stepKey, scenarioID, user.TelegramID, err)
			return
		}
		if step != nil {
			log.Printf("Updating session for user %d to scenario %d, step %s", user.TelegramID, scenarioID, step.StepKey)
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

			if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
				log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
			}
		} else {
			log.Printf("Step not found for user %d: scenarioID=%d, stepKey=%s", user.TelegramID, scenarioID, stepKey)
		}
	} else {
		log.Printf("Invalid goto data for user %d: %s", user.TelegramID, query.Data)
	}
	log.Printf("handleGoto finished for user %d", user.TelegramID)
}

func ShouldOfferSiteLink(messageCount int, triggerCount int, currentState string) bool {
	return messageCount == triggerCount && currentState == string(fsm.StateIdle)
}

func (b *Bot) Stop() {
	b.api.StopReceivingUpdates()
}

func (b *Bot) GetUsername() string {
	return b.api.Self.UserName
}

func (b *Bot) createInlineKeyboard(buttons []fsm.Button) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

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

func (b *Bot) handleAction(query *tgbotapi.CallbackQuery, user *storage.User) {
	log.Printf("handleAction called for user %d with data: %s", query.From.ID, query.Data)
	parts := strings.Split(query.Data, "_")
	if len(parts) >= 3 {
		scenarioID, _ := strconv.Atoi(parts[1])
		actionKey := parts[2]

		log.Printf("Parsed action for user %d: scenarioID=%d, actionKey=%s", user.TelegramID, scenarioID, actionKey)

		step, err := b.storage.GetFSMScenarioStep(scenarioID, actionKey)
		if err != nil {
			log.Printf("Error getting action step %s for scenario %d, user %d: %v", actionKey, scenarioID, user.TelegramID, err)
			return
		}
		if step != nil {
			log.Printf("Updating session for user %d to scenario %d, step %s", user.TelegramID, scenarioID, step.StepKey)
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

			if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
				log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
			}
		}
	}
	log.Printf("handleAction finished for user %d", user.TelegramID)
}

func (b *Bot) handleBack(query *tgbotapi.CallbackQuery, user *storage.User) {
	log.Printf("handleBack called for user %d with data: %s", query.From.ID, query.Data)
	parts := strings.Split(query.Data, "_")
	if len(parts) >= 3 {
		scenarioID, _ := strconv.Atoi(parts[1])
		currentStepKey := parts[2]

		log.Printf("Parsed back for user %d: scenarioID=%d, currentStepKey=%s", user.TelegramID, scenarioID, currentStepKey)

		if currentStepKey == "root" {
			log.Printf("User %d on root step, clearing session to return to scenario selection", user.TelegramID)
			if err := b.storage.DeleteUserSession(user.TelegramID); err != nil {
				log.Printf("Error clearing session for user %d: %v", user.TelegramID, err)
				return
			}

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

			if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
				log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
			}
			log.Printf("handleBack finished for user %d (root case)", user.TelegramID)
			return
		}

		previousStepKey := b.fsm.GetPreviousStepKey(currentStepKey)
		log.Printf("Previous step key for user %d from %s is %s", user.TelegramID, currentStepKey, previousStepKey)

		step, err := b.storage.GetFSMScenarioStep(scenarioID, previousStepKey)
		if err != nil {
			log.Printf("Error getting previous step %s for scenario %d, user %d: %v", previousStepKey, scenarioID, user.TelegramID, err)
			return
		}
		if step != nil {
			log.Printf("Updating session for user %d to scenario %d, step %s", user.TelegramID, scenarioID, step.StepKey)
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

			if err := b.storage.LogMessage(user.TelegramID, sentMsg.Text, "outgoing"); err != nil {
				log.Printf("Error logging outgoing message for user %d: %v", user.TelegramID, err)
			}
		} else {
			log.Printf("Previous step not found for user %d: scenarioID=%d, previousStepKey=%s", user.TelegramID, scenarioID, previousStepKey)
		}
	} else {
		log.Printf("Invalid back data for user %d: %s", user.TelegramID, query.Data)
	}
	log.Printf("handleBack finished for user %d", user.TelegramID)
}

func GetUserIDFromString(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func (b *Bot) processMessage(message *tgbotapi.Message, user *storage.User) {
	if err := b.storage.UpdateUserMessageCount(message.From.ID); err != nil {
		log.Printf("Error updating message count for user %d: %v", message.From.ID, err)
		return
	}

	updatedUser, err := b.storage.GetUser(message.From.ID)
	if err != nil {
		log.Printf("Error reloading user %d: %v", message.From.ID, err)
		return
	}
	if updatedUser == nil {
		log.Printf("ERROR: Failed to reload user %d, got nil", message.From.ID)
		return
	}

	if err := b.storage.LogMessage(message.From.ID, message.Text, "incoming"); err != nil {
		log.Printf("Error logging incoming message for user %d: %v", message.From.ID, err)
	}

	settings, err := b.storage.GetSettings()
	if err != nil {
		log.Printf("Error getting settings for user %d: %v", message.From.ID, err)
		return
	}

	if updatedUser.MessageCount >= settings.TriggerMessageCount {
		b.offerSiteLink(message.Chat.ID, updatedUser)
		if err := b.storage.ResetUserMessageCount(message.From.ID); err != nil {
			log.Printf("Error resetting message count for user %d: %v", message.From.ID, err)
		}
		return
	}

	response, buttons, handled, err := b.fsm.ProcessMessage(message.From.ID, message.Text)
	if err != nil {
		log.Printf("Error processing message through FSM for user %d: %v", message.From.ID, err)
		return
	}

	if handled && response != "" {
		msg := tgbotapi.NewMessage(message.Chat.ID, response)

		if len(buttons) > 0 {
			keyboard := b.createInlineKeyboard(buttons)
			msg.ReplyMarkup = keyboard
		}

		sentMsg, err := b.api.Send(msg)
		if err != nil {
			log.Printf("Error sending message for user %d: %v", message.From.ID, err)
			return
		}

		if err := b.storage.LogMessage(message.From.ID, sentMsg.Text, "outgoing"); err != nil {
			log.Printf("Error logging outgoing message for user %d: %v", message.From.ID, err)
		}
	} else if !handled {
		genericResponse := "Я вас понял. Если возникнут проблемы с электроинструментом, опишите их подробнее, и я постараюсь помочь!"
		msg := tgbotapi.NewMessage(message.Chat.ID, genericResponse)
		sentMsg, err := b.api.Send(msg)
		if err != nil {
			log.Printf("Error sending generic response for user %d: %v", message.From.ID, err)
			return
		}

		if err := b.storage.LogMessage(message.From.ID, sentMsg.Text, "outgoing"); err != nil {
			log.Printf("Error logging generic response for user %d: %v", message.From.ID, err)
		}
	}
}

func (b *Bot) processCallbackQuery(query *tgbotapi.CallbackQuery, user *storage.User) {
	if err := b.storage.UpdateUserMessageCount(query.From.ID); err != nil {
		log.Printf("Error updating message count for user %d (callback): %v", query.From.ID, err)
		return
	}

	updatedUser, err := b.storage.GetUser(query.From.ID)
	if err != nil {
		log.Printf("Error reloading user %d (callback): %v", query.From.ID, err)
		return
	}
	if updatedUser == nil {
		log.Printf("ERROR: Failed to reload user %d, got nil (callback)", query.From.ID)
		return
	}

	settings, err := b.storage.GetSettings()
	if err != nil {
		log.Printf("Error getting settings for user %d during callback: %v", query.From.ID, err)
		return
	}

	if updatedUser.MessageCount >= settings.TriggerMessageCount {
		b.offerSiteLink(query.Message.Chat.ID, updatedUser)
		if err := b.storage.ResetUserMessageCount(query.From.ID); err != nil {
			log.Printf("Error resetting message count for user %d: %v", query.From.ID, err)
		}
		return
	}

	switch query.Data {
	case "site_link_yes":
		b.handleSiteLinkYes(query, user)
	case "site_link_no":
		b.handleSiteLinkNo(query, user)
	case "site_link_yes_post":
		b.handleSiteLinkYesPost(query, user, settings)
	case "site_link_no_post":
		b.handleSiteLinkNoPost(query, user, settings)
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
		} else if strings.HasPrefix(query.Data, "back_") {
			b.handleBack(query, user)
		} else {
			log.Printf("Unknown callback data for user %d: %s", query.From.ID, query.Data)
		}
	}
}
