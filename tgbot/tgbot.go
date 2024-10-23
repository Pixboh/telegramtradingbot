package tgbot

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/go-faster/errors"
	"github.com/gotd/td/bin"
	"github.com/gotd/td/tdp"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/updates"
	updhook "github.com/gotd/td/telegram/updates/hook"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"os/signal"
	"reflect"
	"tdlib/authmanager"
	"tdlib/config"
	"tdlib/redis_client"
	"time"
)

type TgBot struct {
	terminalAuth *authmanager.TerminalPrompt
	RedisClient  *redis_client.RedisClient
	AppConfig    *config.AppConfig
	tdClient     *telegram.Client
	Bot          *gotgbot.Bot
}

func NewTgBot(appConfig config.AppConfig, redisClient *redis_client.RedisClient, terminalAuth *authmanager.TerminalPrompt) *TgBot {
	//botToken := "8138746202:AAGoUErnWQHgPem_avFGfheP48B8ltkF9Ns"
	botToken := appConfig.BotToken
	// Create bot from environment value.
	b, err := gotgbot.NewBot(botToken, nil)
	if err != nil {
		panic("failed to create new bot: " + err.Error())
	}
	return &TgBot{
		terminalAuth: authmanager.NewTerminalPrompt(appConfig),
		RedisClient:  redis_client.NewRedisClient(),
		AppConfig:    &appConfig,
		Bot:          b,
	}
}

func (tgBot *TgBot) Start() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := tgBot.run(ctx); err != nil {
		panic(err)
	}
}

func (tgBot *TgBot) run(ctx context.Context) error {

	// run boris bot

	log, _ := zap.NewDevelopment(zap.IncreaseLevel(zapcore.InfoLevel), zap.AddStacktrace(zapcore.FatalLevel))
	defer func() { _ = log.Sync() }()
	d := tg.NewUpdateDispatcher()
	gaps := updates.New(updates.Config{
		Handler: d,
		Logger:  log.Named("gaps"),
	})

	// Authentication flow handles authentication process, like prompting for code and 2FA password.
	//flow := auth.NewFlow(authmanager.TerminalPrompt{PhoneNumber: "+221771307579"}, auth.SendCodeOptions{})
	flow := auth.NewFlow(authmanager.TerminalPrompt{PhoneNumber: tgBot.AppConfig.PhoneNumber}, auth.SendCodeOptions{})

	// Initializing client from environment.
	// Available environment variables:
	// 	APP_ID:         app_id of Telegram app.
	// 	APP_HASH:       app_hash of Telegram app.
	// 	SESSION_FILE:   path to session file
	// 	SESSION_DIR:    path to session directory, if SESSION_FILE is not set
	//
	//remove current session first

	client, err := telegram.ClientFromEnvironment(telegram.Options{
		Logger:        log,
		UpdateHandler: gaps,
		Middlewares: []telegram.Middleware{
			updhook.UpdateHook(gaps.Handle),
			//prettyMiddleware(),
		},
	})
	if err != nil {
		return err
	}
	go func() {
		tgBot.LaunchBorisBot(client)
	}()

	// check open api
	defaultVolume := tgBot.RedisClient.GetTradingVolume()
	openaiApiKey := tgBot.AppConfig.OpenAiToken
	metaApiToken := tgBot.AppConfig.MetaApiToken
	metaApiAccountId := tgBot.AppConfig.MetaApiAccountID
	//clientOpenApi := openai.NewClient(openaiApiKey)
	// Setup message update handlers.
	d.OnNewChannelMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewChannelMessage) error {

		log.Info("Channel message", zap.Any("message", u.Message))
		m, ok := u.Message.(*tg.Message)
		if ok && m.Out {
			log.Info("Message is not incoming from channel")
			return nil
		}
		if !messageIsTradingSignal(m) {
			log.Info("Message is not a trading signal")
			return nil
		}
		// get channel by loop e.Channels map[int64]*tg.Channel
		messageChannel := tg.Channel{}
		for _, channel := range e.Channels {
			messageChannel = *channel
			if !tgBot.RedisClient.IsChannelExist(channel.ID) {
				log.Info("Channel not included", zap.Int64("channel_id", channel.ID))
				return nil
			}
		}
		// check current symbole price
		to, isReplyTo := m.GetReplyTo()
		replyTradeRequest := &TradeRequest{}
		//replyMessage := nil
		if isReplyTo {
			if to != nil {
				s := to.String()
				if &s != nil {
					var errS error
					replyToMsgId, errS := ExtractReplyToMessageId(s)
					if errS != nil {
						log.Error("Error extracting reply to message id", zap.Error(errS))
						return errS
					}
					replyTradeBytes := tgBot.RedisClient.GetTradeRequest(int64(replyToMsgId))
					var replyTrade TradeRequest
					errUnmarshal := json.Unmarshal(replyTradeBytes, &replyTrade)
					if errUnmarshal != nil {
						log.Info("Error unmarshalling trade request", zap.Error(errUnmarshal))
						return errUnmarshal
					}
					if replyTrade.Symbol == "" {
						log.Info("No trade request found for this message")
						return nil
					}
					replyTradeRequest = &replyTrade
				}
			}
		}
		if replyTradeRequest.Symbol == "" {
			replyTradeRequest = nil
		}
		if !tgBot.RedisClient.IsBotOn() {
			log.Info("Bot is off")
			return nil
		}
		_, _, err := tgBot.HandleTradeRequest(ctx, m,
			openaiApiKey, metaApiAccountId, metaApiToken, defaultVolume, replyTradeRequest, messageChannel)
		if err != nil {
			log.Error("Error handling trade request", zap.Error(err))
			return err
		}

		return nil
	})

	return client.Run(ctx, func(ctx context.Context) error {
		// Perform auth if no session is available.
		if err := client.Auth().IfNecessary(ctx, flow); err != nil {
			return errors.Wrap(err, "auth")
		}
		// Fetch user info.
		user, err := client.Self(ctx)
		if err != nil {
			return errors.Wrap(err, "call self")
		}

		return gaps.Run(ctx, client.API(), user.ID, updates.AuthOptions{
			OnStart: func(ctx context.Context) {
				log.Info("Gaps started")
			},
		})
	})
}

func prettyMiddleware() telegram.MiddlewareFunc {
	return func(next tg.Invoker) telegram.InvokeFunc {
		return func(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
			fmt.Println("→", formatObject(input))
			start := time.Now()
			if err := next.Invoke(ctx, input, output); err != nil {
				fmt.Println("←", err)
				return err
			}

			fmt.Printf("← (%s) %s\n", time.Since(start).Round(time.Millisecond), formatObject(output))

			return nil
		}
	}
}

func formatObject(input interface{}) string {
	o, ok := input.(tdp.Object)
	if !ok {
		// Handle tg.*Box values.
		rv := reflect.Indirect(reflect.ValueOf(input))
		for i := 0; i < rv.NumField(); i++ {
			if v, ok := rv.Field(i).Interface().(tdp.Object); ok {
				return formatObject(v)
			}
		}

		return fmt.Sprintf("%T (not object)", input)
	}
	return tdp.Format(o)
}

func IsPriceInEntryZone(price float64, entryMin, entryMax float64) bool {
	// add a little margin base on the difference between max and min value to tolerate trading updates
	diff := entryMax - entryMin
	marginToTolerate := diff * 0.5
	if entryMin > 0 {
		if entryMax > 0 {
			if price < entryMin-marginToTolerate || price > entryMax+marginToTolerate {
				return false
			}
		} else {
			if price < entryMin-marginToTolerate {
				return false
			}
		}
	}
	return true

}
