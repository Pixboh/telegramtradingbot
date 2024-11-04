package redis_client

import (
	"context"
	"encoding/json"
	"github.com/redis/go-redis/v9"
	"strconv"
	"tdlib/custom_request"
)

var ctx = context.Background()

type RedisClient struct {
	Rdb *redis.Client
}

func NewRedisClient() *RedisClient {
	return &RedisClient{
		Rdb: redis.NewClient(&redis.Options{
			Addr:     "localhost:6379",
			Password: "", // no password set
			DB:       0,  // use default DB
		}),
	}
}
func (rdClient *RedisClient) sendLPush(key string, value string) {
	if rdClient.Rdb.Ping(ctx).Err() != nil {
		panic(rdClient.Rdb.Ping(ctx).Err())
	} else {
		err := rdClient.Rdb.LPush(ctx, key, value).Err()
		if err != nil {
			panic(err)
		}
		println("Sent to redis code " + value)
		rdClient.Rdb.Publish(ctx, "coupons", "new")

	}

}

func (rdClient *RedisClient) PushCoupon(coupons []*map[string]custom_request.GetCouponResponse) {
	for _, coupon := range coupons {
		for couponCode, couponEntity := range *coupon {
			jsonL, _ := json.Marshal(couponEntity)
			nx := rdClient.Rdb.HSetNX(context.Background(), "coupons", couponCode, jsonL)
			if nx.Err() != nil {
				panic(nx.Err().Error())
			}
			println("Sent to redis code " + couponCode)
			rdClient.Rdb.Publish(context.Background(), "coupons", "new")
		}
	}

}

func (rdClient *RedisClient) PublishAutoBot() {
	rdClient.Rdb.Publish(context.Background(), "autobet", "new")
}

// set bot on
func (rdClient *RedisClient) SetBotOn() {
	rdClient.Rdb.Set(context.Background(), "bot_status", "on", 0)
}

// set bot off
func (rdClient *RedisClient) SetBotOff() {
	rdClient.Rdb.Set(context.Background(), "bot_status", "off", 0)
}

// is bot on
func (rdClient *RedisClient) IsBotOn() bool {
	status := rdClient.Rdb.Get(context.Background(), "bot_status")
	if status.Err() != nil {
		return false
	}
	return status.Val() == "on"
}

// add channels to list channels
func (rdClient *RedisClient) AddChannel(channelId int64) {
	rdClient.Rdb.SAdd(context.Background(), "channels", channelId)
}

// remove channels from list channels
func (rdClient *RedisClient) RemoveChannel(channelId int64) {
	rdClient.Rdb.SRem(context.Background(), "channels", channelId)
}

// get all channels
func (rdClient *RedisClient) GetChannels() []int64 {
	channels := rdClient.Rdb.SMembers(context.Background(), "channels")
	if channels.Err() != nil {
		return nil
	}
	var result []int64
	for _, channel := range channels.Val() {
		chanelIntVal, _ := strconv.ParseInt(channel, 10, 64)
		result = append(result, chanelIntVal)
	}
	return result
}

// check channel in list channels
func (rdClient *RedisClient) IsChannelExist(channelId int64) bool {
	channels := rdClient.Rdb.SMembers(context.Background(), "channels")
	if channels.Err() != nil {
		return false
	}
	for _, channel := range channels.Val() {
		chanelIntVal, _ := strconv.ParseInt(channel, 10, 64)
		if chanelIntVal == channelId {
			return true
		}
	}
	return false
}

// trading default volume
func (rdClient *RedisClient) SetDefaultTradingVolume(volume float64) {
	rdClient.Rdb.Set(context.Background(), "trading_volume", volume, 0)
}

func (rdClient *RedisClient) GetDefaultTradingVolume() float64 {
	volume := rdClient.Rdb.Get(context.Background(), "trading_volume")
	if volume.Err() != nil {
		return 0.001
	}
	volumeFloat, _ := strconv.ParseFloat(volume.Val(), 64)
	return volumeFloat
}

// get trading default volume default to 0.001 if not set
func (rdClient *RedisClient) GetTradingVolume(channelId int) float64 {
	// if channel volume is defined, it will be used instead of the default volume
	volume := rdClient.GetChannelVolume(channelId)
	return volume
}

//type TradeRequest struct {
//	ActionType   string  `json:"actionType,omitempty"`
//	Symbol       string  `json:"symbol,omitempty"`
//	Volume       float64 `json:"volume,omitempty"`
//	StopLoss     float64 `json:"stopLoss,omitempty"`
//	TakeProfit1  float64 `json:"takeProfit1,omitempty"`
//	TakeProfit2  float64 `json:"takeProfit2,omitempty"`
//	TakeProfit3  float64 `json:"takeProfit3,omitempty"`
//	EntryZoneMin float64 `json:"entryZoneMin,omitempty"`
//	EntryZoneMax float64 `json:"entryZoneMax,omitempty"`
//}

// stock trade request link to the id of the telegram message
func (rdClient *RedisClient) SetTradeRequest(messageId int64, tradeRequestByte []byte) {
	rdClient.Rdb.HSet(context.Background(), "trade_request", strconv.FormatInt(messageId, 10), tradeRequestByte)
}

// get trade request by message id
func (rdClient *RedisClient) GetTradeRequest(messageId int64) []byte {
	// get first message id of the trade
	tradeMessageId := rdClient.GetTradeFirstMessageId(messageId)
	if tradeMessageId == 0 {
		tradeMessageId = messageId
	}
	tradeRequest := rdClient.Rdb.HGet(context.Background(), "trade_request", strconv.FormatInt(tradeMessageId, 10))
	if tradeRequest.Err() != nil {
		return nil
	}
	//var result tgbot.TradeRequest
	//json.Unmarshal([]byte(tradeRequest.Val()), &result)
	return []byte(tradeRequest.Val())
}

// add symboles
func (rdClient *RedisClient) AddSymbol(symbol string) {
	rdClient.Rdb.SAdd(context.Background(), "symbols", symbol)
}

// remove symboles
func (rdClient *RedisClient) RemoveSymbol(symbol string) {
	rdClient.Rdb.SRem(context.Background(), "symbols", symbol)
}

// get all symboles
func (rdClient *RedisClient) GetSymbols() []string {
	symbols := rdClient.Rdb.SMembers(context.Background(), "symbols")
	if symbols.Err() != nil {
		return nil
	}
	return symbols.Val()
}

// is symbol
func (rdClient *RedisClient) IsSymbolExist(symbol string) bool {
	// if all symbols return true
	if rdClient.GetAllSymbols() {
		return true
	}
	symbols := rdClient.Rdb.SMembers(context.Background(), "symbols")
	if symbols.Err() != nil {
		return false
	}
	for _, s := range symbols.Val() {
		if s == symbol {
			return true
		}
	}
	return false
}

// boolean to authorize all symbols
func (rdClient *RedisClient) SetAllSymbols(allSymbols bool) {
	rdClient.Rdb.Set(context.Background(), "all_symbols", allSymbols, 0)
}

// get all symbols
func (rdClient *RedisClient) GetAllSymbols() bool {
	allSymbols := rdClient.Rdb.Get(context.Background(), "all_symbols")
	if allSymbols.Err() != nil {
		return false
	}
	allSymbolsBool, _ := strconv.ParseBool(allSymbols.Val())
	return allSymbolsBool
}

// set strategy
func (rdClient *RedisClient) SetStrategy(strategy string) {
	rdClient.Rdb.Set(context.Background(), "strategy", strategy, 0)
}

// get strategy
func (rdClient *RedisClient) GetStrategy() string {
	strategy := rdClient.Rdb.Get(context.Background(), "strategy")
	if strategy.Err() != nil {
		return ""
	}
	return strategy.Val()
}

// set chat id
func (rdClient *RedisClient) SetChatId(chatId int64) {
	rdClient.Rdb.Set(context.Background(), "chat_id", chatId, 0)
}

// get chat id
func (rdClient *RedisClient) GetChatId() int64 {
	chatId := rdClient.Rdb.Get(context.Background(), "chat_id")
	if chatId.Err() != nil {
		return 0
	}
	chatIdInt, _ := strconv.ParseInt(chatId.Val(), 10, 64)
	return chatIdInt
}

// store a list of trade request key
// add trade key
func (rdClient *RedisClient) AddTradeKey(tradeKey string) {
	rdClient.Rdb.SAdd(context.Background(), "trade_keys", tradeKey)
}

// remove trade key
func (rdClient *RedisClient) RemoveTradeKey(tradeKey string) {
	rdClient.Rdb.SRem(context.Background(), "trade_keys", tradeKey)
}

// get all trade keys
func (rdClient *RedisClient) GetTradeKeys() []string {
	tradeKeys := rdClient.Rdb.SMembers(context.Background(), "trade_keys")
	if tradeKeys.Err() != nil {
		return nil
	}
	return tradeKeys.Val()
}

// link trade key to the id of the telegram message
func (rdClient *RedisClient) SetTradeKeyMessageId(tradeKey string, messageId int64) {
	rdClient.Rdb.HSet(context.Background(), "trade_key_message_id", tradeKey, strconv.FormatInt(messageId, 10))
}

// get message id by trade key
func (rdClient *RedisClient) GetTradeKeyMessageId(tradeKey string) int64 {
	messageId := rdClient.Rdb.HGet(context.Background(), "trade_key_message_id", tradeKey)
	if messageId.Err() != nil {
		return 0
	}
	messageIdInt, _ := strconv.ParseInt(messageId.Val(), 10, 64)
	return messageIdInt
}

// get trade key by message id
func (rdClient *RedisClient) GetTradeKeyByMessageId(messageId int64) string {
	tradeKey := rdClient.Rdb.HGet(context.Background(), "trade_key_message_id", strconv.FormatInt(messageId, 10))
	if tradeKey.Err() != nil {
		return ""
	}
	return tradeKey.Val()
}

// is trade key
func (rdClient *RedisClient) IsTradeKeyExist(tradeKey string) bool {
	tradeKeys := rdClient.Rdb.SMembers(context.Background(), "trade_keys")
	if tradeKeys.Err() != nil {
		return false
	}
	for _, tk := range tradeKeys.Val() {
		if tk == tradeKey {
			return true
		}
	}
	return false
}

// position message
// set message id
func (rdClient *RedisClient) SetPositionMessageId(positionId string, messageId int64) {
	rdClient.Rdb.HSet(context.Background(), "position_id", positionId, strconv.FormatInt(messageId, 10))
}

// get message id
func (rdClient *RedisClient) GetPositionMessageId(positionId string) int64 {
	messageId := rdClient.Rdb.HGet(context.Background(), "position_id", positionId)
	if messageId.Err() != nil {
		return 0
	}
	messageIdInt, _ := strconv.ParseInt(messageId.Val(), 10, 64)
	return messageIdInt
}

func (r *RedisClient) SetRiskPercentage(risk float64) {
	// Stocker le pourcentage dans Redis
	r.Rdb.Set(ctx, "risk_percentage", risk, 0)
}

func (r *RedisClient) GetRiskPercentage() float64 {
	defaultRisk := 0.01
	// Obtenir le pourcentage depuis Redis
	risk := r.Rdb.Get(ctx, "risk_percentage")
	if risk.Err() != nil {
		return defaultRisk
	}
	riskFloat, _ := strconv.ParseFloat(risk.Val(), 64)
	if riskFloat == 0 {
		return defaultRisk
	}
	return riskFloat
}

// we can have the same trade in multiple messages. We need to store the first message id of the trade and then store the following message ids on the same trade
// we dont have trade id so we use the first message id as key
func (rdClient *RedisClient) SetFirstTradeMessageId(firstMessageId int64, messageId int64) {
	rdClient.Rdb.HSet(context.Background(), "trade_message_id", strconv.FormatInt(messageId, 10), strconv.FormatInt(firstMessageId, 10))
}

// given a message : if it is the first message of a trade, return the message id of the first message else if it is a follow up message return the first message id
func (rdClient *RedisClient) GetTradeFirstMessageId(messageId int64) int64 {
	firstMessageId := rdClient.Rdb.HGet(context.Background(), "trade_message_id", strconv.FormatInt(messageId, 10))
	if firstMessageId.Err() != nil {
		return 0
	}
	firstMessageIdInt, _ := strconv.ParseInt(firstMessageId.Val(), 10, 64)
	return firstMessageIdInt
}

// is message id a first message of a trade
func (rdClient *RedisClient) IsTradeMessageIdExist(messageId int64) bool {
	tradeKeys := rdClient.Rdb.HKeys(context.Background(), "trade_message_id")
	if tradeKeys.Err() != nil {
		return false
	}
	for _, tk := range tradeKeys.Val() {
		if tk == strconv.FormatInt(messageId, 10) {
			return true
		}
	}
	return false
}

// is message id a follow up message of a trade
func (rdClient *RedisClient) IsTradeMessageIdFollowUpExist(messageId int64) bool {
	tradeKeys := rdClient.Rdb.HVals(context.Background(), "trade_message_id")
	if tradeKeys.Err() != nil {
		return false
	}
	for _, tk := range tradeKeys.Val() {
		if tk == strconv.FormatInt(messageId, 10) {
			return true
		}
	}
	return false
}

func (rdClient *RedisClient) GetDailyProfitGoal() float64 {
	profit := rdClient.Rdb.Get(ctx, "daily_profit_goal")
	if profit.Err() != nil {
		return 0
	}
	profitFloat, _ := strconv.ParseFloat(profit.Val(), 64)
	return profitFloat
}

func (rdClient *RedisClient) SetDailyProfitGoal(profit float64) {
	rdClient.Rdb.Set(ctx, "daily_profit_goal", profit, 0)
}

func (rdClient *RedisClient) SetBotStatus(s string) {
	rdClient.Rdb.Set(ctx, "bot_status", s, 0)
}

func (rdClient *RedisClient) GetBotStatus() string {
	status := rdClient.Rdb.Get(ctx, "bot_status")
	if status.Err() != nil {
		return ""
	}
	return status.Val()
}

// enable breakeven for all
func (rdClient *RedisClient) EnableBreakeven() {
	rdClient.Rdb.Set(ctx, "breakeven_enabled_for_all", "true", 0)
}

// disable breakeven for all
func (rdClient *RedisClient) DisableBreakeven() {
	rdClient.Rdb.Set(ctx, "breakeven_enabled_for_all", "false", 0)
}

// is breakeven enabled for all
func (rdClient *RedisClient) IsBreakevenEnabledForAll() bool {
	enabled := rdClient.Rdb.Get(ctx, "breakeven_enabled_for_all")
	if enabled.Err() != nil {
		return true
	}
	return enabled.Val() == "true"
}

func (rdClient *RedisClient) IsBreakevenEnabled(id int) bool {
	// is breakeven enabled for all
	if rdClient.IsBreakevenEnabledForAll() {
		return true
	}
	enabled := rdClient.Rdb.HGet(ctx, "breakeven_enabled", strconv.Itoa(id))
	if enabled.Err() != nil {
		return false
	}
	return enabled.Val() == "1"
}
func (rdClient *RedisClient) SetBreakevenEnabled(id int, enabled bool) {
	rdClient.Rdb.HSet(ctx, "breakeven_enabled", strconv.Itoa(id), enabled)
}

func (rdClient *RedisClient) SetBreakevenEnabledForAll(b bool) {
	if b {
		rdClient.EnableBreakeven()
	} else {
		rdClient.DisableBreakeven()
	}
}

func (rdClient *RedisClient) GetMaxOpenTrades() int {
	defaultOpenedTrades := 10
	max := rdClient.Rdb.Get(ctx, "max_open_trades")
	if max.Err() != nil {
		return defaultOpenedTrades
	}
	maxInt, _ := strconv.Atoi(max.Val())
	// default to ten
	if maxInt == 0 {
		return defaultOpenedTrades
	}
	return maxInt
}

func (rdClient *RedisClient) SetMaxOpenTrades(max int) {
	rdClient.Rdb.Set(ctx, "max_open_trades", max, 0)
}

// if channel volume is defined, it will be used instead of the default volume
func (rdClient *RedisClient) GetChannelVolume(i int) float64 {
	defaultVolume := rdClient.GetDefaultTradingVolume()
	volume := rdClient.Rdb.HGet(ctx, "channel_volume", strconv.Itoa(i))
	if volume.Err() != nil {
		return defaultVolume
	}
	volumeFloat, _ := strconv.ParseFloat(volume.Val(), 64)
	if volumeFloat == 0 {
		return defaultVolume
	}
	return volumeFloat
}

func (rdClient *RedisClient) SetChannelVolume(i int, volume float64) {
	rdClient.Rdb.HSet(ctx, "channel_volume", strconv.Itoa(i), volume)
}

func (rdClient *RedisClient) SetAccountBalance(balance float64) {
	rdClient.Rdb.Set(ctx, "account_balance", balance, 0)
}

func (rdClient *RedisClient) GetAccountBalance() float64 {
	balance := rdClient.Rdb.Get(ctx, "account_balance")
	if balance.Err() != nil {
		return 0
	}
	balanceFloat, _ := strconv.ParseFloat(balance.Val(), 64)
	return balanceFloat
}
