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
func (rdClient *RedisClient) SetTradingVolume(volume float64) {
	rdClient.Rdb.Set(context.Background(), "trading_volume", volume, 0)
}

// get trading default volume default to 0.001 if not set
func (rdClient *RedisClient) GetTradingVolume() float64 {
	volume := rdClient.Rdb.Get(context.Background(), "trading_volume")
	if volume.Err() != nil {
		return 0.001
	}
	volumeFloat, _ := strconv.ParseFloat(volume.Val(), 64)
	return volumeFloat
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
	tradeRequest := rdClient.Rdb.HGet(context.Background(), "trade_request", strconv.FormatInt(messageId, 10))
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
	// Obtenir le pourcentage depuis Redis
	risk := r.Rdb.Get(ctx, "risk_percentage")
	if risk.Err() != nil {
		return 0
	}
	riskFloat, _ := strconv.ParseFloat(risk.Val(), 64)
	return riskFloat
}
