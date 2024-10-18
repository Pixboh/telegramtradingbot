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
	dispatcher.AddHandler(handlers.NewCommand("set_symbols", tgBot.setSymbolsCallback))
	dispatcher.AddHandler(handlers.NewCommand("set_strategy", tgBot.setStrategyCallback))
	dispatcher.AddHandler(handlers.NewCommand("status", tgBot.GetBotStatus))

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

	// Idle, to keep updates coming in, and avoid bot stopping.
	updater.Idle()
}

func (tgBot *TgBot) setStrategyCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	return tgBot.setStrategy(b, ctx, false)
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
	text = text + "\nTrade Volume : 📈" + fmt.Sprintf("%.2f", tgBot.RedisClient.GetTradingVolume())
	// separator
	text = text + "\n-------------------------"
	// strategy
	text = text + "\nStrategy 🤔: " + tgBot.RedisClient.GetStrategy()
	text = text + "\n-------------------------"

	// symboles
	text = text + "\nSymbols 💸: " + strings.Join(tgBot.RedisClient.GetSymbols(), "\n")
	text = text + "\n-------------------------"

	ctx.EffectiveMessage.Reply(b, text, nil)
	// channels
	tgBot.paginateChannels(b, ctx, -1)
	return nil
}

func (tgBot *TgBot) setStrategy(b *gotgbot.Bot, ctx *ext.Context, b2 bool) error {
	// Create the inline keyboard buttons
	// list of volumes
	strategies := []string{"3TP", "TP1", "TP2", "TP3"}
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
	symbolsPerPage := 50
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
	_ = append(selectedSymbols, unselectedSymbols...)

	// Calculer le nombre total de pages
	totalPages := (len(unselectedSymbols) + symbolsPerPage - 1) / symbolsPerPage
	if page >= totalPages {
		page = totalPages - 1
	}
	if page < 0 {
		page = 0
	}

	// Déterminer l'intervalle pour paginer les symboles non sélectionnés
	start := page * symbolsPerPage
	end := start + symbolsPerPage
	if end > len(unselectedSymbols) {
		end = len(unselectedSymbols)
	}
	currentSymbols := unselectedSymbols[start:end]

	// Créer la liste complète des symboles (sélectionnés toujours en haut)
	displaySymbols := append(selectedSymbols, currentSymbols...)

	// Créer le clavier inline avec les boutons pour chaque symbole
	var inlineKeyboard [][]gotgbot.InlineKeyboardButton
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
		currentVolume := tgBot.RedisClient.GetTradingVolume()
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

		tgBot.RedisClient.SetTradingVolume(volume)
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
	// strattegy
	if strings.HasPrefix(data, "strategy_") {
		strategy := strings.TrimPrefix(data, "strategy_")
		tgBot.RedisClient.SetStrategy(strategy)
		return tgBot.setStrategy(b, ctx, true)
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
	default:
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
