package tgbot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gotd/td/tg"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

// Function to place a trade and retrieve the response
func executeTrade(endpoint string, trade MetaApiTradeRequest, accountId, authToken string) (*TradeResponse, error) {
	// Convert TradeRequest to JSON
	tradeJSON, err := json.Marshal(trade)
	if err != nil {
		return nil, fmt.Errorf("error marshalling trade request: %v", err)
	}

	// Define MetaApi endpoint URL
	apiBaseUrl := endpoint
	url := fmt.Sprintf(apiBaseUrl+"/users/current/accounts/%s/trade", accountId)

	// Create HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(tradeJSON))
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %v", err)
	}

	// Set headers
	req.Header.Set("auth-token", authToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Execute HTTP request
	client := &http.Client{Timeout: time.Second * 10}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending trade request: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		// response body string
		reponseBody := ""
		if resp.Body != nil {
			bodyBytes, _ := ioutil.ReadAll(resp.Body)
			reponseBody = string(bodyBytes)
		}
		return nil, fmt.Errorf("failed to execute trade, status code: %d body : %s", resp.StatusCode, reponseBody)
	}

	// Read response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	// Parse response JSON into TradeResponse struct
	var tradeResponse TradeResponse
	err = json.Unmarshal(body, &tradeResponse)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling response: %v", err)
	}

	// Return the response object
	return &tradeResponse, nil
}

func fetchCurrentPrice(symbol, metaApiAccountId, metaApiToken string) (*MetaApiPriceResponse, error) {
	url := fmt.Sprintf("https://mt-client-api-v1.london.agiliumtrade.ai/users/current/accounts/%s/symbols/%s/current-price?keepSubscription=false", metaApiAccountId, symbol)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("auth-token", metaApiToken)
	req.Header.Add("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch current price")
	}

	var priceResponse MetaApiPriceResponse
	err = json.NewDecoder(resp.Body).Decode(&priceResponse)
	if err != nil {
		return nil, err
	}

	return &priceResponse, nil
}

func (tgBot *TgBot) HandleTradeRequest(ctx context.Context, message *tg.Message, openaiApiKey, metaApiAccountId,
	metaApiToken string, volume float64, parentRequest *TradeRequest) (*TradeRequest, *[]TradeResponse, error) {
	// Parse the incoming message into a TradeRequest
	symbols, err := fetchAllSymbols(tgBot.AppConfig.MetaApiEndpoint, tgBot.AppConfig.MetaApiAccountID, tgBot.AppConfig.MetaApiToken)
	if err != nil {
		log.Printf("Error fetching symbols: %v", err)
		return nil, nil, err
	}
	if parentRequest == nil {

		tradeRequest, err := tgBot.GptParseNewMessage(message.Message, openaiApiKey, symbols)
		if err != nil {
			log.Printf("Error parsing trade request: %v", err)
			return nil, nil, err
		}
		tradeRequest.Volume = volume
		tradeRequest = setTradeRequestEntryZone(tradeRequest)
		if !tgBot.RedisClient.IsSymbolExist(tradeRequest.Symbol) {
			log.Printf("Symbol %s is not allowed", tradeRequest.Symbol)
			return nil, nil, errors.New("symbol is not allowed")
		}
		errTrade := validateTradeValue(tradeRequest)

		// log fixed trade object
		log.Printf("TradeRequest struct: %+v\n", tradeRequest)
		if errTrade != nil {
			log.Printf("Error validating trade request: %v", errTrade)
			return nil, nil, errTrade
		}

		// Fetch current price from MetaApi
		//priceResponse, err := fetchCurrentPrice(tradeRequest.Symbol, metaApiAccountId, metaApiToken)
		//if err != nil {
		//	log.Printf("Error fetching price: %v", err)
		//	return nil, nil, err
		//}

		//var currentPrice float64
		//if tradeRequest.ActionType == "ORDER_TYPE_BUY" {
		//	currentPrice = priceResponse.Ask
		//} else if tradeRequest.ActionType == "ORDER_TYPE_SELL" {
		//	currentPrice = priceResponse.Bid
		//} else {
		//	// Handle other action types or return an error
		//	log.Println("Unsupported action type")
		//	return nil, nil, errors.New("unsupported action type")
		//}
		//
		//// Check if the current price is within the defined entry zone
		//if !IsPriceInEntryZone(currentPrice, tradeRequest.EntryZoneMin, tradeRequest.EntryZoneMax) {
		//	log.Println("Current price : " + strconv.FormatFloat(currentPrice, 'f', -1, 64) + " is not within the entry zone. Min" + strconv.FormatFloat(tradeRequest.EntryZoneMin, 'f', -1, 64) + " Max" + strconv.FormatFloat(tradeRequest.EntryZoneMax, 'f', -1, 64))
		//	return nil, nil, errors.New("current price is not within the entry zone")
		//}

		// Proceed with the trade
		metaApiRequests := ConvertToMetaApiTradeRequests(*tradeRequest)
		// trade response list
		var tradeResponses []TradeResponse
		strategy := tgBot.RedisClient.GetStrategy()
		for i, metaApiRequest := range metaApiRequests {
			if strategy == "TP1" {
				if i > 0 {
					break
				}
			} else if strategy == "TP2" {
				if i > 1 {
					break
				}
			} else if strategy == "TP3" {
				if i > 2 {
					break
				}
			}
			metaApiRequest.Volume = volume
			clientId := fmt.Sprintf("%s_%s_pending", strconv.Itoa(message.ID), "TP"+strconv.Itoa(i+1))
			// channelID_messageId_TP1
			metaApiRequest.ClientID = &clientId
			// try at least three time
			for j := 0; j < 3; j++ {
				trade, err := executeTrade(tgBot.AppConfig.MetaApiEndpoint, metaApiRequest, metaApiAccountId, metaApiToken)
				if err != nil {
					log.Printf("Error placing trade: %v", err)
					continue
				}
				if trade != nil && trade.NumericCode == 0 && trade.OrderId != nil {
					log.Printf("Trade placed successfully: %v", trade)
					// save trade to redis
					tradeResponses = append(tradeResponses, *trade)
					// exit
					break
				}
				// sleep 500ms
				time.Sleep(300 * time.Millisecond)
			}
		}

		return tradeRequest, &tradeResponses, nil
	} else {
		//here its an update of a given order so we need to fetch the order and update it
		tradeUpdate, err := GptParseUpdateMessage(message.Message, openaiApiKey)
		if err != nil {
			log.Printf("Error parsing trade request: %v", err)
			return nil, nil, err
		}
		positions, err := currentUserPositions(tgBot.AppConfig.MetaApiEndpoint, metaApiAccountId, metaApiToken)
		if err != nil {
			return nil, nil, err
		}
		currentMessagePositions := getPositionsByMessageId(positions, *parentRequest.MessageId)
		var tradeResponses []TradeResponse
		if tradeUpdate.UpdateType == "TP1_HIT" {
			//	if tp1 hit modify SL to tp 1 value on all other TP
			for _, position := range currentMessagePositions {
				metaApiRequest := MetaApiTradeRequest{
					ActionType: "POSITION_MODIFY",
					StopLoss:   parentRequest.TakeProfit1,
					PositionID: position.ID,
				}
				for j := 0; j < 3; j++ {
					trade, err := executeTrade(tgBot.AppConfig.MetaApiEndpoint, metaApiRequest, metaApiAccountId, metaApiToken)
					if err != nil {
						log.Printf("Error placing trade: %v", err)
						continue
					}
					if trade != nil && trade.NumericCode != 0 {
						log.Printf("Trade placed successfully: %v", trade)
						// save trade to redis
						tradeResponses = append(tradeResponses, *trade)
						// exit
						break
					}
					// sleep 500ms
					time.Sleep(300 * time.Millisecond)
				}

			}

		} else if tradeUpdate.UpdateType == "TP2_HIT" {
			// pull all SL to takeprofit2
			for _, position := range currentMessagePositions {
				metaApiRequest := MetaApiTradeRequest{
					ActionType: "POSITION_MODIFY",
					StopLoss:   parentRequest.TakeProfit2,
					PositionID: position.ID,
				}
				for j := 0; j < 3; j++ {
					trade, err := executeTrade(tgBot.AppConfig.MetaApiEndpoint, metaApiRequest, metaApiAccountId, metaApiToken)
					if err != nil {
						log.Printf("Error placing trade: %v", err)
						continue
					}
					if trade != nil && trade.NumericCode == 0 {
						log.Printf("Trade placed successfully: %v", trade)
						// save trade to redis
						tradeResponses = append(tradeResponses, *trade)
						// exit
						break
					}
					// sleep 500ms
					time.Sleep(300 * time.Millisecond)
				}
			}
		} else if tradeUpdate.UpdateType == "TP3_HIT" {
			// pull all SL to takeprofit3
			for _, position := range currentMessagePositions {
				metaApiRequest := MetaApiTradeRequest{
					ActionType: "POSITION_MODIFY",
					StopLoss:   parentRequest.TakeProfit3,
					PositionID: position.ID,
				}
				for j := 0; j < 3; j++ {
					trade, err := executeTrade(tgBot.AppConfig.MetaApiEndpoint, metaApiRequest, metaApiAccountId, metaApiToken)
					if err != nil {
						log.Printf("Error placing trade: %v", err)
						continue
					}
					if trade != nil && trade.NumericCode == 0 {
						log.Printf("Trade placed successfully: %v", trade)
						// save trade to redis
						tradeResponses = append(tradeResponses, *trade)
						// exit
						break
					}
					// sleep 500ms
					time.Sleep(300 * time.Millisecond)
				}
			}
		} else if tradeUpdate.UpdateType == "CLOSE_TRADE" {
			// close all positions
			for _, position := range currentMessagePositions {
				metaApiRequest := MetaApiTradeRequest{
					ActionType: "POSITION_CLOSE_ID",
					PositionID: position.ID,
				}
				trade, err := executeTrade(tgBot.AppConfig.MetaApiEndpoint, metaApiRequest, metaApiAccountId, metaApiToken)
				if err != nil {
					log.Printf("Error closing trade: %v", err)
					return nil, nil, err
				}
				if trade != nil && trade.NumericCode == 0 {
					log.Printf("Trade closed successfully")
				}
				tradeResponses = append(tradeResponses, *trade)
			}
		} else if tradeUpdate.UpdateType == "MODIFY_STOPLOSS" {
			// modify stop loss to the value given
			for _, position := range currentMessagePositions {
				metaApiRequest := MetaApiTradeRequest{
					ActionType: "POSITION_MODIFY",
					StopLoss:   *tradeUpdate.Value,
					PositionID: position.ID,
				}
				for j := 0; j < 3; j++ {
					trade, err := executeTrade(tgBot.AppConfig.MetaApiEndpoint, metaApiRequest, metaApiAccountId, metaApiToken)
					if err != nil {
						log.Printf("Error placing trade: %v", err)
						continue
					}
					if trade != nil && trade.NumericCode == 0 {
						log.Printf("Trade placed successfully: %v", trade)
						// save trade to redis
						tradeResponses = append(tradeResponses, *trade)
						// exit
						break
					}
					// sleep 500ms
					time.Sleep(300 * time.Millisecond)
				}
			}
		} else if tradeUpdate.UpdateType == "SL_TO_ENTRY_PRICE" {
			// modify stop loss to the value given
			for _, position := range currentMessagePositions {
				metaApiRequest := MetaApiTradeRequest{
					ActionType: "POSITION_MODIFY",
					StopLoss:   position.OpenPrice,
					PositionID: position.ID,
				}
				for j := 0; j < 3; j++ {
					trade, err := executeTrade(tgBot.AppConfig.MetaApiEndpoint, metaApiRequest, metaApiAccountId, metaApiToken)
					if err != nil {
						log.Printf("Error placing trade: %v", err)
						continue
					}
					if trade != nil && trade.NumericCode == 0 {
						log.Printf("Trade placed successfully: %v", trade)
						// save trade to redis
						tradeResponses = append(tradeResponses, *trade)
						// exit
						break
					}
					// sleep 500ms
					time.Sleep(300 * time.Millisecond)
				}
			}
		}
		//get list active positions

		return nil, &tradeResponses, nil
	}
}

func setTradeRequestEntryZone(request *TradeRequest) *TradeRequest {
	// set entryzone max if set to -1
	entryZoneMin := request.EntryZoneMin
	if request.EntryZoneMax == -1 {
		tp1Value := request.TakeProfit1
		stopLossValue := request.StopLoss
		// calculate max entry zone base on stop loss and tp1
		if request.ActionType == "ORDER_TYPE_BUY" {
			request.EntryZoneMax = entryZoneMin + ((tp1Value - stopLossValue) / 2)
		} else if request.ActionType == "ORDER_TYPE_SELL" {
			request.EntryZoneMin = entryZoneMin - ((tp1Value - stopLossValue) / 2)
			/// swap min and max
			request.EntryZoneMax = entryZoneMin
		}
	}
	return request
}

func validateTradeValue(r *TradeRequest) error {
	// check if takeprofit1 is set
	if r.TakeProfit1 == 0 {
		return errors.New("takeprofit1 is required")
	}
	// check if stoploss is set
	if r.StopLoss == 0 {
		return errors.New("stoploss is required")
	}
	// check if volume is set
	if r.Volume == 0 {
		return errors.New("volume is required")
	}
	// check if symbol is set
	if r.Symbol == "" {
		return errors.New("symbol is required")
	}
	// check if actionType is set
	if r.ActionType == "" {
		return errors.New("actionType is required")
	}
	// value coerence check
	// if buy
	if r.ActionType == "ORDER_TYPE_BUY" {
		if r.TakeProfit1 <= r.EntryZoneMin {
			return errors.New("takeprofit1 must be greater than entryZoneMin")
		}
		if r.StopLoss >= r.EntryZoneMin {
			return errors.New("stoploss must be less than entryZoneMin")
		}
		if r.TakeProfit1 <= r.StopLoss {
			return errors.New("takeprofit1 must be greater than stoploss")
		}
	} else if r.ActionType == "ORDER_TYPE_SELL" {
		if r.TakeProfit1 >= r.EntryZoneMin {
			return errors.New("takeprofit1 must be less than entryZoneMin")
		}
		if r.StopLoss <= r.EntryZoneMin {
			return errors.New("stoploss must be greater than entryZoneMin")
		}
		if r.TakeProfit1 >= r.StopLoss {
			return errors.New("takeprofit1 must be less than stoploss")
		}
	}

	return nil

}

func ExtractReplyToMessageId(input string) (int, error) {
	// Define a regular expression pattern to match "ReplyToMsgID:<some number>"
	re := regexp.MustCompile(`ReplyToMsgID:(\d+)`)

	// Find the first match in the input string
	matches := re.FindStringSubmatch(input)

	if len(matches) < 2 {
		return 0, fmt.Errorf("ReplyToMsgID not found in the input string")
	}

	// Convert the matched ReplyToMsgID to an integer
	replyToMsgID, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("failed to convert ReplyToMsgID to integer: %w", err)
	}

	return replyToMsgID, nil
}

// curl to get curent user positions
// curl --location 'https://mt-client-api-v1.london.agiliumtrade.ai/users/current/accounts/993fc6b0-60eb-47c2-bc71-f1c149275153/positions' \
// --header 'auth-token: bearerToken' \
// --header 'Accept: application/json'
func currentUserPositions(endpoint string, metaApiAccountId, metaApiToken string) ([]MetaApiPosition, error) {
	url := fmt.Sprintf("%s/users/current/accounts/%s/positions", endpoint, metaApiAccountId)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("auth-token", metaApiToken)
	req.Header.Add("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch current user positions")
	}

	var positions []MetaApiPosition
	err = json.NewDecoder(resp.Body).Decode(&positions)
	if err != nil {
		return nil, err
	}

	return positions, nil
}

// get position by messageID containing in the clientID field with. Example for clientId 6203_TP2_pending 6203 is the messageID
func getPositionsByMessageId(positions []MetaApiPosition, messageID int) []*MetaApiPosition {
	// search for position wich clientID start with messageID
	var result []*MetaApiPosition
	for _, position := range positions {
		if position.ClientID != "" {
			// regex to check if clientId start with message ID
			re := regexp.MustCompile(`^` + strconv.Itoa(messageID))
			if re.MatchString(position.ClientID) {
				result = append(result, &position)
			}
		}
	}
	return result
}
func getPositionsByMessageIdAndTP(positions []MetaApiPosition, messageID int, tpNumber int) []*MetaApiPosition {
	// search for position wich clientID start with messageID
	var result []*MetaApiPosition
	for _, position := range positions {
		if position.ClientID != "" {
			// regex to check if clientId start with message ID
			re := regexp.MustCompile(`^` + strconv.Itoa(messageID) + `_TP` + strconv.Itoa(tpNumber))
			if re.MatchString(position.ClientID) {
				result = append(result, &position)
			}
		}
	}
	return result
}

type MetaApiPosition struct {
	ID                          string  `json:"id"`
	Platform                    string  `json:"platform,omitempty"`
	Type                        string  `json:"type,omitempty"`
	Symbol                      string  `json:"symbol,omitempty"`
	Magic                       int     `json:"magic,omitempty"`
	Time                        string  `json:"time,omitempty"`
	BrokerTime                  string  `json:"brokerTime,omitempty"`
	UpdateTime                  string  `json:"updateTime,omitempty"`
	OpenPrice                   float64 `json:"openPrice,omitempty"`
	Volume                      float64 `json:"volume,omitempty"`
	Swap                        float64 `json:"swap,omitempty"`
	RealizedSwap                float64 `json:"realizedSwap,omitempty"`
	UnrealizedSwap              float64 `json:"unrealizedSwap,omitempty"`
	Commission                  float64 `json:"commission,omitempty"`
	RealizedCommission          float64 `json:"realizedCommission,omitempty"`
	UnrealizedCommission        float64 `json:"unrealizedCommission,omitempty"`
	RealizedProfit              float64 `json:"realizedProfit,omitempty"`
	Reason                      string  `json:"reason,omitempty"`
	AccountCurrencyExchangeRate float64 `json:"accountCurrencyExchangeRate,omitempty"`
	BrokerComment               string  `json:"brokerComment,omitempty"`
	ClientID                    string  `json:"clientId,omitempty"`
	StopLoss                    float64 `json:"stopLoss,omitempty"`
	TakeProfit                  float64 `json:"takeProfit,omitempty"`
	UnrealizedProfit            float64 `json:"unrealizedProfit,omitempty"`
	Profit                      float64 `json:"profit,omitempty"`
	CurrentPrice                float64 `json:"currentPrice,omitempty"`
	CurrentTickValue            float64 `json:"currentTickValue,omitempty"`
	UpdateSequenceNumber        float64 `json:"updateSequenceNumber,omitempty"`
}

type MetaApiTradeRequest struct {
	ActionType          string            `json:"actionType,omitempty,omitempty"`
	Symbol              string            `json:"symbol,omitempty"`
	Volume              float64           `json:"volume,omitempty"`
	OpenPrice           float64           `json:"openPrice,omitempty"`
	StopLimitPrice      float64           `json:"stopLimitPrice,omitempty"`
	StopLoss            float64           `json:"stopLoss,omitempty"`
	TakeProfit          float64           `json:"takeProfit,omitempty"`
	StopLossUnits       string            `json:"stopLossUnits,omitempty"`
	TakeProfitUnits     string            `json:"takeProfitUnits,omitempty"`
	StopPriceBase       string            `json:"stopPriceBase,omitempty"`
	OpenPriceUnits      string            `json:"openPriceUnits,omitempty"`
	OpenPriceBase       string            `json:"openPriceBase,omitempty"`
	StopLimitPriceUnits string            `json:"stopLimitPriceUnits,omitempty"`
	StopLimitPriceBase  string            `json:"stopLimitPriceBase,omitempty"`
	TrailingStopLoss    *TrailingStopLoss `json:"trailingStopLoss,omitempty"`
	OrderID             string            `json:"orderId,omitempty"`
	PositionID          string            `json:"positionId,omitempty"`
	CloseByPositionID   string            `json:"closeByPositionId,omitempty"`
	Comment             *string           `json:"comment,omitempty"`
	ClientID            *string           `json:"clientId,omitempty"`
	Magic               float64           `json:"magic,omitempty"`
	Slippage            float64           `json:"slippage,omitempty"`
	FillingModes        []string          `json:"fillingModes,omitempty"`
	Expiration          *Expiration       `json:"expiration,omitempty"`
}

type TrailingStopLoss struct {
	Distance struct {
		Distance float64 `json:"distance,omitempty"`
		Units    string  `json:"units,omitempty"`
	} `json:"distance,omitempty"`
	Threshold struct {
		Thresholds []struct {
			Threshold float64 `json:"threshold,omitempty"`
			StopLoss  float64 `json:"stopLoss,omitempty"`
		} `json:"thresholds,omitempty"`
		Units         string `json:"units,omitempty"`
		StopPriceBase string `json:"stopPriceBase,omitempty"`
	} `json:"threshold,omitempty"`
}

type Expiration struct {
	Type string `json:"type,omitempty"`
	Time string `json:"time,omitempty"`
}

func ConvertToMetaApiTradeRequests(trade TradeRequest) []MetaApiTradeRequest {
	var metaApiRequests []MetaApiTradeRequest

	tpValues := []float64{trade.TakeProfit1, trade.TakeProfit2, trade.TakeProfit3}

	for _, tp := range tpValues {
		//comment := fmt.Sprintf("Trade for TP%d", i+1)

		if tp > 0 { // Only create requests for defined TPs
			metaTrade := MetaApiTradeRequest{
				ActionType: trade.ActionType,
				Symbol:     trade.Symbol,
				Volume:     trade.Volume, // Assuming volume is the same for all trades
				StopLoss:   trade.StopLoss,
				TakeProfit: tp,
				//OpenPriceUnits:      "RELATIVE_BALANCE_PERCENTAGE",
				//OpenPriceBase:       "OPEN_PRICE",
				//StopLimitPriceUnits: "RELATIVE_POINTS",
				//StopLimitPriceBase:  "STOP_LIMIT_PRICE",
				//Comment: &comment,
				//FillingModes: []string{"ORDER_FILLING_IOC"},
				//Expiration: Expiration{
				//	Type: "ORDER_TIME_GTC",
				//	Time: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
				//},
			}

			metaApiRequests = append(metaApiRequests, metaTrade)
		}
	}

	return metaApiRequests
}

type TradeRequestMetaApi struct {
	ActionType   string     `json:"actionType,omitempty"`
	Symbol       string     `json:"symbol,omitempty"`
	Volume       float64    `json:"volume,omitempty"`
	StopLoss     float64    `json:"stopLoss,omitempty,omitempty"`
	TakeProfit   float64    `json:"takeProfit,omitempty,omitempty"` // This will be used for each TP trade
	EntryZoneMin float64    `json:"entryZoneMin,omitempty,omitempty"`
	EntryZoneMax float64    `json:"entryZoneMax,omitempty,omitempty"`
	OrderID      string     `json:"orderId,omitempty"`
	Comment      string     `json:"comment,omitempty"`
	ClientID     string     `json:"clientId,omitempty"`
	Slippage     float64    `json:"slippage,omitempty,omitempty"`
	Expiration   Expiration `json:"expiration,omitempty"`
}

type TradeResponse struct {
	NumericCode int     `json:"numericCode,omitempty"`
	StringCode  string  `json:"stringCode,omitempty"`
	Message     string  `json:"message,omitempty"`
	OrderId     *string `json:"orderId,omitempty"`
	PositionId  *string `json:"positionId,omitempty"`
}

type Trade struct {
	OrderId    string  `json:"orderId,omitempty"`
	PositionId string  `json:"positionId,omitempty"`
	Symbol     string  `json:"symbol,omitempty"`
	Volume     float64 `json:"volume,omitempty"`
	StopLoss   float64 `json:"stopLoss,omitempty"`
	TakeProfit float64 `json:"takeProfit,omitempty"`
	EntryPrice float64 `json:"entryPrice,omitempty"`
	Profit     float64 `json:"profit,omitempty"` // Peut être mis à jour après l'exécution du trade
}

type MetaApiPriceResponse struct {
	Symbol string  `json:"symbol"`
	Ask    float64 `json:"ask"`
	Bid    float64 `json:"bid"`
	// other fields you may want to include...
}

// get all symbos
// curl --location 'https://mt-client-api-v1.london.agiliumtrade.ai/users/current/accounts/<string>/symbols' \
// --header 'auth-token: <string>' \
// --header 'Accept: application/json'
func fetchAllSymbols(endpoint string, metaApiAccountId, metaApiToken string) ([]string, error) {
	url := fmt.Sprintf("%s/users/current/accounts/%s/symbols", endpoint, metaApiAccountId)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("auth-token", metaApiToken)
	req.Header.Add("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch symbols")
	}

	var symbols []string
	err = json.NewDecoder(resp.Body).Decode(&symbols)
	if err != nil {
		return nil, err
	}

	return symbols, nil
}
