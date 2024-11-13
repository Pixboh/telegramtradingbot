package tgbot

import (
	"context"
	"encoding/json"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/go-faster/errors"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/updates"
	updhook "github.com/gotd/td/telegram/updates/hook"
	"github.com/gotd/td/tg"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"os/signal"
	"strconv"
	"tdlib/authmanager"
	"tdlib/config"
	"tdlib/redis_client"
)

type TgBot struct {
	terminalAuth     *authmanager.TerminalPrompt
	RedisClient      *redis_client.RedisClient
	AppConfig        *config.AppConfig
	tdClient         *telegram.Client
	Bot              *gotgbot.Bot
	CurrentPositions map[string]MetaApiPosition
}

func NewTgBot(appConfig config.AppConfig, redisClient *redis_client.RedisClient, terminalAuth *authmanager.TerminalPrompt) *TgBot {
	//botToken := "8138746202:AAGoUErnWQHgPem_avFGfheP48B8ltkF9Ns"
	botToken := appConfig.BotToken
	//Create bot from environment value.
	b, err := gotgbot.NewBot(botToken, nil)
	if err != nil {
		panic("failed to create new bot: " + err.Error())
	}
	return &TgBot{
		terminalAuth: authmanager.NewTerminalPrompt(appConfig),
		RedisClient:  redis_client.NewRedisClient(),
		AppConfig:    &appConfig,
		Bot:          b,
		// stock list of current positions MetaApiPosition
		CurrentPositions: make(map[string]MetaApiPosition),
	}
}

func (tgBot *TgBot) Start() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	// run cron
	c := cron.New()
	c.AddFunc("@every 1m", tgBot.checkCurrentPositions) // Adapter le délai
	// cron to run every day at 00:00
	c.AddFunc("0 0 * * *", tgBot.updateDailyInfo)
	// TODO remove line
	tgBot.checkCurrentPositions()
	c.Start()
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

	// listen to redis for new trading signals
	pubSub := tgBot.RedisClient.Rdb.Subscribe(context.Background(), "trading_signals")
	_, errSubscribing := pubSub.Receive(context.Background())
	if errSubscribing != nil {
		log.Error("Error subscribing to trading signals", zap.Error(errSubscribing))
		return errSubscribing
	}

	// Ensure we unsubscribe when done
	defer pubSub.Close()
	go func() {
		ch := pubSub.Channel()
		for msg := range ch {
			log.Info("Received message", zap.String("channel", msg.Channel), zap.String("payload", msg.Payload))
			var tradeRequest HandleRequestInput
			err := json.Unmarshal([]byte(msg.Payload), &tradeRequest)
			if err != nil {
				log.Error("Error unmarshalling trade request", zap.Error(err))
				continue
			}
			// get chat id
			chatId := tgBot.RedisClient.GetChatId()
			// send message to chat
			headerMessage := "Trading signal from " + tradeRequest.ChannelName
			botM := headerMessage + "\n" + tradeRequest.Message
			_, errSend := tgBot.Bot.SendMessage(chatId, botM, nil)
			if errSend != nil {
				log.Error("Error sending message to chat", zap.Error(errSend))
				continue
			}
			// handle request
			_, _, err = tgBot.HandleTradeRequest(tradeRequest)
			if err != nil {
				log.Error("Error handling trade request", zap.Error(err))
			}
		}
	}()

	flow := auth.NewFlow(authmanager.TerminalPrompt{PhoneNumber: tgBot.AppConfig.PhoneNumber}, auth.SendCodeOptions{})

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
	tgBot.tdClient = client
	go func() {
		tgBot.LaunchBorisBot(client)
	}()

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
		input := HandleRequestInput{
			MessageId:         m.ID,
			Message:           m.Message,
			ParentRequest:     replyTradeRequest,
			ChannelID:         messageChannel.ID,
			ChannelName:       messageChannel.Title,
			ChannelAccessHash: messageChannel.AccessHash,
		}
		err := tgBot.PushHandleRequestInputToRedis(&input)
		if err != nil {
			log.Error("Error pushing handle request input to redis", zap.Error(err))
			return err
		}

		return nil
	})

	errClientTg := client.Run(ctx, func(ctx context.Context) error {
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
	if errClientTg != nil {
		log.Error("Error running telegram client", zap.Error(errClientTg))
		return errClientTg
	}

	// listen to redis for new trading signals
	return nil

}

func (tgBot *TgBot) PushHandleRequestInputToRedis(input *HandleRequestInput) error {
	jsonL, _ := json.Marshal(input)
	nx := tgBot.RedisClient.Rdb.HSetNX(context.Background(), "trading_signals", strconv.Itoa(int(input.MessageId)), jsonL)
	if nx.Err() != nil {
		return nx.Err()
	}
	// publish to channel
	pub := tgBot.RedisClient.Rdb.Publish(context.Background(), "trading_signals", jsonL)
	if pub.Err() != nil {
		// print
		println("Error publishing to trading signals")
		return pub.Err()
	}
	return nil

}

func (tgBot *TgBot) updateDailyInfo() {
	// get account balance
	information, err := tgBot.getAccountInformation()
	if err != nil {
		return
	}
	balance := information.Balance
	// save account balance if 0
	if tgBot.getAccountBalance() == 0 {
		tgBot.RedisClient.SetAccountBalance(balance)
	}
}

// get account balance
func (tgBot *TgBot) getAccountBalance() float64 {
	balance := tgBot.RedisClient.GetAccountBalance()
	if balance == 0 {
		// get account balance
		information, err := tgBot.getAccountInformation()
		if err != nil {
			return 0
		}
		balance = information.Equity
		// save account balance
		tgBot.RedisClient.SetAccountBalance(balance)
	}
	return balance
}

//
