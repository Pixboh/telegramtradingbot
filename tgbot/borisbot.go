package tgbot

import (
	"context"
	"fmt"
	"github.com/go-faster/errors"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"log"
	"strconv"
	"strings"
	"tdlib/authmanager"
	"time"
)

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

// This bot demonstrates some example interactions with commands on telegram.
// It has a basic start command with a bot intro.
// It also has a source command, which sends the bot sourcecode, as a file.
func (tgBot *TgBot) LaunchBorisBot(*telegram.Client) {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		// If an error is returned by a handler, log it and continue going.
		Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
			log.Println("an error occurred while handling update:", err.Error())
			return ext.DispatcherActionNoop
		},
		MaxRoutines: ext.DefaultMaxRoutines,
	})
	updater := ext.NewUpdater(dispatcher, nil)

	// /start command to introduce the bot
	dispatcher.AddHandler(handlers.NewCommand("start_trading", tgBot.start_trading))
	// /source command to send the bot source code
	dispatcher.AddHandler(handlers.NewCommand("stop_trading", tgBot.stop))
	dispatcher.AddHandler(handlers.NewCommand("add_working_channels", tgBot.addWorkingChannels))
	dispatcher.AddHandler(handlers.NewCommand("set_volume", tgBot.setTradeVolumeCallback))
	// set a volume for each channel select_chan_volume
	dispatcher.AddHandler(handlers.NewCommand("set_channel_volume", tgBot.setChannelVolumeCallback))
	dispatcher.AddHandler(handlers.NewCommand("set_daily_profit_goal", tgBot.setDailyProfitGoalCallback))
	dispatcher.AddHandler(handlers.NewCommand("set_symbols", tgBot.setSymbolsCallback))
	// set channel breakeven
	dispatcher.AddHandler(handlers.NewCommand("set_channel_breakeven", tgBot.setChannelBreakevenCallback))
	dispatcher.AddHandler(handlers.NewCommand("set_strategy", tgBot.setStrategyCallback))
	dispatcher.AddHandler(handlers.NewCommand("status", tgBot.GetBotStatus))
	dispatcher.AddHandler(handlers.NewCommand("set_risk", tgBot.setRiskPercentageCallback))
	// maximum open trades
	dispatcher.AddHandler(handlers.NewCommand("set_max_open_trades", tgBot.setMaxOpenTradesCallback))

	dispatcher.AddHandler(handlers.NewCallback(nil, tgBot.handleCallback))

	// Start receiving updates.
	err := updater.StartPolling(tgBot.Bot, &ext.PollingOpts{
		DropPendingUpdates: true,
		GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
			Timeout: 9,
			RequestOpts: &gotgbot.RequestOpts{
				Timeout: time.Second * 10,
			},
		},
	})
	if err != nil {
		panic("failed to start polling: " + err.Error())
	}
	log.Printf("%s has been started...\n", tgBot.Bot.User.Username)

	tgBot.Bot.SetMyCommands([]gotgbot.BotCommand{
		{
			Command:     "start_trading",
			Description: "Start Auto Trading",
		},
		{
			Command:     "stop_trading",
			Description: "Stop Auto Trading",
		},
		{
			Command:     "add_working_channels",
			Description: "Add working channels",
		},
		{
			Command:     "set_volume",
			Description: "Set trading volume",
		},
		{
			Command:     "set_daily_profit_goal",
			Description: "Set daily profit goal",
		},
		{
			Command:     "set_symbols",
			Description: "Set Symbols",
		},
		{
			Command:     "set_channel_breakeven",
			Description: "Set channel breakeven",
		},
		{
			Command:     "set_strategy",
			Description: "Set strategy",
		},
		{
			Command:     "status",
			Description: "Bot status",
		},
		{
			Command:     "set_risk",
			Description: "Set risk percentage",
		},
		{
			Command:     "set_max_open_trades",
			Description: "Set maximum open trades",
		},
		{
			Command:     "set_channel_volume",
			Description: "Set channel volume",
		},
	}, nil)

	// Idle, to keep updates coming in, and avoid bot stopping.
	updater.Idle()
}

func (tgBot *TgBot) setMaxOpenTradesCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	return tgBot.setMaxOpenTrades(b, ctx, false)
}

func (tgBot *TgBot) setChannelVolumeCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	return tgBot.setChannelVolume(b, ctx, false)
}

// select a channel from list of added channels then set a volume for it base on a list of custom volumes.
// it will help to customize volume for each channel
func (tgBot *TgBot) setChannelVolume(b *gotgbot.Bot, ctx *ext.Context, update bool) error {
	// first show list of channels
	// get all working channelIds
	channelIds := tgBot.RedisClient.GetChannels()
	// load telegram channelIds
	// list of channels telegram
	telegramChats := make([]tg.MessagesChats, 0)
	for _, channelId := range channelIds {
		inputChannles := make([]tg.InputChannelClass, 0)
		inputChannles = append(inputChannles, &tg.InputChannel{
			ChannelID: channelId,
		})
		telegramChannelById, errTg := tgBot.tdClient.API().ChannelsGetChannels(context.Background(), inputChannles)
		if errTg != nil {
			//		return fmt.Errorf("failed to get channels: %w", errTg)
		}
		if telegramChannelById == nil {
			continue
		} else {
			// cast to chats
			tMessageChat := telegramChannelById.(*tg.MessagesChats)
			telegramChats = append(telegramChats, *tMessageChat)
		}

	}
	// create inline keyboard
	var inlineKeyboard [][]gotgbot.InlineKeyboardButton
	{
		for _, channelItemA := range telegramChats {
			for _, channelItem := range channelItemA.Chats {
				channelVolume := tgBot.RedisClient.GetChannelVolume(int(channelItem.(*tg.Channel).ID))
				defaultVolume := tgBot.RedisClient.GetDefaultTradingVolume()
				if channel, ok := channelItem.(*tg.Channel); ok {
					text := channel.Title
					// add arrow emoji : ➡️
					text = text + " ➡️ " + fmt.Sprintf("%.2f", channelVolume) + ""
					if channelVolume != defaultVolume {
						text = text + " ✅"
					}
					inlineKeyboard = append(inlineKeyboard, []gotgbot.InlineKeyboardButton{
						{
							Text:         text,
							CallbackData: fmt.Sprintf("channel_volume_%s", strconv.Itoa(int(channel.ID))),
						},
					})
				}
			}
		}
	}

	// Create an InlineKeyboardMarkup with the buttons
	replyMarkup := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: inlineKeyboard,
	}

	// check if reply or edit
	if !update {
		_, err := ctx.EffectiveMessage.Reply(b, fmt.Sprintf("Choose the channel to set the volume:"), &gotgbot.SendMessageOpts{
			ParseMode:   "HTML",
			ReplyMarkup: replyMarkup,
		})
		if err != nil {
			return fmt.Errorf("failed to send start message: %w", err)
		}
	} else {
		_, _, err := ctx.EffectiveMessage.EditText(b, fmt.Sprintf("Choose the channel to set the volume:"), &gotgbot.EditMessageTextOpts{
			ParseMode:   "HTML",
			ReplyMarkup: replyMarkup,
		})
		if err != nil {
			return fmt.Errorf("failed to send start message: %w", err)
		}
	}

	return nil
}
func (tgBot *TgBot) setMaxOpenTrades(b *gotgbot.Bot, ctx *ext.Context, update bool) error {
	// Create the inline keyboard buttons
	// list of volumes
	maxOpenTrades := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	// generate inlineKeyboard base on volumes
	var inlineKeyboard [][]gotgbot.InlineKeyboardButton
	for _, maxOpenTrade := range maxOpenTrades {
		// current volule
		currentMaxOpenTrade := tgBot.RedisClient.GetMaxOpenTrades()
		// limit to 2
		text := fmt.Sprintf("%d", maxOpenTrade)
		if currentMaxOpenTrade == maxOpenTrade {
			text = text + " ✅"
		}
		inlineKeyboard = append(inlineKeyboard, []gotgbot.InlineKeyboardButton{
			{
				Text:         text,
				CallbackData: fmt.Sprintf("max_open_trade_%d", maxOpenTrade),
			},
		})
	}
	// Create an InlineKeyboardMarkup with the buttons
	replyMarkup := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: inlineKeyboard,
	}
	// check if reply or edit
	if !update {
		_, err := ctx.EffectiveMessage.Reply(b, fmt.Sprintf("Choose the maximum open trades:"), &gotgbot.SendMessageOpts{
			ParseMode:   "HTML",
			ReplyMarkup: replyMarkup,
		})
		if err != nil {
			return fmt.Errorf("failed to send start message: %w", err)
		}
	} else {
		_, _, err := ctx.EffectiveMessage.EditText(b, fmt.Sprintf("Choose the maximum open trades:"), &gotgbot.EditMessageTextOpts{
			ParseMode:   "HTML",
			ReplyMarkup: replyMarkup,
		})
		if err != nil {
			return fmt.Errorf("failed to send start message: %w", err)
		}
	}

	return nil
}

func (tgBot *TgBot) setStrategyCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	return tgBot.setStrategy(b, ctx, false)
}
func (tgBot *TgBot) setRiskPercentageCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	return tgBot.setRiskPercentage(b, ctx, false)
}

func (tgBot *TgBot) setChannelBreakevenCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	return tgBot.setChannelBreakeven(b, ctx, false)
}

func (tgBot *TgBot) setDailyProfitGoalCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	return tgBot.setDailyProfitGoal(b, ctx, false)
}

func (tgBot *TgBot) setRiskPercentage(b *gotgbot.Bot, ctx *ext.Context, update bool) error {
	// Définir une liste de pourcentages de risque disponibles
	percentages := []float64{0.5, 1, 1.5, 2, 3, 5}

	// Générer un clavier inline basé sur les pourcentages
	var inlineKeyboard [][]gotgbot.InlineKeyboardButton
	for _, percentage := range percentages {
		currentRiskPercentage := tgBot.RedisClient.GetRiskPercentage() // Obtenir le pourcentage actuel depuis Redis
		text := fmt.Sprintf("%.2f%%", percentage)
		if currentRiskPercentage == percentage {
			text = text + " ✅" // Ajouter une coche si le pourcentage est actuellement sélectionné
		}
		inlineKeyboard = append(inlineKeyboard, []gotgbot.InlineKeyboardButton{
			{
				Text:         text,
				CallbackData: fmt.Sprintf("risk_%f", percentage),
			},
		})
	}

	// Créer le clavier Inline
	replyMarkup := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: inlineKeyboard,
	}

	// Vérifier si l'on édite ou envoie un nouveau message
	if !update {
		_, err := ctx.EffectiveMessage.Reply(b, "Choisissez le pourcentage de risque pour vos trades :", &gotgbot.SendMessageOpts{
			ParseMode:   "HTML",
			ReplyMarkup: replyMarkup,
		})
		if err != nil {
			return fmt.Errorf("failed to send risk percentage message: %w", err)
		}
	} else {
		_, _, err := ctx.EffectiveMessage.EditText(b, "Choisissez le pourcentage de risque pour vos trades :", &gotgbot.EditMessageTextOpts{
			ParseMode:   "HTML",
			ReplyMarkup: replyMarkup,
		})
		if err != nil {
			return fmt.Errorf("failed to edit risk percentage message: %w", err)
		}
	}

	return nil
}

func (tgBot *TgBot) GetBotStatus(b *gotgbot.Bot, ctx *ext.Context) error {
	// display all bot current settings
	// on off status
	text := ""
	if tgBot.RedisClient.IsBotOn() {
		text = "Status :  ON (✅)"
	} else {
		text = "Status : OFF (❌)"
	}
	text = text + "\n-------------------------"

	// current trade volume
	text = text + "\nTrade Volume : 📈" + fmt.Sprintf("%.2f", tgBot.RedisClient.GetDefaultTradingVolume())
	// separator
	text = text + "\n-------------------------"
	// strategy
	text = text + "\nStrategy 🤔: " + tgBot.RedisClient.GetStrategy()
	text = text + "\n-------------------------"

	// symboles
	text = text + "\nSymbols 💸: " + strings.Join(tgBot.RedisClient.GetSymbols(), "\n")
	text = text + "\n-------------------------"

	// daily profit goal
	text = text + "\nDaily Profit Goal 💰: " + fmt.Sprintf("%.2f", tgBot.RedisClient.GetDailyProfitGoal())
	text = text + "\n-------------------------"

	// current profit reached
	text = text + "\nCurrent Profit 💰: " + fmt.Sprintf("%.2f", tgBot.getTodayProfit())
	text = text + "\n-------------------------"

	ctx.EffectiveMessage.Reply(b, text, nil)
	// channels
	tgBot.paginateChannels(b, ctx, -1)
	// set chat id
	chatId := ctx.EffectiveChat.Id
	tgBot.RedisClient.SetChatId(chatId)
	return nil
}

func (tgBot *TgBot) setStrategy(b *gotgbot.Bot, ctx *ext.Context, b2 bool) error {
	// Create the inline keyboard buttons
	// list of volumes
	strategies := []string{"3TP", "TP1", "TP2"}
	// generate inlineKeyboard base on strategies
	var inlineKeyboard [][]gotgbot.InlineKeyboardButton
	for _, strategy := range strategies {
		// current volule
		currentStrategy := tgBot.RedisClient.GetStrategy()
		// limit to 2
		text := strategy
		if currentStrategy == strategy {
			text = text + " ✅"
		}
		inlineKeyboard = append(inlineKeyboard, []gotgbot.InlineKeyboardButton{
			{
				Text:         text,
				CallbackData: fmt.Sprintf("strategy_%s", strategy),
			},
		})
	}

	replyMarkup := gotgbot.InlineKeyboardMarkup{InlineKeyboard: inlineKeyboard}
	// check if reply or edit
	if !b2 {
		_, err := ctx.EffectiveMessage.Reply(b, fmt.Sprintf("Choose the strategy you want to trade:"), &gotgbot.SendMessageOpts{
			ParseMode:   "HTML",
			ReplyMarkup: replyMarkup,
		})
		if err != nil {
			return fmt.Errorf("failed to send start message: %w", err)
		}
	} else {
		_, _, err := ctx.EffectiveMessage.EditText(b, fmt.Sprintf("Choose the strategy you want to trade:"), &gotgbot.EditMessageTextOpts{
			ParseMode:   "HTML",
			ReplyMarkup: replyMarkup,
		})
		if err != nil {
			return fmt.Errorf("failed to send start message: %w", err)
		}

	}
	return nil
}

func (tgBot *TgBot) setSymbolsCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	return tgBot.setSymbols(b, ctx, 0)
}

// set symbols available for trading
func (tgBot *TgBot) setSymbols(b *gotgbot.Bot, ctx *ext.Context, page int) error {
	symbolsPerPage := 20
	// Récupérer la liste des symboles depuis MetaTrader
	symbols, errS := fetchAllSymbols(tgBot.AppConfig.MetaApiEndpoint, tgBot.AppConfig.MetaApiAccountID, tgBot.AppConfig.MetaApiToken)
	if errS != nil {
		return fmt.Errorf("failed to fetch symbols: %w", errS)
	}

	// Filtrer les symboles sélectionnés et non sélectionnés
	var selectedSymbols []string
	var unselectedSymbols []string
	for _, symbol := range symbols {
		if tgBot.RedisClient.IsSymbolExist(symbol) {
			selectedSymbols = append(selectedSymbols, symbol) // Symboles déjà sélectionnés
		} else {
			unselectedSymbols = append(unselectedSymbols, symbol) // Symboles non sélectionnés
		}
	}

	// Concaténer les symboles sélectionnés et non sélectionnés, les sélectionnés restant toujours en haut
	allSymbols := append(selectedSymbols, unselectedSymbols...)

	// Calculer le nombre total de pages
	totalPages := (len(unselectedSymbols) + len(selectedSymbols) + symbolsPerPage - 1) / symbolsPerPage
	if page >= totalPages {
		page = totalPages - 1
	}
	if page < 0 {
		page = 0
	}

	// Déterminer l'intervalle pour paginer les symboles non sélectionnés
	start := page * symbolsPerPage
	end := start + symbolsPerPage
	currentSymbols := []string{}
	if len(allSymbols) > 0 {
		if end > len(allSymbols) {
			end = len(allSymbols)
		}
		currentSymbols = allSymbols[start:end]
	}

	// Créer la liste complète des symboles (sélectionnés toujours en haut)
	displaySymbols := currentSymbols

	// Créer le clavier inline avec les boutons pour chaque symbole
	var inlineKeyboard [][]gotgbot.InlineKeyboardButton
	// add a button to allow all symbols
	textAuthorizeAll := "Authorize All"
	if tgBot.RedisClient.GetAllSymbols() {
		textAuthorizeAll = textAuthorizeAll + " ✅"
	}
	inlineKeyboard = append(inlineKeyboard, []gotgbot.InlineKeyboardButton{
		{
			Text:         textAuthorizeAll,
			CallbackData: fmt.Sprintf("authorize_all_symb"),
		},
	})
	for _, symbol := range displaySymbols {
		text := symbol
		if tgBot.RedisClient.IsSymbolExist(symbol) {
			text = text + " ✅"
		}
		inlineKeyboard = append(inlineKeyboard, []gotgbot.InlineKeyboardButton{
			{
				Text: text,

				CallbackData: fmt.Sprintf("symbol_%s", symbol),
			},
		})
	}

	// Ajouter les boutons de pagination uniquement si nécessaire
	var paginationButtons []gotgbot.InlineKeyboardButton
	if page > 0 {
		paginationButtons = append(paginationButtons, gotgbot.InlineKeyboardButton{
			Text:         "⬅️ Précédent",
			CallbackData: fmt.Sprintf("page_%d", page-1),
		})
	}
	if page < totalPages-1 {
		paginationButtons = append(paginationButtons, gotgbot.InlineKeyboardButton{
			Text:         "Suivant ➡️",
			CallbackData: fmt.Sprintf("page_%d", page+1),
		})
	}
	if len(paginationButtons) > 0 {
		inlineKeyboard = append(inlineKeyboard, paginationButtons)
	}

	replyMarkup := gotgbot.InlineKeyboardMarkup{InlineKeyboard: inlineKeyboard}

	// Vérifier si on édite un message ou on envoie un nouveau message
	if ctx.CallbackQuery != nil {
		// Éditer le message existant
		_, _, err := ctx.EffectiveMessage.EditText(b, fmt.Sprintf("Choisissez les symboles que vous voulez trader (Page %d/%d):", page+1, totalPages), &gotgbot.EditMessageTextOpts{
			ParseMode:   "HTML",
			ReplyMarkup: replyMarkup,
		})
		if err != nil {
			return fmt.Errorf("failed to edit symbols message: %w", err)
		}
	} else {
		// Envoyer un nouveau message
		_, err := ctx.EffectiveMessage.Reply(b, fmt.Sprintf("Choisissez les symboles que vous voulez trader (Page %d/%d):", page+1, totalPages), &gotgbot.SendMessageOpts{
			ParseMode:   "HTML",
			ReplyMarkup: replyMarkup,
		})
		if err != nil {
			return fmt.Errorf("failed to send symbols message: %w", err)
		}
	}

	return nil
}

// display list of selected channels and allow  user to enable or disable breakeven for each channel
func (tgBot *TgBot) setChannelBreakeven(b *gotgbot.Bot, ctx *ext.Context, update bool) error {
	// get all working channelIds
	channelIds := tgBot.RedisClient.GetChannels()
	// load telegram channelIds
	// list of channels telegram
	telegramChats := make([]tg.MessagesChats, 0)
	for _, channelId := range channelIds {
		inputChannles := make([]tg.InputChannelClass, 0)
		inputChannles = append(inputChannles, &tg.InputChannel{
			ChannelID: channelId,
		})
		telegramChannelById, errTg := tgBot.tdClient.API().ChannelsGetChannels(context.Background(), inputChannles)
		if errTg != nil {
			//		return fmt.Errorf("failed to get channels: %w", errTg)
		}
		if telegramChannelById == nil {
			continue
		} else {
			// cast to chats
			tMessageChat := telegramChannelById.(*tg.MessagesChats)
			telegramChats = append(telegramChats, *tMessageChat)
		}

	}
	// create inline keyboard
	var inlineKeyboard [][]gotgbot.InlineKeyboardButton
	// add allow for all channels button
	textAuthorize := "Allow All"
	if tgBot.RedisClient.IsBreakevenEnabledForAll() {
		textAuthorize = textAuthorize + " ✅"
	}
	inlineKeyboard = append(inlineKeyboard, []gotgbot.InlineKeyboardButton{
		{
			Text:         textAuthorize,
			CallbackData: fmt.Sprintf("breakeven_authorize_all"),
		},
	})
	{
		for _, channelItemA := range telegramChats {
			for _, channelItem := range channelItemA.Chats {
				if channel, ok := channelItem.(*tg.Channel); ok {
					text := channel.Title
					if tgBot.RedisClient.IsBreakevenEnabled(int(channel.ID)) {
						text = text + " ✅"
					}
					inlineKeyboard = append(inlineKeyboard, []gotgbot.InlineKeyboardButton{
						{
							Text:         text,
							CallbackData: fmt.Sprintf("channel_breakeven_%s", strconv.Itoa(int(channel.ID))),
						},
					})
				}
			}
		}
	}

	// Create an InlineKeyboardMarkup with the buttons
	replyMarkup := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: inlineKeyboard,
	}

	// check if reply or edit
	if !update {
		_, err := ctx.EffectiveMessage.Reply(b, fmt.Sprintf("Choose the channels you want to enable breakeven:"), &gotgbot.SendMessageOpts{
			ParseMode:   "HTML",
			ReplyMarkup: replyMarkup,
		})
		if err != nil {
			return fmt.Errorf("failed to send start message: %w", err)
		}
	} else {
		_, _, err := ctx.EffectiveMessage.EditText(b, fmt.Sprintf("Choose the channels you want to enable breakeven:"), &gotgbot.EditMessageTextOpts{
			ParseMode:   "HTML",
			ReplyMarkup: replyMarkup,
		})
		if err != nil {
			return fmt.Errorf("failed to send start message: %w", err)
		}
	}
	return nil
}

// set symbols available for trading
func (tgBot *TgBot) setTradeVolumeCallback(b *gotgbot.Bot,
	ctx *ext.Context) error {
	return tgBot.setTradeVolume(b, ctx, false)
}

func (tgBot *TgBot) setTradeVolume(b *gotgbot.Bot, ctx *ext.Context, update bool) error {

	// Create the inline keyboard buttons
	// list of volumes
	volumes := []float64{0.01, 0.02, 0.03, 0.04, 0.05, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1, 2, 10}
	// generate inlineKeyboard base on volumes
	var inlineKeyboard [][]gotgbot.InlineKeyboardButton
	for _, volume := range volumes {
		// current volule
		currentVolume := tgBot.RedisClient.GetDefaultTradingVolume()
		// limit to 2
		text := fmt.Sprintf("%.2f", (volume*100)/100)
		if currentVolume == volume {
			text = text + " ✅"
		}
		inlineKeyboard = append(inlineKeyboard, []gotgbot.InlineKeyboardButton{
			{
				Text:         text,
				CallbackData: fmt.Sprintf("volume_%f", (volume*100)/100),
			},
		})
	}
	// Create an InlineKeyboardMarkup with the buttons
	replyMarkup := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: inlineKeyboard,
	}
	// check if reply or edit
	if !update {
		_, err := ctx.EffectiveMessage.Reply(b, fmt.Sprintf("Choose the trade volume:"), &gotgbot.SendMessageOpts{
			ParseMode:   "HTML",
			ReplyMarkup: replyMarkup,
		})
		if err != nil {
			return fmt.Errorf("failed to send start message: %w", err)
		}
	} else {
		_, _, err := ctx.EffectiveMessage.EditText(b, fmt.Sprintf("Choose the trade volume:"), &gotgbot.EditMessageTextOpts{
			ParseMode:   "HTML",
			ReplyMarkup: replyMarkup,
		})
		if err != nil {
			return fmt.Errorf("failed to send start message: %w", err)
		}
	}

	return nil
}

func (tgBot *TgBot) setDailyProfitGoal(b *gotgbot.Bot, ctx *ext.Context, update bool) error {
	// Create the inline keyboard buttons
	// list of volumes
	profitGoals := []float64{50, 100, 150, 200, 250, 300, 400, 500, 600, 700, 800, 900, 1000}
	// generate inlineKeyboard base on volumes
	var inlineKeyboard [][]gotgbot.InlineKeyboardButton
	for _, profitGoal := range profitGoals {
		// current volule
		currentProfitGoal := tgBot.RedisClient.GetDailyProfitGoal()
		// limit to 2
		text := fmt.Sprintf("%.2f", profitGoal)
		if currentProfitGoal == profitGoal {
			text = text + " ✅"
		}
		inlineKeyboard = append(inlineKeyboard, []gotgbot.InlineKeyboardButton{
			{
				Text:         text,
				CallbackData: fmt.Sprintf("profit_goal_%f", profitGoal),
			},
		})
	}
	// Create an InlineKeyboardMarkup with the buttons
	replyMarkup := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: inlineKeyboard,
	}
	// check if reply or edit
	if !update {
		_, err := ctx.EffectiveMessage.Reply(b, fmt.Sprintf("Choose the daily profit goal:"), &gotgbot.SendMessageOpts{
			ParseMode:   "HTML",
			ReplyMarkup: replyMarkup,
		})
		if err != nil {
			return fmt.Errorf("failed to send start message: %w", err)
		}
	} else {
		_, _, err := ctx.EffectiveMessage.EditText(b, fmt.Sprintf("Choose the daily profit goal:"), &gotgbot.EditMessageTextOpts{
			ParseMode:   "HTML",
			ReplyMarkup: replyMarkup,
		})
		if err != nil {
			return fmt.Errorf("failed to send start message: %w", err)
		}
	}

	return nil
}

func (tgBot *TgBot) start(b *gotgbot.Bot, ctx *ext.Context) error {
	// Create the inline keyboard buttons
	inlineKeyboard := [][]gotgbot.InlineKeyboardButton{
		{
			gotgbot.InlineKeyboardButton{
				Text:         "Start Auto Trading",
				CallbackData: "start_trading",
			},
		},
		{
			gotgbot.InlineKeyboardButton{
				Text:         "Channels",
				CallbackData: "add_working_channels",
			},
		},
	}

	// Create an InlineKeyboardMarkup with the buttons
	replyMarkup := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: inlineKeyboard,
	}

	tgBot.RedisClient.SetBotOn()

	// Send the message with the inline keyboard
	_, err := ctx.EffectiveMessage.Reply(b, fmt.Sprintf("Hello, I'm @%s.\nI can help you with trading commands. Choose an option:", b.User.Username), &gotgbot.SendMessageOpts{
		ParseMode:   "HTML",
		ReplyMarkup: replyMarkup,
	})
	if err != nil {
		return fmt.Errorf("failed to send start message: %w", err)
	}
	return nil
}

func (tgBot *TgBot) stop(b *gotgbot.Bot, ctx *ext.Context) error {
	// Initialize the logger
	log, err := zap.NewDevelopment(zap.IncreaseLevel(zapcore.InfoLevel), zap.AddStacktrace(zapcore.FatalLevel))
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer log.Sync() // Ensure logs are flushed at the end

	// Set up authentication flow
	flow := auth.NewFlow(authmanager.TerminalPrompt{PhoneNumber: "+33658532534"}, auth.SendCodeOptions{})

	// Create a new Telegram client from environment
	client, err := telegram.ClientFromEnvironment(telegram.Options{
		Logger: log,
	})
	if err != nil {
		return fmt.Errorf("failed to create telegram client: %w", err)
	}

	var user *tg.User
	err = client.Run(context.Background(), func(ctxA context.Context) error {
		// Authenticate if necessary
		if err := client.Auth().IfNecessary(ctxA, flow); err != nil {
			return errors.Wrap(err, "auth")
		}

		// Fetch user info
		user, err = client.Self(ctxA)
		if err != nil {
			return errors.Wrap(err, "call self")
		}

		tgBot.RedisClient.SetBotOff()
		// change title
		_, err = b.SetChatTitle(ctx.EffectiveChat.Id, "BorisLazyTrade ❌", &gotgbot.SetChatTitleOpts{
			RequestOpts: nil,
		})

		// Reply to the user about stopping the bot
		_, errA := ctx.EffectiveMessage.Reply(b, fmt.Sprintf("Bot has now stopped, %s!", user.FirstName), &gotgbot.SendMessageOpts{
			ParseMode: "HTML",
		})

		if errA != nil {
			return fmt.Errorf("failed to send stop message: %w", errA)
		}

		return nil
	})

	// Handle potential errors from the client run
	if err != nil {
		return fmt.Errorf("failed to run telegram client: %w", err)
	}

	return nil
}
func (tgBot *TgBot) start_trading(b *gotgbot.Bot, ctx *ext.Context) error {
	// Initialize the logger
	if tgBot.RedisClient.IsBotOn() {
		_, errA := ctx.EffectiveMessage.Reply(b, fmt.Sprintf("Bot is already on"), &gotgbot.SendMessageOpts{
			ParseMode: "HTML",
		})
		if errA != nil {
		}
	} else {
		tgBot.RedisClient.SetBotOn()

		// Reply to the user about stopping the bot
		_, errA := ctx.EffectiveMessage.Reply(b, fmt.Sprintf("Bot is on, Good luck!"), &gotgbot.SendMessageOpts{
			ParseMode: "HTML",
		})
		if errA != nil {
			return fmt.Errorf("failed to send stop message: %w", errA)
		}
	}
	_, err := b.SetChatTitle(ctx.EffectiveChat.Id, "BorisLazyTrade ✅", &gotgbot.SetChatTitleOpts{
		RequestOpts: nil,
	})
	if err != nil {
		return err
	}

	return nil
}

// Callback handler
func (tgBot *TgBot) handleCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	data := ctx.CallbackQuery.Data
	if strings.HasPrefix(data, "next_page_") {
		// Extraire l'offset de la callback data
		offsetStr := strings.TrimPrefix(data, "next_page_")
		offset, err := strconv.Atoi(offsetStr)
		if err != nil {
			return fmt.Errorf("invalid offset value")
		}

		// Appel de l'API Telegram avec le nouvel offset pour récupérer la prochaine page de channels
		return tgBot.paginateChannels(b, ctx, offset)
	}
	if strings.HasPrefix(data, "volume_") {
		volumeStr := strings.TrimPrefix(data, "volume_")
		volume, err := strconv.ParseFloat(volumeStr, 64)
		if err != nil {
			return fmt.Errorf("invalid volume value")
		}

		tgBot.RedisClient.SetDefaultTradingVolume(volume)
		// Confirmer la sélection à l'utilisateur
		return tgBot.setTradeVolume(b, ctx, true)
	}
	if strings.HasPrefix(data, "page_") {
		// Extraire le numéro de page
		pageStr := strings.TrimPrefix(data, "page_")
		page, err := strconv.Atoi(pageStr)
		if err != nil {
			return fmt.Errorf("failed to parse page number: %w", err)
		}

		// Appeler la fonction setSymbols avec la nouvelle page
		return tgBot.setSymbols(b, ctx, page)
	}
	/// symbols
	if strings.HasPrefix(data, "symbol_") {
		symbol := strings.TrimPrefix(data, "symbol_")

		if tgBot.RedisClient.IsSymbolExist(symbol) {
			tgBot.RedisClient.RemoveSymbol(symbol)
		} else {
			tgBot.RedisClient.AddSymbol(symbol)
		}
		tgBot.RedisClient.SetAllSymbols(false)
		return tgBot.setSymbols(b, ctx, 0)
	}

	if strings.HasPrefix(data, "select_channel_") {
		channelIDStr := strings.TrimPrefix(data, "select_channel_")
		channelID, err := strconv.Atoi(channelIDStr)
		if err != nil {
			return fmt.Errorf("invalid channel ID")
		}

		// Ajouter le channel sélectionné à Redis ou autre

		if tgBot.RedisClient.IsChannelExist(int64(channelID)) {
			tgBot.RedisClient.RemoveChannel(int64(channelID))
			//_, err = ctx.EffectiveMessage.Reply(b, fmt.Sprintf("Channel avec ID %d retiré de votre liste.", channelID), &gotgbot.SendMessageOpts{
			//	ParseMode: "HTML",
			//})

		} else {
			tgBot.RedisClient.AddChannel(int64(channelID))
			// Confirmer la sélection à l'utilisateur
			//_, err = ctx.EffectiveMessage.Reply(b, fmt.Sprintf("Channel avec ID %d ajouté à votre liste.", channelID), &gotgbot.SendMessageOpts{
			//	ParseMode: "HTML",
			//})
		}

		return tgBot.paginateChannels(b, ctx, 0)
	}
	if strings.HasPrefix(data, "risk_") {
		// Extraire le pourcentage de risque
		riskStr := strings.TrimPrefix(data, "risk_")
		risk, err := strconv.ParseFloat(riskStr, 64)
		if err != nil {
			return fmt.Errorf("failed to parse risk percentage: %w", err)
		}

		// Stocker le pourcentage de risque dans Redis
		tgBot.RedisClient.SetRiskPercentage(risk)

		// Répondre à l'utilisateur
		_, err = ctx.CallbackQuery.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text: "Pourcentage de risque mis à jour avec succès !",
		})
		if err != nil {
			return fmt.Errorf("failed to answer callback query: %w", err)
		}

		// Mettre à jour l'affichage
		return tgBot.setRiskPercentage(b, ctx, true)
	}
	// strattegy
	if strings.HasPrefix(data, "strategy_") {
		strategy := strings.TrimPrefix(data, "strategy_")
		tgBot.RedisClient.SetStrategy(strategy)
		return tgBot.setStrategy(b, ctx, true)
	}

	// breakeven
	if strings.HasPrefix(data, "breakeven_authorize_all") {
		if tgBot.RedisClient.IsBreakevenEnabledForAll() {
			tgBot.RedisClient.SetBreakevenEnabledForAll(false)
		} else {
			tgBot.RedisClient.SetBreakevenEnabledForAll(true)
		}
		return tgBot.setChannelBreakeven(b, ctx, true)
	}
	if strings.HasPrefix(data, "channel_breakeven_") {
		channelIDStr := strings.TrimPrefix(data, "channel_breakeven_")
		channelID, err := strconv.Atoi(channelIDStr)
		if err != nil {
			return fmt.Errorf("invalid channel ID")
		}

		// Ajouter le channel sélectionné à Redis ou autre

		if tgBot.RedisClient.IsBreakevenEnabled(channelID) {
			tgBot.RedisClient.SetBreakevenEnabled(channelID, false)
		} else {
			tgBot.RedisClient.SetBreakevenEnabled(channelID, true)
		}
		tgBot.RedisClient.SetBreakevenEnabledForAll(false)

		return tgBot.setChannelBreakeven(b, ctx, true)
	}

	if strings.HasPrefix(data, "profit_goal_") {
		profitGoalStr := strings.TrimPrefix(data, "profit_goal_")
		profitGoal, err := strconv.ParseFloat(profitGoalStr, 64)
		if err != nil {
			return fmt.Errorf("failed to parse profit goal: %w", err)
		}

		// Stocker le pourcentage de risque dans Redis
		tgBot.RedisClient.SetDailyProfitGoal(profitGoal)

		// Répondre à l'utilisateur
		_, err = ctx.CallbackQuery.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text: "Daily profit goal updated successfully!",
		})
		if err != nil {
			return fmt.Errorf("failed to answer callback query: %w", err)
		}

		// Mettre à jour l'affichage
		return tgBot.setDailyProfitGoal(b, ctx, true)
	}
	// channel volume
	if strings.HasPrefix(data, "channel_volume_") {
		// if we click on a channel volume show the list of volumes to be set
		channelIDStr := strings.TrimPrefix(data, "channel_volume_")
		channelID, err := strconv.Atoi(channelIDStr)
		if err != nil {
			return fmt.Errorf("invalid channel ID")
		}
		return tgBot.selectChannelVolume(b, ctx, channelID)
	}
	// select channel volume
	if strings.HasPrefix(data, "select_chan_volume_") {
		// Extraire le channel ID et le volume
		parts := strings.Split(strings.TrimPrefix(data, "select_chan_volume_"), "_")
		channelID, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("invalid channel ID")
		}
		volume, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return fmt.Errorf("invalid volume")
		}

		// Stocker le volume du channel dans Redis
		tgBot.RedisClient.SetChannelVolume(channelID, volume)

		// Mettre à jour l'affichage
		return tgBot.selectChannelVolume(b, ctx, channelID)
	}
	// max open trades
	if strings.HasPrefix(data, "max_open_trades_") {
		maxOpenTradesStr := strings.TrimPrefix(data, "max_open_trades_")
		maxOpenTrades, err := strconv.Atoi(maxOpenTradesStr)
		if err != nil {
			return fmt.Errorf("failed to parse max open trades: %w", err)
		}

		// Stocker le nombre de trades ouverts max dans Redis
		tgBot.RedisClient.SetMaxOpenTrades(maxOpenTrades)

		// Répondre à l'utilisateur
		_, err = ctx.CallbackQuery.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text: "Max open trades updated successfully!",
		})
		if err != nil {
			return fmt.Errorf("failed to answer callback query: %w", err)
		}

		// Mettre à jour l'affichage
		return tgBot.setMaxOpenTrades(b, ctx, true)
	}

	switch ctx.CallbackQuery.Data {
	case "start_trading":
		// Add your logic to start trading
		return tgBot.start_trading(b, ctx)
	case "stop_trading":
		// Send source code or provide information

		// You could also call the source function here
		return tgBot.stop(b, ctx)
	case "add_working_channels":
		// Send source code or provide information

		// You could also call the source function here
		return tgBot.paginateChannels(b, ctx, 0)
	case "set_volume":
		return tgBot.setTradeVolume(b, ctx, false)
	case "set_channel_volume":
		return tgBot.setChannelVolume(b, ctx, true)
	case "set_daily_profit_goal":
		return tgBot.setDailyProfitGoal(b, ctx, false)
	case "authorize_all_symb":
		{
			if tgBot.RedisClient.GetAllSymbols() {
				tgBot.RedisClient.SetAllSymbols(false)
			} else {
				tgBot.RedisClient.SetAllSymbols(true)
			}
			return tgBot.setSymbols(b, ctx, 0)
		}
	default:
	}
	return nil
}

// select channel volume add a previous button to go back to the list of channels
func (tgBot *TgBot) selectChannelVolume(b *gotgbot.Bot, ctx *ext.Context, channelID int) error {
	// Create the inline keyboard buttons
	// list of volumes
	volumes := []float64{0.01, 0.02, 0.03, 0.04, 0.05, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1, 2, 10}
	// generate inlineKeyboard base on volumes
	var inlineKeyboard [][]gotgbot.InlineKeyboardButton
	// previous button with emoji back : ⬅️
	inlineKeyboard = append(inlineKeyboard, []gotgbot.InlineKeyboardButton{
		{
			Text:         " ⬅️ Back",
			CallbackData: "set_channel_volume",
		},
	})
	for _, volume := range volumes {
		// current volule
		currentVolume := tgBot.RedisClient.GetChannelVolume(channelID)
		// limit to 2
		text := fmt.Sprintf("%.2f", (volume*100)/100)
		if currentVolume == volume {
			text = text + " ✅"
		}

		inlineKeyboard = append(inlineKeyboard, []gotgbot.InlineKeyboardButton{
			{
				Text:         text,
				CallbackData: fmt.Sprintf("select_chan_volume_%d_%f", channelID, (volume*100)/100),
			},
		})
	}
	// Create an InlineKeyboardMarkup with the buttons
	replyMarkup := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: inlineKeyboard,
	}
	_, _, err := ctx.EffectiveMessage.EditText(b, fmt.Sprintf("Choose the trade volume for the channel:"), &gotgbot.EditMessageTextOpts{
		ParseMode:   "HTML",
		ReplyMarkup: replyMarkup,
	})
	if err != nil {
		return fmt.Errorf("failed to send start message: %w", err)
	}

	return nil
}

func (tgBot *TgBot) addWorkingChannels(b *gotgbot.Bot, ctx *ext.Context) error {
	return tgBot.paginateChannels(b, ctx, -1)
}

func (tgBot *TgBot) paginateChannels(b *gotgbot.Bot, ctx *ext.Context, offset int) error {
	log, err := zap.NewDevelopment(zap.IncreaseLevel(zapcore.InfoLevel), zap.AddStacktrace(zapcore.FatalLevel))
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer log.Sync()

	client, err := telegram.ClientFromEnvironment(telegram.Options{Logger: log})
	if err != nil {
		return fmt.Errorf("failed to create telegram client: %w", err)
	}

	err = client.Run(context.Background(), func(ctxA context.Context) error {
		dialogs, err := client.API().MessagesGetDialogs(ctxA, &tg.MessagesGetDialogsRequest{
			OffsetID:   offset, // Utiliser l'offset ici
			OffsetPeer: &tg.InputPeerEmpty{},
			Limit:      500, // Nombre d'éléments à récupérer
		})
		if err != nil {
			return fmt.Errorf("failed to get dialogs: %w", err)
		}
		var selectedChannels []tg.Channel
		var unselectedChannels []tg.Channel

		// Gérer les différents types de dialogues
		switch d := dialogs.(type) {
		case *tg.MessagesDialogs:
			for _, chat := range d.Chats {
				if channel, ok := chat.(*tg.Channel); ok {
					if tgBot.RedisClient.IsChannelExist(channel.ID) {
						selectedChannels = append(selectedChannels, *channel)
					} else {
						unselectedChannels = append(unselectedChannels, *channel)
					}
				}
			}
		case *tg.MessagesDialogsSlice:
			for _, chat := range d.Chats {
				if channel, ok := chat.(*tg.Channel); ok {
					if tgBot.RedisClient.IsChannelExist(channel.ID) {
						selectedChannels = append(selectedChannels, *channel)
					} else {
						unselectedChannels = append(unselectedChannels, *channel)
					}
				}
			}
		default:
			return fmt.Errorf("unsupported dialog type")
		}

		// Créer un tableau combiné avec les canaux sélectionnés en premier
		var inlineKeyboard [][]gotgbot.InlineKeyboardButton
		for _, channel := range selectedChannels {
			inlineKeyboard = append(inlineKeyboard, []gotgbot.InlineKeyboardButton{
				{
					Text:         fmt.Sprintf("%s (✅)", channel.Title),
					CallbackData: fmt.Sprintf("select_channel_%d", channel.ID),
				},
			})
		}
		for _, channel := range unselectedChannels {
			inlineKeyboard = append(inlineKeyboard, []gotgbot.InlineKeyboardButton{
				{
					Text:         fmt.Sprintf("%s (❌)", channel.Title),
					CallbackData: fmt.Sprintf("select_channel_%d", channel.ID),
				},
			})
		}

		// Re-encoder les channels dans le message
		replyMarkup := gotgbot.InlineKeyboardMarkup{InlineKeyboard: inlineKeyboard}
		if offset != -1 {

			// Envoyer le message avec les canaux
			_, _, err = ctx.EffectiveMessage.EditText(b, "Voici vos channels,cliquer pour autoriser/bloquer  : ", &gotgbot.EditMessageTextOpts{
				ReplyMarkup: replyMarkup,
			})
			if err != nil {
				return fmt.Errorf("failed to edit message: %w", err)
			}
		} else {
			// use reply
			_, err = ctx.EffectiveMessage.Reply(b, "Voici vos channels, cliquer pour autoriser/bloquer : ", &gotgbot.SendMessageOpts{
				ReplyMarkup: replyMarkup,
			})
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to run telegram client: %w", err)
	}
	return nil
}

// function to send a new parsed message from the bot to the current chat
func (tgBot *TgBot) sendMessage(message string, replyToMessageID int) (*gotgbot.Message, error) {
	//tgBot.Bot
	chatID := tgBot.RedisClient.GetChatId()
	if replyToMessageID != 0 {
		return tgBot.Bot.SendMessage(chatID, message, &gotgbot.SendMessageOpts{
			BusinessConnectionId: "",
			MessageThreadId:      0,
			ParseMode:            "",
			Entities:             nil,
			LinkPreviewOptions:   nil,
			DisableNotification:  false,
			ProtectContent:       false,
			MessageEffectId:      "",
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId: int64(replyToMessageID),
			},
			ReplyMarkup: nil,
			RequestOpts: nil,
		})
	} else {
		return tgBot.Bot.SendMessage(chatID, message, nil)
	}

}
