package tgbot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gotd/td/tg"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
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

type HandleRequestInput struct {
	MessageId         int
	Message           string
	ParentRequest     *TradeRequest
	ChannelID         int64
	ChannelName       string
	ChannelAccessHash int64
}

func (tgBot *TgBot) HandleTradeRequest(input HandleRequestInput) (*TradeRequest, *[]TradeResponse, error) {
	// Parse the incoming message into a TradeRequest
	symbols, err := fetchAllSymbols(tgBot.AppConfig.MetaApiEndpoint, tgBot.AppConfig.MetaApiAccountID, tgBot.AppConfig.MetaApiToken)
	if err != nil {
		log.Printf("Error fetching symbols: %v", err)
		tgBot.sendMessage(fmt.Sprintf("❌ Error fetching symbols from MetaApi : %v", err), 0)
		return nil, nil, err
	}

	openaiApiKey := tgBot.AppConfig.OpenAiToken
	metaApiToken := tgBot.AppConfig.MetaApiToken
	metaApiAccountId := tgBot.AppConfig.MetaApiAccountID
	channel := tg.Channel{
		ID:    input.ChannelID,
		Title: input.ChannelName,
	}
	messageId := input.MessageId
	message := input.Message
	parentRequest := input.ParentRequest
	if parentRequest == nil {

		tradeRequest, err := tgBot.GptParseNewMessage(input.Message, tgBot.AppConfig.OpenAiToken, symbols)
		if err != nil {
			log.Printf("Error parsing trade request with Openai: %v", err)
			// send erreur with log to telegram
			tgBot.sendMessage(fmt.Sprintf("❌ Error parsing trade request: %v", err), 0)
			return nil, nil, err
		}

		// check if reached daily profit
		todayProfit := tgBot.getTodayProfit()
		dailyProfitGoal := tgBot.RedisClient.GetDailyProfitGoal()
		riskableProfit := -1.0
		if todayProfit >= dailyProfitGoal {
			riskableProfit = todayProfit - dailyProfitGoal
			if riskableProfit > 4 {
				log.Printf("Reached daily profit goal but still have riskable profit")
			} else {
				log.Printf("Reached daily profit goal")
				tgBot.sendMessage("❌ Daily profit goal reached", 0)
				return nil, nil, errors.New("daily profit goal reached")
			}

		}

		// check if reach loss limit
		if tgBot.reachedDailyLossLimit() {
			log.Printf("Reached daily loss limit")
			tgBot.sendMessage("❌ Daily loss limit reached", 0)
			return nil, nil, errors.New("daily loss limit reached")
		}
		// check symbol trend

		// get current position
		positions, err := tgBot.currentUserPositions(tgBot.AppConfig.MetaApiEndpoint, metaApiAccountId, metaApiToken)
		if err != nil {
			return nil, nil, err
		}
		if len(positions) > 0 && riskableProfit > 0 {
			// reach daily profit goal
			log.Printf("Reached daily profit goal")
			tgBot.sendMessage("❌ Daily profit goal reached", 0)
			return nil, nil, errors.New("daily profit goal reached")
		}

		// check for similar trades ongoing
		maxSimilarTrades := tgBot.RedisClient.GetMaxSimilarTrades()
		if tgBot.CountSimilarTrades(positions, *tradeRequest) >= maxSimilarTrades {
			log.Printf("Similar trade already exist")
			tgBot.sendMessage("❌ Similar trade already exist", 0)
			return nil, nil, errors.New("similar trade already exist")
		}

		// check if max trades position reached
		//	maxOpenedTrade := tgBot.RedisClient.GetMaxOpenTrades()
		//	if len(positions) > 0 && len(positions) > maxOpenedTrade {
		//		log.Printf("Max opened trades : %d reached", maxOpenedTrade)
		//		tgBot.sendMessage("❌ Skipping signal. Max opened trades : "+strconv.Itoa(maxOpenedTrade)+" reached", 0)
		//		return nil, nil, errors.New("max opened trades reached")
		//	}

		tradeRequest = setTradeRequestEntryZone(tradeRequest)
		if !tgBot.RedisClient.IsSymbolExist(tradeRequest.Symbol) {
			log.Printf("Symbol %s is not allowed", tradeRequest.Symbol)
			// sen message
			// Optional. Quoted part of the message to be replied to; 0-1024 characters after entities parsing. The quote must be an exact substring of the message to be replied to, including bold, italic, underline, strikethrough, spoiler, and custom_emoji entities. The message will fail to send if the quote isn't found in the original message.
			tgBot.sendMessage("❌ Symbol is not allowed", 0)
			return nil, nil, errors.New("symbol is not allowed")
		}
		strategy := tgBot.RedisClient.GetStrategy()

		// Fetch current price from MetaApi
		priceResponse, err := fetchCurrentPrice(tradeRequest.Symbol, metaApiAccountId, metaApiToken)
		if err != nil {
			log.Printf("Error fetching price: %v", err)
			return nil, nil, err
		}

		var currentPrice float64
		if tradeRequest.ActionType == "ORDER_TYPE_BUY" {
			currentPrice = priceResponse.Ask
			// if symbol trend is bearish and action is buy
		} else if tradeRequest.ActionType == "ORDER_TYPE_SELL" {
			currentPrice = priceResponse.Bid
			// if symbol trend is bullish and action is sell
		} else {
			// Handle other action types or return an error
			log.Println("Unsupported action type")
			return nil, nil, errors.New("unsupported action type")
		}

		// pass trade request to risk management to validate or reject the trade
		balance := tgBot.getAccountBalance()
		volume := tgBot.GetTradingDynamicVolume(tradeRequest, currentPrice, balance, int(input.ChannelID), riskableProfit)
		// do not trade if volume inferior to 0.01
		if volume < 0.01 {
			log.Printf("Volume less than 0.01")
			tgBot.sendMessage("❌ Volume less than 0.01", 0)
			return nil, nil, errors.New("volume less than 0.01")
		}
		tradeRequest.Volume = volume

		// avoid dboule trade
		if tgBot.RedisClient.IsTradeKeyExist(tradeRequest.GenerateTradeRequestKey()) {
			log.Printf("Trade already placed")
			// add the new message id if not already set
			if tgBot.RedisClient.GetTradeFirstMessageId(int64(messageId)) == 0 {
				// get the first message id
				firstMessageId := tgBot.RedisClient.GetTradeKeyMessageId(tradeRequest.GenerateTradeRequestKey())
				// set first message id for current message
				tgBot.RedisClient.SetFirstTradeMessageId(firstMessageId, int64(messageId))
			}
			// send message
			tgBot.sendMessage("❌ Trade already exist", 0)
			return nil, nil, errors.New("trade already exist")
		}
		// check if ongoing trades
		if tgBot.CheckIfTradeCanFit(positions, *tradeRequest, currentPrice) {
			log.Printf("Trade can fit")
		} else {
			log.Printf("Trade can't fit")
			tgBot.sendMessage("❌ Trade can't fit", 0)
			return nil, nil, errors.New("trade can't fit")
		}

		errTrade := tgBot.validateTradeValue(tradeRequest, strategy)
		// check if the same trade already exist

		// log fixed trade object
		log.Printf("TradeRequest struct: %+v\n", tradeRequest)
		if errTrade != nil {
			log.Printf("Error validating trade request: %v", errTrade)
			// send erreur with log to telegram
			tgBot.sendMessage(fmt.Sprintf("❌ Error validating trade request: %v", errTrade), 0)
			return nil, nil, errTrade
		}

		// Proceed with the trade
		metaApiRequests := ConvertToMetaApiTradeRequests(*tradeRequest, strategy)
		// trade response list
		var tradeResponses []TradeResponse
		tradeSuccess := false
		remainingVolume := tradeRequest.Volume
		for i, metaApiRequest := range metaApiRequests {
			if strategy == "TP1" {
				if i > 0 {
					break
				}
			} else if strategy == "TP2" {
				if i > 1 {
					break
				}
			} else if strategy == "3TP" {
				if i > 2 {
					break
				}
			}
			takeProfit := 0.0
			metaApiTradeVolume := remainingVolume
			if i == 0 {
				takeProfit = tradeRequest.TakeProfit1
				// tp1 should be 60% of the volume
				if len(metaApiRequests) == 1 {
					metaApiTradeVolume = tradeRequest.Volume
				} else {
					metaApiTradeVolume = tradeRequest.Volume * 0.7
				}
				remainingVolume = tradeRequest.Volume - metaApiTradeVolume

			} else if i == 1 {
				takeProfit = tradeRequest.TakeProfit2
				// tp2 should be 40% of the volume
				if len(metaApiRequests) == 2 {
					metaApiTradeVolume = tradeRequest.Volume * 0.3
				} else if len(metaApiRequests) == 3 {
					metaApiTradeVolume = tradeRequest.Volume * 0.2
				}
				remainingVolume = remainingVolume - metaApiTradeVolume
			} else if i == 2 {
				takeProfit = tradeRequest.TakeProfit3
				// tp3 should be 10% of the volume
				metaApiTradeVolume = remainingVolume

			}
			tpNumber := i + 1
			// 2 digits
			metaApiTradeVolume = math.Floor(metaApiTradeVolume*100) / 100
			// if volume is less than 0.01 skip trade²
			if metaApiTradeVolume < 0.01 {
				log.Printf("Volume less than 0.01")
				tgBot.sendMessage("❌ Volume less than 0.01", 0)
				return nil, nil, errors.New("volume less than 0.01")
			}
			metaApiRequest.Volume = &metaApiTradeVolume
			// concat channel id and channel initial
			chanelInitials := GenerateInitials(channel.Title) + "@" + strconv.Itoa(int(channel.ID))
			clientId := fmt.Sprintf("%s_%s_%s", chanelInitials, strconv.Itoa(int(messageId)), "TP"+strconv.Itoa(i+1))
			// channelID_messageId_TP1
			metaApiRequest.ClientID = &clientId

			// try at least three time
			for j := 0; j < 3; j++ {
				trade, err := executeTrade(tgBot.AppConfig.MetaApiEndpoint, metaApiRequest, metaApiAccountId, metaApiToken)
				if err != nil {
					log.Printf("Error placing trade: %v", err)
					if j == 2 {
						// generate error message with reason
						/**
						❌ Trade not placed
						🏀 Channel : @channel
						 Buy EURUSD
						 Entry Price: 1.1234
						 Stop Loss: 1.1200
						 Take Profit: 1.1300
						 ❌ Error: Invalid volume
						*/
						errorReason := fmt.Sprintf("Error request %v", err)
						messageText := fmt.Sprintf("❌ Trade not placed\n"+
							"🏀 Channel : %s\n %s %s\n  Stop Loss: %.2f\n TP%s: %.2f\n❌ Error: %s",
							channel.Title, tradeRequest.ActionType, tradeRequest.Symbol,
							tradeRequest.StopLoss, strconv.Itoa(tpNumber), takeProfit, errorReason)
						_, err := tgBot.sendMessage(messageText, 0)
						if err != nil {
							log.Printf("Error sending message: %v", err)
						}
					}
					continue
				}
				errorTrade := HandleTradeError(trade.NumericCode)
				if tradeErr, ok := errorTrade.(*TradeError); ok {
					if tradeErr.Type == Success {
						if !tradeSuccess {
							tradeSuccess = true
						}
						log.Printf("Trade placed successfully: %v", trade)
						// send fancy bot message with emoji to  inform usser the trade was placed with additional info about the provenance (message , channel )
						// ad next line separation also add also values info stopLoss takeprofit etc
						// example
						/**
						Trade placed
						⛏ ID : 1234
						🏀 Channel : @channel
						📈 Buy EURUSD
						🔴 Stop Loss: 1.1200
						🟢 Take Profit: 1.1300
						*/
						messageText := fmt.Sprintf("Trade placed\n⛏ ID : %s\n🏀 Channel : %s\n📈 %s %s\n🔴 SL: %.2f\n🟢 TP%s: %.2f",
							clientId, channel.Title, tradeRequest.ActionType, tradeRequest.Symbol, tradeRequest.StopLoss, strconv.Itoa(tpNumber), takeProfit)
						m, err := tgBot.sendMessage(messageText, 0)
						if err != nil {
							log.Printf("Error sending message: %v", err)
						}
						tgBot.RedisClient.SetPositionMessageId(*trade.PositionId, m.MessageId)
						break
					} else {
						// if last retry send error message
						if j == 2 {
							// generate error message with reason
							/**
							❌ Trade not placed
							🏀 Channel : @channel
							 Buy EURUSD
							 Entry Price: 1.1234
							 Stop Loss: 1.1200
							 Take Profit: 1.1300
							 ❌ Error: Invalid volume
							*/
							messageText := fmt.Sprintf("❌ Trade not placed\n"+
								"🏀 Channel : %s\n %s %s\n  Stop Loss: %.2f\n TP%s: %.2f\n❌ Error: %s",
								channel.Title, tradeRequest.ActionType, tradeRequest.Symbol, tradeRequest.StopLoss,
								strconv.Itoa(tpNumber), takeProfit, tradeErr.Description)
							_, err := tgBot.sendMessage(messageText, 0)
							if err != nil {
								log.Printf("Error sending message: %v", err)
								return nil, nil, err
							}
						}
						continue
					}
				} else {
					if j == 2 {
						// generate error message with reason
						/**
						❌ Trade not placed
						🏀 Channel : @channel
						 Entry Price: 1.1234
						 Stop Loss: 1.1200
						 Take Profit: 1.1300
						 ❌ Error: Invalid volume
						*/
						errorReason := fmt.Sprintf("Invalid code returned %v", trade)
						messageText := fmt.Sprintf("❌ Trade not placed\n🏀"+
							" Channel : %s\n %s %s\n  Stop Loss: %.2f\n TP%s: %.2f\n❌ Error: %s",
							channel.Title, tradeRequest.ActionType, tradeRequest.Symbol, tradeRequest.StopLoss,
							strconv.Itoa(tpNumber), takeProfit, errorReason)
						_, err := tgBot.sendMessage(messageText, 0)
						if err != nil {
							log.Printf("Error sending message: %v", err)
							return nil, nil, err
						}
					}

				}
			}
		}
		if tradeSuccess {
			// save trade request
			tradeRequest.MessageId = &messageId
			tradeRbytes, errJ := json.Marshal(tradeRequest)
			if errJ == nil {
				tgBot.RedisClient.SetTradeRequest(int64(messageId), tradeRbytes)
				// add trade key
				tradeKey := tradeRequest.GenerateTradeRequestKey()
				tgBot.RedisClient.AddTradeKey(tradeKey)
				// set trade key message id
				tgBot.RedisClient.SetTradeKeyMessageId(tradeKey, int64(messageId))

				return tradeRequest, &tradeResponses, nil
			}
		} else {
			return nil, nil, errors.New("trade not placed")
		}
	} else {
		//here its an update of a given order so we need to fetch the order and update it
		tradeUpdate, err := GptParseUpdateMessage(message, openaiApiKey)
		if err != nil {
			log.Printf("Error parsing trade request: %v", err)
			// send erreur with log to telegram
			tgBot.sendMessage(fmt.Sprintf("❌ Error parsing trade request: %v", err), 0)
			return nil, nil, err
		}
		positions, err := tgBot.currentUserPositions(tgBot.AppConfig.MetaApiEndpoint, metaApiAccountId, metaApiToken)
		if err != nil {
			return nil, nil, err
		}
		currentMessagePositions := getPositionsByMessageId(positions, *parentRequest.MessageId)
		var tradeResponses []TradeResponse
		if tradeUpdate.UpdateType == "TP1_HIT" {
			// do breakeven
			//	if tp1 hit modify SL to tp 1 value on all other TP
			if parentRequest.TakeProfit1 == -1 {
				// manual close tp1
				positionTP1 := getPositionByMessageIdAndTP(currentMessagePositions, *parentRequest.MessageId, 1)
				if positionTP1 != nil && positionTP1.TakeProfit == 0 {
					customPositions := []MetaApiPosition{*positionTP1}
					_ = tgBot.doCloseTrade(customPositions)
					positions, err = tgBot.currentUserPositions(tgBot.AppConfig.MetaApiEndpoint, metaApiAccountId, metaApiToken)
					if err != nil {
						return nil, nil, err
					}
					currentMessagePositions = getPositionsByMessageId(positions, *parentRequest.MessageId)
				}

			}
			if tgBot.RedisClient.IsBreakevenEnabled(int(channel.ID)) {
				tgBot.doBreakeven(currentMessagePositions, 1)
			}

		} else if tradeUpdate.UpdateType == "TP2_HIT" {
			// do breakeven
			//	if tp1 hit modify SL to tp 1 value on all other TP
			if parentRequest.TakeProfit2 == -1 {
				// manual close tp1
				positionTP1 := getPositionByMessageIdAndTP(currentMessagePositions, *parentRequest.MessageId, 2)
				if positionTP1 != nil && positionTP1.TakeProfit == 0 {
					customPositions := []MetaApiPosition{*positionTP1}
					_ = tgBot.doCloseTrade(customPositions)
					positions, err = tgBot.currentUserPositions(tgBot.AppConfig.MetaApiEndpoint, metaApiAccountId, metaApiToken)
					if err != nil {
						return nil, nil, err
					}
					currentMessagePositions = getPositionsByMessageId(positions, *parentRequest.MessageId)
				}

			}
			if tgBot.RedisClient.IsBreakevenEnabled(int(channel.ID)) {
				tgBot.doBreakeven(currentMessagePositions, 2)
			}
		} else if tradeUpdate.UpdateType == "TP3_HIT" {
			// do breakeven
			//	if tp1 hit modify SL to tp 1 value on all other TP
			if parentRequest.TakeProfit3 == -1 {
				// manual close tp1
				positionTP1 := getPositionByMessageIdAndTP(currentMessagePositions, *parentRequest.MessageId, 3)
				if positionTP1 != nil && positionTP1.TakeProfit == 0 {
					customPositions := []MetaApiPosition{*positionTP1}
					_ = tgBot.doCloseTrade(customPositions)
					positions, err = tgBot.currentUserPositions(tgBot.AppConfig.MetaApiEndpoint, metaApiAccountId, metaApiToken)
					if err != nil {
						return nil, nil, err
					}
					currentMessagePositions = getPositionsByMessageId(positions, *parentRequest.MessageId)
				}

			}
			if tgBot.RedisClient.IsBreakevenEnabled(int(channel.ID)) {
				tgBot.doBreakeven(currentMessagePositions, 3)
			}

		} else if tradeUpdate.UpdateType == "CLOSE_TRADE" {
			// close all positions
			errClodeTrade := tgBot.doCloseTrade(currentMessagePositions)
			if errClodeTrade != nil {

			}
		} else if tradeUpdate.UpdateType == "MODIFY_STOPLOSS" {
			// modify stop loss to the value given
			errModifyStopLoss := tgBot.doModifyStopLoss(parentRequest, tradeUpdate, currentMessagePositions, metaApiAccountId, metaApiToken)
			if errModifyStopLoss != nil {

			}

		} else if tradeUpdate.UpdateType == "SL_TO_ENTRY_PRICE" {
			// modify stop loss to the value given
			errSlToEntryPrice := tgBot.doSlToEntryPrice(currentMessagePositions)
			if errSlToEntryPrice != nil {

			}

		}
		//get list active positions

		return nil, &tradeResponses, nil
	}
	return nil, nil, errors.New("trade not placed")
}

func (tgBot *TgBot) CheckIfTradeCanFit(positions []MetaApiPosition, request TradeRequest, currentPrice float64) bool {
	// calculate total loss possible on all trades
	totalLoss := 0.0
	for _, position := range positions {
		stopLoss := position.StopLoss
		entryPrice := position.OpenPrice
		volume := position.Volume
		// stopLoss distance in pips
		pipsToStopLoss := calculatePips(entryPrice, stopLoss, position.Symbol)
		// use absolute value
		if pipsToStopLoss < 0 {
			pipsToStopLoss = pipsToStopLoss * -1
		}
		// calculate loss
		loss := pipsToStopLoss * volume
		totalLoss += loss
	}
	// get trading dynamic volume
	// get possible loss on the trade request
	possibleLoss := tgBot.GetTradeRequestPossibleLoss(&request, currentPrice)
	// check if possible loss is less than total loss
	totalLoss = totalLoss + possibleLoss
	lossLimitamount := tgBot.getDailyLossLimitAmount()
	if totalLoss > lossLimitamount {
		return false
	}
	return true
}

// trade are similar if they have the same symbol and the same direction and trade time is less than 1 hour
func (tgBot *TgBot) CountSimilarTrades(positions []MetaApiPosition, request TradeRequest) int {
	count := 0
	similarPositions := []MetaApiPosition{}
	for _, position := range positions {
		// count only tp1 positions
		if position.isBreakevenSetted() {
			// do not count breakeven positions
			continue
		}
		tpNumber := extractTPFromClientId(position.ClientID)
		if tpNumber != 1 {
			continue
		}
		// check if the trade is less than 1 hour
		tradeTimeString := position.Time
		tradeTimeTime, err := time.Parse(time.RFC3339, tradeTimeString)
		if err != nil {
			log.Printf("Error parsing trade time: %v", err)
			continue
		}
		positionType := "POSITION_TYPE_BUY"
		if request.ActionType == "ORDER_TYPE_SELL" {
			positionType = "POSITION_TYPE_SELL"
		}
		if position.Symbol == request.Symbol && position.Type == positionType {
			similarTradeMaxHour := tgBot.RedisClient.GetSimilarTradeMaxHour()
			if time.Since(tradeTimeTime).Hours() < similarTradeMaxHour {
				count++
				similarPositions = append(similarPositions, position)
			}
		}
	}

	return count
}

func (tgBot *TgBot) doSlToEntryPrice(positions []MetaApiPosition) error {
	// get entry price base on positions
	tradeSuccess := false
	// generate a telegram response for the bot
	botMessage := fmt.Sprintf("✅ SL to Entry Price 🎉")
	for _, position := range positions {
		newStopLoss := calculateNewStopLossPriceForBreakeven(position.OpenPrice, position.Type, position.Symbol)
		positionMessageId := tgBot.RedisClient.GetPositionMessageId(position.ID)
		metaApiRequest := MetaApiTradeRequest{
			ActionType: "POSITION_MODIFY",
			StopLoss:   &newStopLoss,
			PositionID: &position.ID,
			TakeProfit: &position.TakeProfit,
		}
		// place all positions stop loss to their open price
		for j := 0; j < 3; j++ {
			trade, err := executeTrade(tgBot.AppConfig.MetaApiEndpoint, metaApiRequest, tgBot.AppConfig.MetaApiAccountID,
				tgBot.AppConfig.MetaApiToken)
			//
			if err != nil {
				log.Printf("Error placing trade: %v", err)
				if j == 2 {
					// send parsed error message from bot
					// generate error message with reason
					botErrorMessage := fmt.Sprintf("❌ Failed moving SL to entry price")
					botErrorMessage = fmt.Sprintf("%s\n%s", botErrorMessage,
						fmt.Sprintf("❌ Error: %s", err))
					tgBot.sendMessage(botMessage, int(positionMessageId))
				}
				continue
			}
			errorTrade := HandleTradeError(trade.NumericCode)
			if tradeErr, ok := errorTrade.(*TradeError); ok {
				if tradeErr.Type == Success {
					if !tradeSuccess {
						tradeSuccess = true
					}
					if tradeErr.Type == Success {
						log.Printf("Trade update placed successfully: %v", trade)
						// append message to inform user that we moved the stop loss to the entry price of this current tp
						botMessage = fmt.Sprintf("%s\n%s", botMessage,
							fmt.Sprintf("➡️Moving SL to entry price"))
						// append with values on next  line
						botMessage = fmt.Sprintf("%s\n%s", botMessage,
							fmt.Sprintf("➡️Position ID: %s\nSL: %.2f -> %.2f", position.ID, position.StopLoss, position.OpenPrice))
						// get message to reply to
						// get chat message and reply to it
						sendMessage, errM := tgBot.sendMessage(botMessage, int(positionMessageId))
						if sendMessage != nil {

						}
						if errM != nil {
							return errM
						}
						break
					} else {
						time.Sleep(150 * time.Millisecond)
						continue
					}
				} else {
					// if last retry send error message
					if j == 2 {
						botErrorMessage := fmt.Sprintf("❌ Failed moving SL to entry price")
						botErrorMessage = fmt.Sprintf("%s\n%s", botErrorMessage,
							fmt.Sprintf("❌ Error: %s", tradeErr.Description))
						tgBot.sendMessage(botMessage, int(positionMessageId))
					}
					time.Sleep(150 * time.Millisecond)
					continue
				}
			}
		}
	}
	return nil
}

// function to do breakeven when tp are hit on a trade
func (tgBot *TgBot) doBreakeven(currentMessagePositions []MetaApiPosition, tpHitNumber int) error {
	// get entry price base on positions
	entryPrice := 0.0

	tradeSuccess := false
	// generate a telegram response for the bot
	botMessage := fmt.Sprintf("✅ TP%d hit 🎉", tpHitNumber)
	for _, position := range currentMessagePositions {
		//
		// safe
		entryPrice = position.OpenPrice
		// add a litle margin
		newStopLoss := calculateNewStopLossPriceForBreakeven(position.OpenPrice, position.Type, position.Symbol)
		positionMessageId := tgBot.RedisClient.GetPositionMessageId(position.ID)
		_ = TrailingStopLoss{
			Distance: &DistanceTrailingStopLoss{
				Distance: 0.1,
				Units:    "RELATIVE_PRICE",
			},
		}
		metaApiRequest := MetaApiTradeRequest{
			ActionType: "POSITION_MODIFY",
			StopLoss:   &newStopLoss,
			PositionID: &position.ID,
			TakeProfit: &position.TakeProfit,
			//TrailingStopLoss: &trailingStopLoss,
		}
		curentTp := extractTPFromClientId(position.ClientID)
		// place all positions stop loss to their open price
		for j := 0; j < 3; j++ {
			trade, err := executeTrade(tgBot.AppConfig.MetaApiEndpoint, metaApiRequest, tgBot.AppConfig.MetaApiAccountID,
				tgBot.AppConfig.MetaApiToken)
			//
			if err != nil {
				log.Printf("Error placing trade: %v", err)
				if j == 2 {
					// send parsed error message from bot
					// generate error message with reason
					botErrorMessage := fmt.Sprintf("❌ Failed moving TP%d SL to entry price", curentTp)
					botErrorMessage = fmt.Sprintf("%s\n%s", botErrorMessage,
						fmt.Sprintf("❌ Error: %s", err))
					tgBot.sendMessage(botErrorMessage, int(positionMessageId))
				}
				continue
			}
			errorTrade := HandleTradeError(trade.NumericCode)
			if tradeErr, ok := errorTrade.(*TradeError); ok {
				if tradeErr.Type == Success {
					if !tradeSuccess {
						tradeSuccess = true
					}
					if tradeErr.Type == Success {
						log.Printf("Trade update placed successfully: %v", trade)
						// append message to inform user that we moved the stop loss to the entry price of this current tp
						botMessage = fmt.Sprintf("%s\n%s", botMessage,
							fmt.Sprintf("➡️Moving TP%d SL to entry price", curentTp))
						// append with values on next  line
						botMessage = fmt.Sprintf("%s\n%s", botMessage,
							fmt.Sprintf("➡️Position ID: %s\nSL: %.2f -> %.2f", position.ID, position.StopLoss, entryPrice))
						// get message to reply to
						// get chat message and reply to it
						sendMessage, errM := tgBot.sendMessage(botMessage, int(positionMessageId))
						if errM != nil {
							return errM
						}
						if sendMessage != nil {

						}
						break
					} else {
						time.Sleep(150 * time.Millisecond)
						continue
					}
				} else {
					// if last retry send error message
					if j == 2 {
						botErrorMessage := fmt.Sprintf("❌ Failed moving TP%d SL to entry price", curentTp)
						botErrorMessage = fmt.Sprintf("%s\n%s", botErrorMessage,
							fmt.Sprintf("❌ Error: %s", tradeErr.Description))
						messageT, err := tgBot.sendMessage(botErrorMessage, int(positionMessageId))
						if messageT != nil {

						}
						if err != nil {
							log.Printf("Error sending message: %v", err)
						}
						break
					}
					time.Sleep(150 * time.Millisecond)
					continue
				}
			} else {
				if j == 2 {
					botErrorMessage := fmt.Sprintf("❌ Failed moving TP%d SL to entry price", curentTp)
					botErrorMessage = fmt.Sprintf("%s\n%s", botErrorMessage,
						fmt.Sprintf("❌ Error: %s", "Invalid code returned"))
					tgBot.sendMessage(botErrorMessage, int(positionMessageId))
				}

			}
		}

	}
	return nil
}

// automatic breakeven triggered by cron job

func (tgBot *TgBot) doModifyStopLoss(request *TradeRequest, update *TradeUpdateRequest, positions []MetaApiPosition, id string, token string) error {
	// get entry price base on positions
	tradeSuccess := false
	// generate a telegram response for the bot
	botMessage := fmt.Sprintf("✅ Modify stop loss 🎉")
	for _, position := range positions {
		//
		if *update.Value == 0 {
			// safe
			update.Value = &position.OpenPrice
		}
		positionMessageId := tgBot.RedisClient.GetPositionMessageId(position.ID)
		metaApiRequest := MetaApiTradeRequest{
			ActionType: "POSITION_MODIFY",
			StopLoss:   update.Value,
			PositionID: &position.ID,
			TakeProfit: &position.TakeProfit,
		}
		// place all positions stop loss to their open price
		for j := 0; j < 3; j++ {
			trade, err := executeTrade(tgBot.AppConfig.MetaApiEndpoint, metaApiRequest, tgBot.AppConfig.MetaApiAccountID,
				tgBot.AppConfig.MetaApiToken)
			//
			if err != nil {
				log.Printf("Error placing trade: %v", err)
				if j == 2 {
					// send parsed error message from bot
					// generate error message with reason
					botErrorMessage := fmt.Sprintf("❌ Failed moving stop loss")
					botErrorMessage = fmt.Sprintf("%s\n%s", botErrorMessage,
						fmt.Sprintf("❌ Error: %s", err))
					tgBot.sendMessage(botMessage, int(positionMessageId))
				}
				continue
			}
			errorTrade := HandleTradeError(trade.NumericCode)
			if tradeErr, ok := errorTrade.(*TradeError); ok {
				if tradeErr.Type == Success {
					if !tradeSuccess {
						tradeSuccess = true
					}
					if tradeErr.Type == Success {
						log.Printf("Trade update placed successfully: %v", trade)
						// append message to inform user that we moved the stop loss to the entry price of this current tp
						botMessage = fmt.Sprintf("%s\n%s", botMessage,
							fmt.Sprintf("➡️Moving SL to %.2f", *update.Value))
						// append with values on next  line
						botMessage = fmt.Sprintf("%s\n%s", botMessage,
							fmt.Sprintf("➡️Position ID: %s\nSL: %.2f -> %.2f", position.ID, position.StopLoss, *update.Value))
						// get message to reply to
						// get chat message and reply to it
						sendMessage, errM := tgBot.sendMessage(botMessage, int(positionMessageId))
						if errM != nil {
							return errM
						}
						if sendMessage != nil {

						}
						break
					} else {
						time.Sleep(150 * time.Millisecond)
						continue
					}
				} else {
					// if last retry send error message
					if j == 2 {
						botErrorMessage := fmt.Sprintf("❌ Failed moving stop loss")
						botErrorMessage = fmt.Sprintf("%s\n%s", botErrorMessage,
							fmt.Sprintf("❌ Error: %s", tradeErr.Description))
						messageT, err := tgBot.sendMessage(botMessage, int(positionMessageId))
						if messageT != nil {

						}
						if err != nil {
							log.Printf("Error sending message: %v", err)
						}
						break
					}
					time.Sleep(150 * time.Millisecond)
					continue
				}
			}
		}
	}
	return nil
}

// close all positions and implement the same logic as breakeven
func (tgBot *TgBot) doCloseTrade(positions []MetaApiPosition) error {
	// get entry price base on positions
	entryPrice := 0.0
	tradeSuccess := false
	// generate a telegram response for the bot
	botMessage := fmt.Sprintf("✅ Close trade 🎉")
	for _, position := range positions {
		//
		if entryPrice == 0 {
			// safe
			entryPrice = position.OpenPrice
		}
		positionMessageId := tgBot.RedisClient.GetPositionMessageId(position.ID)
		metaApiRequest := MetaApiTradeRequest{
			ActionType: "POSITION_CLOSE_ID",
			PositionID: &position.ID,
		}
		// place all positions stop loss to their open price
		for j := 0; j < 3; j++ {
			trade, err := executeTrade(tgBot.AppConfig.MetaApiEndpoint, metaApiRequest, tgBot.AppConfig.MetaApiAccountID,
				tgBot.AppConfig.MetaApiToken)
			//
			if err != nil {
				log.Printf("Error placing trade: %v", err)
				if j == 2 {
					// send parsed error message from bot
					// generate error message with reason
					botErrorMessage := fmt.Sprintf("❌ Failed closing trade")
					botErrorMessage = fmt.Sprintf("%s\n%s", botErrorMessage,
						fmt.Sprintf("❌ Error: %s", err))
					tgBot.sendMessage(botMessage, int(positionMessageId))
				}
				continue
			}
			errorTrade := HandleTradeError(trade.NumericCode)
			if tradeErr, ok := errorTrade.(*TradeError); ok {
				if tradeErr.Type == Success {
					if !tradeSuccess {
						tradeSuccess = true
					}
					if tradeErr.Type == Success {
						log.Printf("Trade update placed successfully: %v", trade)
						// append message to inform user that we moved the stop loss to the entry price of this current tp
						botMessage = fmt.Sprintf("%s\n%s", botMessage,
							fmt.Sprintf("➡️Closed trade"))
						// append with values on next  line
						botMessage = fmt.Sprintf("%s\n%s", botMessage,
							fmt.Sprintf("➡️Position ID: %s\n", position.ID))
						// get message to reply to
						// get chat message and reply to it
						sendMessage, errM := tgBot.sendMessage(botMessage, int(positionMessageId))
						if errM != nil {
							return errM
						}
						if sendMessage != nil {

						}
						break
					} else {
						time.Sleep(150 * time.Millisecond)
						continue
					}
				} else {
					// if last retry send error message
					if j == 2 {
						botErrorMessage := fmt.Sprintf("❌ Failed closing trade")
						botErrorMessage = fmt.Sprintf("%s\n%s", botErrorMessage,
							fmt.Sprintf("❌ Error: %s", tradeErr.Description))
						messageT, err := tgBot.sendMessage(botMessage, int(positionMessageId))
						if messageT != nil {

						}
						if err != nil {
							log.Printf("Error sending message: %v", err)
						}
						break
					}
					time.Sleep(150 * time.Millisecond)
					continue
				}
			}
		}
	}
	return nil
}

func messageIsTradingSignal(m *tg.Message) bool {
	// check if message contains certains terms
	termsToSearch := []string{"buy", "sell", "entry", "exit", "long", "short", "close", "stop", "loss", "take", "profit", "tp", "sl",
		"stoploss", "vente", "achete", "achat", "touche", "zone", "entry", "vend", "ferm", "securise"}
	for _, term := range termsToSearch {
		// use same case search
		if m != nil && m.Message != "" && strings.Contains(strings.ToLower(m.Message), term) {
			return true
		}
	}
	return false
}
func GenerateInitials(input string) string {
	// Séparer les mots en fonction des espaces ou des caractères non-alphabétiques
	words := strings.FieldsFunc(input, func(c rune) bool {
		return !unicode.IsLetter(c)
	})

	// Construire les initiales
	var initials string
	for _, word := range words {
		if len(word) > 0 {
			initials += string(unicode.ToUpper(rune(word[0])))
		}
		// Limiter les initiales à 5 caractères maximum
		if len(initials) >= 5 {
			return initials[:5]
		}
	}

	return initials
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

func (tgBot *TgBot) validateTradeValue(r *TradeRequest, strategy string) error {
	// check if takeprofit1 is set

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
	if strategy == "TP1" {
		r.TakeProfit2 = 0
		r.TakeProfit3 = 0
	}
	if strategy == "TP2" {
		r.TakeProfit3 = 0
	}
	// error if  stop loss. inferior to 0
	if r.StopLoss <= 0 {
		return errors.New("stop loss must be superior to 0")
	}

	return nil

}

func (tgBot *TgBot) currentUserPositions(endpoint string, metaApiAccountId, metaApiToken string) ([]MetaApiPosition, error) {
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

// get position by messageID containing in the clientID field with. Example for clientId 6203_TP2_pending 6203 is the messageID
func getPositionsByMessageId(positions []MetaApiPosition, messageID int) []MetaApiPosition {
	// search for position wich clientID start with messageID
	var result []MetaApiPosition
	for _, position := range positions {
		if position.ClientID != "" {
			// regex to check client id contain message
			//	chanelInitials := GenerateInitials(channel.Title) + "@" + strconv.Itoa(int(channel.ID))
			//			clientId := fmt.Sprintf("%s_%s_%s", chanelInitials, strconv.Itoa(message.ID), "TP"+strconv.Itoa(i+1))
			// Example : channelTitle@ChannelID_MessageID_TP2
			// Example = TPA@564464_46456_TP2
			re := regexp.MustCompile(`^[^@]+@\d+_` + strconv.Itoa(messageID) + `_TP\d+`)
			if re.MatchString(position.ClientID) {
				result = append(result, position)
			}
		}
	}
	return result
}

func getPositionByMessageIdAndTP(positions []MetaApiPosition, messageID int, tpNumber int) *MetaApiPosition {
	// search for position wich clientID start with messageID
	for _, position := range positions {
		if position.ClientID != "" {
			// regex to check if clientId contain message id and tp number
			re := regexp.MustCompile(`^[^@]+@\d+_` + strconv.Itoa(messageID) + `_TP` + strconv.Itoa(tpNumber))
			if re.MatchString(position.ClientID) {
				return &position
			}
		}
	}
	return nil
}

// extract tp from client id
func extractTPFromClientId(clientID string) int {
	re := regexp.MustCompile(`TP(\d+)`)
	matches := re.FindStringSubmatch(clientID)
	if len(matches) < 2 {
		return 0
	}
	tp, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}
	return tp
}

type MetaApiPosition struct {
	ID                          string  `json:"id"`
	Platform                    string  `json:"platform,omitempty"`
	Type                        string  `json:"type,omitempty"`
	Symbol                      string  `json:"symbol,omitempty"`
	Magic                       int     `json:"magic,omitempty"`
	Time                        string  `json:"time,omitempty"`
	DoneTime                    *string `json:"doneTime,omitempty"`
	BrokerTime                  string  `json:"brokerTime,omitempty"`
	DoneBrokerTime              *string `json:"doneBrokerTime,omitempty"`
	UpdateTime                  string  `json:"updateTime,omitempty"`
	OpenPrice                   float64 `json:"openPrice,omitempty"`
	EntryType                   string  `json:"entryType,omitempty"`
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
	State                       string  `json:"state,omitempty"`
	OriginalComment             string  `json:"originalComment,omitempty"`
	Price                       float64 `json:"price,omitempty"`
}

// lite position version for IA
type MetaApiPositionLite struct {
	ID         string  `json:"id"`
	OpenPrice  float64 `json:"openPrice,omitempty"`
	Price      float64 `json:"price,omitempty"`
	StopLoss   float64 `json:"stopLoss,omitempty"`
	TakeProfit float64 `json:"takeProfit,omitempty"`
	Profit     float64 `json:"profit,omitempty"`
	ChannelId  int     `json:"channelId,omitempty"`
}

// convert metaApiPosition to MetaApiPositionLite
func (p *MetaApiPosition) ToMetaApiPositionLite() MetaApiPositionLite {
	out := MetaApiPositionLite{
		ID:         p.ID,
		OpenPrice:  p.OpenPrice,
		StopLoss:   p.StopLoss,
		TakeProfit: p.TakeProfit,
		Profit:     p.Profit,
	}
	if p.ClientID != "" {
		c_id := extractChannelIDFromClientId(p.ClientID)
		out.ChannelId = c_id
	}
	if out.ChannelId == 0 {
		if p.BrokerComment != "" {
			c_id := extractChannelIDFromClientId(p.BrokerComment)
			out.ChannelId = c_id
		}
	}
	return out
}

func (p *MetaApiPosition) isBreakevenSetted() bool {
	if p.Type == "POSITION_TYPE_BUY" {
		return p.StopLoss >= p.OpenPrice
	}
	if p.Type == "POSITION_TYPE_SELL" {
		return p.StopLoss <= p.OpenPrice
	}
	return false
}

func (p *MetaApiPosition) isWin() bool {
	if !p.isBreakeven() {
		return p.Profit > 0
	}
	return false
}

func (p *MetaApiPosition) outcomeMessage(profit float64) string {
	// if dont time is set
	// if broker comment contains [tp]=
	if p.isBreakeven() {
		return ""
	}
	if p.Profit == 0 {
		return ""
	}
	// parse time to format 2006-01-02 15:04:05
	if p.Profit > 0 {
		// message format: TP1 hit (1.12 eur)
		tp := extractTPFromClientId(p.ClientID)
		return fmt.Sprintf("TP%d ✅ (%.2f %s)", tp, profit, "EUR")
	}
	// if broker comment contains [sl]
	if p.Profit < 0 {
		// message format: SL hit (1.1234 eur)
		return fmt.Sprintf("SL ❌ (%.2f %s)", profit, "EUR")
	}
	return fmt.Sprintf("CLOSED (%.2f %s)", profit, "EUR")
}

// convert list of MetaApiPosition to MetaApiPositionLite
func convertMetaApiPositionToMetaApiPositionLite(positions []MetaApiPosition) []MetaApiPositionLite {
	var result []MetaApiPositionLite
	for _, p := range positions {
		result = append(result, p.ToMetaApiPositionLite())
	}
	return result
}

type MetaApiTradeRequest struct {
	ActionType          string            `json:"actionType,omitempty,omitempty"`
	Symbol              string            `json:"symbol,omitempty"`
	Volume              *float64          `json:"volume,omitempty"`
	OpenPrice           *float64          `json:"openPrice,omitempty"`
	StopLimitPrice      *float64          `json:"stopLimitPrice,omitempty"`
	StopLoss            *float64          `json:"stopLoss,omitempty"`
	TakeProfit          *float64          `json:"takeProfit,omitempty"`
	StopLossUnits       *string           `json:"stopLossUnits,omitempty"`
	TakeProfitUnits     *string           `json:"takeProfitUnits,omitempty"`
	StopPriceBase       *string           `json:"stopPriceBase,omitempty"`
	OpenPriceUnits      *string           `json:"openPriceUnits,omitempty"`
	OpenPriceBase       *string           `json:"openPriceBase,omitempty"`
	StopLimitPriceUnits *string           `json:"stopLimitPriceUnits,omitempty"`
	StopLimitPriceBase  *string           `json:"stopLimitPriceBase,omitempty"`
	TrailingStopLoss    *TrailingStopLoss `json:"trailingStopLoss,omitempty"`
	OrderID             *string           `json:"orderId,omitempty"`
	PositionID          *string           `json:"positionId,omitempty"`
	CloseByPositionID   *string           `json:"closeByPositionId,omitempty"`
	Comment             *string           `json:"comment,omitempty"`
	ClientID            *string           `json:"clientId,omitempty"`
	Magic               *float64          `json:"magic,omitempty"`
	Slippage            *float64          `json:"slippage,omitempty"`
	FillingModes        *[]string         `json:"fillingModes,omitempty"`
	Expiration          *Expiration       `json:"expiration,omitempty"`
}

type DistanceTrailingStopLoss struct {
	Distance float64 `json:"distance"`        // La distance relative à appliquer pour le SL
	Units    string  `json:"units,omitempty"` // Unités (RELATIVE_PRICE, RELATIVE_POINTS, etc.)
}

type StopLossThreshold struct {
	Threshold float64 `json:"threshold"` // Seuil de prix par rapport au prix d'ouverture
	StopLoss  float64 `json:"stopLoss"`  // Valeur du stop loss
}

type ThresholdTrailingStopLoss struct {
	Thresholds    []StopLossThreshold `json:"thresholds"`              // Liste des seuils
	Units         string              `json:"units,omitempty"`         // Unités (ABSOLUTE_PRICE, RELATIVE_PRICE, etc.)
	StopPriceBase string              `json:"stopPriceBase,omitempty"` // Base du prix pour calculer le SL (CURRENT_PRICE, OPEN_PRICE)
}

type TrailingStopLoss struct {
	Distance *DistanceTrailingStopLoss `json:"distance,omitempty"` // Configuration TSL en fonction de la distance
}

type Expiration struct {
	Type string `json:"type,omitempty"`
	Time string `json:"time,omitempty"`
}

func ConvertToMetaApiTradeRequests(trade TradeRequest, strategy string) []MetaApiTradeRequest {
	var metaApiRequests []MetaApiTradeRequest

	tpValues := []float64{trade.TakeProfit1, trade.TakeProfit2, trade.TakeProfit3}

	for _, tp := range tpValues {

		//comment := fmt.Sprintf("Trade for TP%d", i+1)
		if tp == -1 || tp > 0 {
			metaTrade := MetaApiTradeRequest{
				ActionType: trade.ActionType,
				Symbol:     trade.Symbol,
				Volume:     &trade.Volume, // Assuming volume is the same for all trades
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

			if tp > 0 {
				metaTrade.TakeProfit = &tp
			}
			if trade.StopLoss > 0 {
				metaTrade.StopLoss = &trade.StopLoss
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

/**
[Unit]
Description=tradingbot

[Service]
Type=simple
Restart=always
RestartSec=5s
ExecStart=/home/ubuntu/telegramtradingbot/main


[Install]
WantedBy=multi-user.target
~
*/

func calculateNewStopLossPriceForBreakeven(entryPrice float64, actionType string, symbol string) float64 {
	pointSize := getCurrencyPointSize(symbol)
	margin := pointSize * 2
	pips := 0.0
	// calculate new stop loss
	newStopLoss := 0.0

	if actionType == "POSITION_TYPE_BUY" || actionType == "DEAL_TYPE_BUY" {
		newStopLoss = entryPrice + (pips + margin)
	} else if actionType == "POSITION_TYPE_SELL" || actionType == "DEAL_TYPE_SELL" {
		newStopLoss = entryPrice - (pips + margin)
	}
	return newStopLoss
}

// get default stop loss
func getDefaultStopLoss(actionType string, entryPrice float64) float64 {
	margin := 0.05
	pips := 0.0
	// calculate new stop loss
	stopLoss := 0.0
	if actionType == "POSITION_TYPE_BUY" {
		stopLoss = entryPrice - (pips + margin)
	} else if actionType == "POSITION_TYPE_SELL" {
		stopLoss = entryPrice + (pips + margin)
	}
	return stopLoss
}

func (tgBot *TgBot) checkCurrentPositions() {
	println("Checking current positions")
	startTime := time.Now()
	latestPositions, err := tgBot.currentUserPositions(tgBot.AppConfig.MetaApiEndpoint, tgBot.AppConfig.MetaApiAccountID, tgBot.AppConfig.MetaApiToken)
	if err != nil {
		println("Error getting current user positions: ", err)
	}
	// check for tp2 without tp1 and breakeven not setted
	for _, position := range latestPositions {
		// is position is winning
		if position.Profit > 0 {
			tpNumber := extractTPFromClientId(position.ClientID)
			if tpNumber > 1 && isBreakevenSetted(&position) == false {
				clientId := position.ClientID
				// check if tp1 is containing
				messageId := extractMessageIdFromClientId(clientId)
				tp1Position := getPositionByMessageIdAndTP(latestPositions, messageId, 1)
				channelID := extractChannelIDFromClientId(clientId)
				if tp1Position == nil {
					// trigger tp1_hit and breakeven
					messagePositions := getPositionsByMessageId(latestPositions, messageId)
					// trigger breakeven
					// send message
					if len(messagePositions) > 0 {
						positionMessageId := tgBot.RedisClient.GetPositionMessageId(position.ID)
						// check if breakeven is setted for the channel on redis
						if tgBot.RedisClient.IsBreakevenEnabled(channelID) {
							tgBot.sendMessage("Auto breakeven triggered", int(positionMessageId))
							tgBot.doBreakeven(messagePositions, 1)
						}
					}
				}
			}
		}

	}

	// if profit goal is not reached
	// check if the bot reached the objective amount profit
	// get amount from redis

	defaultProfitGoal := tgBot.RedisClient.GetDailyProfitGoal()
	profitGoal := defaultProfitGoal
	if profitGoal > 0 {
		// add margin of 10% to the profit goal
		profitGoal = profitGoal * 1.11
		onGoingProfit := calculateProfit(latestPositions, false)
		// get today positiions from metaapi
		todayPositions, errP := tgBot.getTodayPositions()
		if errP != nil {
			println("Error getting today positions: ", errP)
		}
		// calculate currentDayProfit
		currentDayProfit := calculateProfit(todayPositions, true)
		if currentDayProfit >= profitGoal || currentDayProfit+onGoingProfit >= profitGoal {
			// if current positions are making currentDayProfit or close to the currentDayProfit goal close all positions
			if onGoingProfit > 0 {
				closeAllPositions := true
				// on going loss
				totalLossOngoing := tgBot.getOngoingLossRiskTotal(latestPositions)
				if totalLossOngoing >= 0 {
					// if loss can be covered wont hurt daily profit goal
					if currentDayProfit-totalLossOngoing >= defaultProfitGoal {
						closeAllPositions = false
					}
				}
				if closeAllPositions {
					tgBot.doCloseTrade(latestPositions)
					// confirm by get current positions
					latestPositions, err := tgBot.currentUserPositions(tgBot.AppConfig.MetaApiEndpoint, tgBot.AppConfig.MetaApiAccountID, tgBot.AppConfig.MetaApiToken)
					if err != nil {
						println("Error getting current user positions: ", err)
					}
					if len(latestPositions) == 0 {
						// send message to user
						tgBot.sendMessage("Profit goal of "+strconv.FormatFloat(profitGoal, 'f', 2, 64)+" reached. Bot is now sleepig until tomorrow", 0)
					}
				}

			}

		}
	}

	// if tp2 and tp3 position is making profit up to 60% take half profit to secure
	// get all tp2 tp3 positions
	for _, position := range latestPositions {
		// FATAL MA NDEY FOFOU
		tpNumber := extractTPFromClientId(position.ClientID)
		// check if position is not already secured
		if tgBot.RedisClient.IsSecuredPosition(position.ID) {
			continue
		}
		if tpNumber > 1 {
			positionMessageId := tgBot.RedisClient.GetPositionMessageId(position.ID)
			// check if position is making profit
			if position.Profit > 0 && position.isBreakevenSetted() {
				// check if position has take profit and we can split the volume
				if position.Volume > 0.02 {
					// if case no take profit setted use a max take profit
					if position.TakeProfit == 0 {
						// add 500 pips to the current price
						pointSize := getCurrencyPointSize(position.Symbol)
						position.TakeProfit = position.CurrentPrice + (500 * pointSize)
					}
					// ensure that we've not already partially closed the position
					// if already partially closed brokerComment with be like "to #positionId"
					// distance to take profit
					distance := position.TakeProfit - position.OpenPrice
					// check if current price is close to take profit by 60%
					if position.CurrentPrice >= position.OpenPrice+distance*0.55 {
						// close half profit
						err := tgBot.doCloseHalfProfitTrade(position)
						if err != nil {

							//log
							println("Error closing half profit trade: ", err)
							// send message to user

							tgBot.sendMessage("[AUTO] Error closing half profit trade : "+err.Error(), int(positionMessageId))
							break
						} else {
							// send message to user
							tgBot.sendMessage("[AUTO] Half profit trade closed", int(positionMessageId))
							// save to secured position
							tgBot.RedisClient.SaveSecuredPosition(position.ID)
						}
					}
				}
			}
		}
	}

	// longer positions that last more than 12 hours should be closed if they are making profit
	for _, position := range latestPositions {
		// check if position is older than 12 hours
		timeString := position.Time
		timePos, err := time.Parse(time.RFC3339, timeString)
		if err != nil {
			println("Error parsing time: ", err)
		}
		positionMessageId := tgBot.RedisClient.GetPositionMessageId(position.ID)

		// check if position is older than 12 hours
		if timePos.Before(time.Now().Add(-12 * time.Hour)) {
			// check if position is making profit
			if position.Profit > 0 {
				// send message
				tgBot.doSlToEntryPrice([]MetaApiPosition{position})
				// send message
				tgBot.sendMessage("Position closed after 12 hours", int(positionMessageId))
			}

		}
	}

	tgBot.updateDailyInfo()
	// log end and duration
	endTime := time.Now()
	duration := endTime.Sub(startTime)
	//duration in seconds line 2s or 0.2 s
	seconds := duration.Seconds()
	println("Checking current positions done in ", seconds)
}

// function to get today profit
func (tgBot *TgBot) getTodayProfit() float64 {
	// get today positiions from metaapi
	todayPositions, errP := tgBot.getTodayPositions()
	if errP != nil {
		println("Error getting today positions: ", errP)
	}
	// calculate profit
	profit := calculateProfit(todayPositions, false)
	return profit
}

// reached profit goal
func (tgBot *TgBot) reachedProfitGoal() bool {
	profitGoal := tgBot.RedisClient.GetDailyProfitGoal()
	if profitGoal > 0 {
		profit := tgBot.getTodayProfit()
		if profit >= profitGoal {
			return true
		}
	}
	return false
}

// bot reached daily loss limit
func (tgBot *TgBot) reachedDailyLossLimit() bool {
	balance := tgBot.getAccountBalance()
	// limit percentage
	lossLimitPercentage := tgBot.RedisClient.GetDailyLossLimitPercentage()
	// daily profit
	profit := tgBot.getTodayProfit()
	// profit is negative on loss
	if profit < 0 {
		// calculate loss percentage
		lossPercentage := (profit / balance) * 100
		// use absolute value
		lossPercentage = math.Abs(lossPercentage)
		if lossPercentage >= lossLimitPercentage {
			return true
		}
	}
	return false
}

// get  daily loss limit amount
func (tgBot *TgBot) getDailyLossLimitAmount() float64 {
	balance := tgBot.getAccountBalance()
	// limit percentage
	lossLimitPercentage := tgBot.RedisClient.GetDailyLossLimitPercentage()
	// calculate loss amount
	lossAmount := balance * lossLimitPercentage / 100
	return lossAmount
}

// can support loss amount. check balance, loss limit and daily loss, profit goal
func (tgBot *TgBot) canSupportLossAmount(newLossAmount float64) bool {
	// daily profit
	balance := tgBot.getAccountBalance()
	if balance == 0 {
		return false
	}
	profit := tgBot.getTodayProfit()
	if profit < 0 {
		// get balance
		// limit percentage
		lossLimitPercentage := tgBot.RedisClient.GetDailyLossLimitPercentage()
		// calculate loss percentage
		lossPercentage := (profit - newLossAmount) / balance * 100
		if lossPercentage <= lossLimitPercentage {
			return true
		}
	}
	dailyProfitGoal := tgBot.RedisClient.GetDailyProfitGoal()
	if dailyProfitGoal > 0 {
		profitMinusNewLoss := profit - newLossAmount
		if profitMinusNewLoss >= dailyProfitGoal {
			return true
		}
	}
	return false
}

func extractChannelIDFromClientId(id string) int {
	re := regexp.MustCompile(`^[^@]+@(\d+)_`)
	matches := re.FindStringSubmatch(id)
	if len(matches) < 2 {
		return 0
	}
	channelID, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}
	return channelID
}

func getWinnigPositions(positions []MetaApiPosition) []MetaApiPosition {
	var result []MetaApiPosition
	for _, position := range positions {
		if position.Profit > 0 {
			result = append(result, position)
		}
	}
	return result
}

func getLoosingPositions(positions []MetaApiPosition) []MetaApiPosition {
	var result []MetaApiPosition
	for _, position := range positions {
		if position.Profit < 0 {
			result = append(result, position)
		}
	}
	return result
}

// curl -X POST --header 'Content-Type: application/json' --header 'Accept: application/json' --header 'auth-token: token' -d '{
// "actionType": "POSITION_PARTIAL",
// "positionId":"46648037",
// "volume": 0.01
// }' 'https://mt-client-api-v1.new-york.agiliumtrade.ai/users/current/accounts/865d3a4d-3803-486d-bdf3-a85679d9fad2/trade'
func (tgBot *TgBot) doCloseHalfProfitTrade(position MetaApiPosition) error {
	// get entry price base on positions
	tradeSuccess := false
	// if volume is lower than 0.02 skip
	if position.Volume < 0.02 {
		return errors.New("Volume is lower than 0.02. Can't close half profit")
	}
	botMessage := fmt.Sprintf("✅ Close half profit trade 🎉")
	halfVolume := position.Volume / 2
	// limit to 2 decimal
	halfVolume = math.Round(halfVolume*100) / 100

	//
	positionMessageId := tgBot.RedisClient.GetPositionMessageId(position.ID)
	metaApiRequest := MetaApiTradeRequest{
		ActionType: "POSITION_PARTIAL",
		PositionID: &position.ID,
		Volume:     &halfVolume,
	}
	// place all positions stop loss to their open price
	for j := 0; j < 3; j++ {
		trade, err := executeTrade(tgBot.AppConfig.MetaApiEndpoint, metaApiRequest, tgBot.AppConfig.MetaApiAccountID,
			tgBot.AppConfig.MetaApiToken)
		//
		if err != nil {
			log.Printf("Error placing trade: %v", err)
			if j == 2 {
				// send parsed error message from bot
				// generate error message with reason
				botErrorMessage := fmt.Sprintf("❌ Failed closing half profit trade")
				botErrorMessage = fmt.Sprintf("%s\n%s", botErrorMessage,
					fmt.Sprintf("❌ Error: %s", err))
				tgBot.sendMessage(botMessage, int(positionMessageId))
			}
			continue
		}
		errorTrade := HandleTradeError(trade.NumericCode)
		if tradeErr, ok := errorTrade.(*TradeError); ok {
			if tradeErr.Type == Success {
				if !tradeSuccess {
					tradeSuccess = true
				}
				log.Printf("Trade update placed successfully: %v", trade)
				// append message to inform user that we moved the stop loss to the entry price of this current tp
				botMessage = fmt.Sprintf("%s\n%s", botMessage,
					fmt.Sprintf("➡️Closed half profit trade"))
				// append with values on next  line
				botMessage = fmt.Sprintf("%s\n%s", botMessage,
					fmt.Sprintf("➡️Position ID: %s\n", position.ID))
				// get message to reply to // get chat message and reply to it
				sendMessage, errM := tgBot.sendMessage(botMessage, int(positionMessageId))
				if errM != nil {
					return errM
				}
				if sendMessage != nil {

				}
				break
			} else {
				// if last retry send error message
				if j == 2 {
					botErrorMessage := fmt.Sprintf("❌ Failed closing trade")
					botErrorMessage = fmt.Sprintf("%s\n%s", botErrorMessage,
						fmt.Sprintf("❌ Error: %s", tradeErr.Description))
					messageT, err := tgBot.sendMessage(botMessage, int(positionMessageId))
					if messageT != nil {

					}
					if err != nil {
						log.Printf("Error sending message: %v", err)
					}
					break
				}
				time.Sleep(150 * time.Millisecond)
				continue
			}
		}
	}
	return nil
}

func calculateProfit(positions []MetaApiPosition, filledOnly bool) float64 {
	// check if position state is ORDER_STATE_FILLED and addition the takeProfit field
	profit := 0.0
	for _, position := range positions {
		profit += position.Profit
	}
	return profit
}

// curl --location 'https://mt-client-api-v1.london.agiliumtrade.ai/users/current/accounts/174013a4-0a6e-4f10-bda7-feee1a846b4a/history-orders/time/2024-10-27T00:00:16Z/2024-10-28T00:00:16Z' \
// --header 'auth-token: eyJhbGciOiJSUzUxMiIsInR5cCI6IkpXVCJ9.eyJfaWQiOiJhOWI3Y2M0ZDkzYzA2NDNmYjc1NDYyNjI1YmI3YjBmMiIsInBlcm1pc3Npb25zIjpbXSwiYWNjZXNzUnVsZXMiOlt7ImlkIjoidHJhZGluZy1hY2NvdW50LW1hbmFnZW1lbnQtYXBpIiwibWV0aG9kcyI6WyJ0cmFkaW5nLWFjY291bnQtbWFuYWdlbWVudC1hcGk6cmVzdDpwdWJsaWM6KjoqIl0sInJvbGVzIjpbInJlYWRlciIsIndyaXRlciJdLCJyZXNvdXJjZXMiOlsiKjokVVNFUl9JRCQ6KiJdfSx7ImlkIjoibWV0YWFwaS1yZXN0LWFwaSIsIm1ldGhvZHMiOlsibWV0YWFwaS1hcGk6cmVzdDpwdWJsaWM6KjoqIl0sInJvbGVzIjpbInJlYWRlciIsIndyaXRlciJdLCJyZXNvdXJjZXMiOlsiKjokVVNFUl9JRCQ6KiJdfSx7ImlkIjoibWV0YWFwaS1ycGMtYXBpIiwibWV0aG9kcyI6WyJtZXRhYXBpLWFwaTp3czpwdWJsaWM6KjoqIl0sInJvbGVzIjpbInJlYWRlciIsIndyaXRlciJdLCJyZXNvdXJjZXMiOlsiKjokVVNFUl9JRCQ6KiJdfSx7ImlkIjoibWV0YWFwaS1yZWFsLXRpbWUtc3RyZWFtaW5nLWFwaSIsIm1ldGhvZHMiOlsibWV0YWFwaS1hcGk6d3M6cHVibGljOio6KiJdLCJyb2xlcyI6WyJyZWFkZXIiLCJ3cml0ZXIiXSwicmVzb3VyY2VzIjpbIio6JFVTRVJfSUQkOioiXX0seyJpZCI6Im1ldGFzdGF0cy1hcGkiLCJtZXRob2RzIjpbIm1ldGFzdGF0cy1hcGk6cmVzdDpwdWJsaWM6KjoqIl0sInJvbGVzIjpbInJlYWRlciIsIndyaXRlciJdLCJyZXNvdXJjZXMiOlsiKjokVVNFUl9JRCQ6KiJdfSx7ImlkIjoicmlzay1tYW5hZ2VtZW50LWFwaSIsIm1ldGhvZHMiOlsicmlzay1tYW5hZ2VtZW50LWFwaTpyZXN0OnB1YmxpYzoqOioiXSwicm9sZXMiOlsicmVhZGVyIiwid3JpdGVyIl0sInJlc291cmNlcyI6WyIqOiRVU0VSX0lEJDoqIl19LHsiaWQiOiJjb3B5ZmFjdG9yeS1hcGkiLCJtZXRob2RzIjpbImNvcHlmYWN0b3J5LWFwaTpyZXN0OnB1YmxpYzoqOioiXSwicm9sZXMiOlsicmVhZGVyIiwid3JpdGVyIl0sInJlc291cmNlcyI6WyIqOiRVU0VSX0lEJDoqIl19LHsiaWQiOiJtdC1tYW5hZ2VyLWFwaSIsIm1ldGhvZHMiOlsibXQtbWFuYWdlci1hcGk6cmVzdDpkZWFsaW5nOio6KiIsIm10LW1hbmFnZXItYXBpOnJlc3Q6cHVibGljOio6KiJdLCJyb2xlcyI6WyJyZWFkZXIiLCJ3cml0ZXIiXSwicmVzb3VyY2VzIjpbIio6JFVTRVJfSUQkOioiXX0seyJpZCI6ImJpbGxpbmctYXBpIiwibWV0aG9kcyI6WyJiaWxsaW5nLWFwaTpyZXN0OnB1YmxpYzoqOioiXSwicm9sZXMiOlsicmVhZGVyIl0sInJlc291cmNlcyI6WyIqOiRVU0VSX0lEJDoqIl19XSwidG9rZW5JZCI6IjIwMjEwMjEzIiwiaW1wZXJzb25hdGVkIjpmYWxzZSwicmVhbFVzZXJJZCI6ImE5YjdjYzRkOTNjMDY0M2ZiNzU0NjI2MjViYjdiMGYyIiwiaWF0IjoxNzI5OTgyOTIxLCJleHAiOjE3Mzc3NTg5MjF9.Io-bIjaBeEZyYAcNpta8QlGM3HeOtS8v4x3rJoSTuzP4aqqLmazYMhsvVVWXAOa-7BW0Idp5HwVkxp2oACPj8vcdSoRY0OsmopAcSP6tWGK-S3oCPS2ig3EaYR2WNiaPkg5rnye_lafBNKEZnQ9OrETYJOl_7hT-Rbw_PIOJMhASzdHsiBKsPJ7_h-qI_xTFVLtBfMgwBsnoAL_RCc4SwbyShw7kIwqv1t2aKrZOOzfDrTlYLCeDXOou62nezHxAHmSmPoiUgcSOq1Q34n71xq12H2Jk9AecyFJrP-Ayycx1a7fy0Nb6kanvMHV-SYQwkLDcC-J1h8QN5uqnSXSByI6K9JheopN-bG14VXFRu6DYOr-MCHDiv1tRkPzc_P9SJQKktkN_Tvrodp7pEVdr4Bczghd6dcZ0OoauwF_dlreHx12hHTEoH-pnfglW5zdtirp0BlUpBAg6OHMWzc1aVz8HKhgANQfACJys62FqUgpwe0d_sZ2rRJI6aA8mp5bkC3uMlN-dtCI82WhOvoXyBqCGp2lPxwxiXClR7m2DDq5OzVIeDolW94G0HJV-CADAQlu4xkKwpGVDovFytzXwuvRxoMxrAu1W1SxIDEOXa5t-vFUrV0coaaiPeRzTRaYYAyKul143TYDCKK62ve1G7kwXOZWjuk3pcUkrO2Uvl1o' \
// --header 'Accept: application/json'
// day from midnight to midnight
func (tgBot *TgBot) getTodayPositions() ([]MetaApiPosition, error) {
	startDay := time.Now().Format("2006-01-02T00:00:00Z")
	//
	endDay := time.Now().Add(24 * time.Hour).Format("2006-01-02T00:00:00Z")
	url := fmt.Sprintf("%s/users/current/accounts/%s/history-deals/time/%s/%s", tgBot.AppConfig.MetaApiEndpoint,
		tgBot.AppConfig.MetaApiAccountID, startDay, endDay)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("auth-token", tgBot.AppConfig.MetaApiToken)
	req.Header.Add("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch today positions")
	}

	var positions []MetaApiPosition
	err = json.NewDecoder(resp.Body).Decode(&positions)
	if err != nil {
		return nil, err
	}
	// exclud positions of type DEAL_TYPE_BALANCE
	var filteredPositions []MetaApiPosition
	for _, position := range positions {
		if position.Type != "DEAL_TYPE_BALANCE" {
			filteredPositions = append(filteredPositions, position)
		}
	}
	// enchace value with order
	positionOrders, errO := tgBot.getTodayOrders()
	if errO != nil {

	}
	// merge positions and orders
	for _, order := range positionOrders {
		for i, position := range filteredPositions {
			if position.ID == order.ID {
				filteredPositions[i].OpenPrice = order.OpenPrice
				filteredPositions[i].CurrentPrice = order.CurrentPrice
			}
		}
	}
	return filteredPositions, nil
}

func (tgBot *TgBot) getMonthPositions() ([]MetaApiPosition, error) {
	now := time.Now()
	// Set startDay to the first day of the current month at midnight
	startDay := now.AddDate(0, 0, -16).Format("2006-01-02T00:00:00Z")

	// Set endDay to tomorrow at midnight
	endDay := now.Add(24 * time.Hour).Truncate(24 * time.Hour).Format("2006-01-02T00:00:00Z")

	url := fmt.Sprintf("%s/users/current/accounts/%s/history-deals/time/%s/%s", tgBot.AppConfig.MetaApiEndpoint,
		tgBot.AppConfig.MetaApiAccountID, startDay, endDay)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("auth-token", tgBot.AppConfig.MetaApiToken)
	req.Header.Add("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch today positions")
	}

	var positions []MetaApiPosition
	err = json.NewDecoder(resp.Body).Decode(&positions)
	if err != nil {
		return nil, err
	}
	// exclud positions of type DEAL_TYPE_BALANCE
	var filteredPositions []MetaApiPosition
	for _, position := range positions {
		if position.Type != "DEAL_TYPE_BALANCE" {
			filteredPositions = append(filteredPositions, position)
		}
	}
	// enchace value with order
	positionOrders, errO := tgBot.getMonthOrders()
	if errO != nil {

	}
	// merge positions and orders
	for _, order := range positionOrders {
		for i, position := range filteredPositions {
			if position.ID == order.ID {
				filteredPositions[i].OpenPrice = order.OpenPrice
				filteredPositions[i].CurrentPrice = order.CurrentPrice
			}
		}
	}
	return filteredPositions, nil
}

// todays position orders ! {{baseUrl}}/users/current/accounts/:accountId/history-orders/time/:startTime/:endTime
func (tgBot *TgBot) getTodayOrders() ([]MetaApiPosition, error) {
	startDay := time.Now().Format("2006-01-02T00:00:00Z")
	endDay := time.Now().Add(24 * time.Hour).Format("2006-01-02T00:00:00Z")
	url := fmt.Sprintf("%s/users/current/accounts/%s/history-orders/time/%s/%s", tgBot.AppConfig.MetaApiEndpoint,
		tgBot.AppConfig.MetaApiAccountID, startDay, endDay)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("auth-token", tgBot.AppConfig.MetaApiToken)
	req.Header.Add("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch today positions")
	}

	var positions []MetaApiPosition
	err = json.NewDecoder(resp.Body).Decode(&positions)
	if err != nil {
		return nil, err
	}
	// exclud positions of type DEAL_TYPE_BALANCE
	var filteredPositions []MetaApiPosition
	for _, position := range positions {
		if position.Type != "DEAL_TYPE_BALANCE" {
			filteredPositions = append(filteredPositions, position)
		}
	}
	return filteredPositions, nil
}

func (tgBot *TgBot) getMonthOrders() ([]MetaApiPosition, error) {
	now := time.Now()
	// Set startDay to the first day of the current month at midnight
	startDay := now.AddDate(0, 0, -16).Format("2006-01-02T00:00:00Z")

	// Set endDay to tomorrow at midnight
	endDay := now.Add(24 * time.Hour).Truncate(24 * time.Hour).Format("2006-01-02T00:00:00Z")
	url := fmt.Sprintf("%s/users/current/accounts/%s/history-orders/time/%s/%s", tgBot.AppConfig.MetaApiEndpoint,
		tgBot.AppConfig.MetaApiAccountID, startDay, endDay)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("auth-token", tgBot.AppConfig.MetaApiToken)
	req.Header.Add("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch today positions")
	}

	var positions []MetaApiPosition
	err = json.NewDecoder(resp.Body).Decode(&positions)
	if err != nil {
		return nil, err
	}
	// exclud positions of type DEAL_TYPE_BALANCE
	var filteredPositions []MetaApiPosition
	for _, position := range positions {
		if position.Type != "DEAL_TYPE_BALANCE" {
			filteredPositions = append(filteredPositions, position)
		}
	}
	return filteredPositions, nil
}

func extractMessageIdFromClientId(id string) int {
	// client id example : channelTitle@ChannelID_MessageID_TP2 :  TGR@2054755865_4609_TP3
	re := regexp.MustCompile(`[^@]+@\d+_(\d+)_TP\d+`)
	matches := re.FindStringSubmatch(id)
	if len(matches) < 2 {
		return 0
	}
	messageId, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}
	return messageId
}

// check if breakeven is setted for a position. it is set when stop is moved to entry price with a margin base on buy or sell
func isBreakevenSetted(position *MetaApiPosition) bool {
	if position.StopLoss == 0 {
		return false
	}
	if position.Type == "POSITION_TYPE_BUY" {
		// stop loss superior to entry price
		if position.StopLoss >= position.OpenPrice {
			return true
		}
		//
		distance := position.OpenPrice - position.StopLoss
		distance = math.Abs(distance)
		symbole := position.Symbol
		pointSize := getCurrencyPointSize(symbole)
		margin := pointSize * 2
		distancePoint := distance / pointSize
		if distancePoint <= margin {
			return true
		}
	} else if position.Type == "POSITION_TYPE_SELL" {
		// stop loss inferior to entry price
		if position.StopLoss <= position.OpenPrice {
			return true
		}
		//
		distance := position.StopLoss - position.OpenPrice
		distance = math.Abs(distance)
		symbole := position.Symbol
		pointSize := getCurrencyPointSize(symbole)
		margin := pointSize * 2
		distancePoint := distance / pointSize
		if distancePoint <= margin {
			return true
		}

	}
	return false
}

// is position breakeven
func (position *MetaApiPosition) isBreakeven() bool {
	if position.Profit == 0 {
		return true
	}
	if position.StopLoss == 0 {
		return false
	}
	actionType := "POSITION_TYPE_BUY"
	if position.EntryType == "DEAL_ENTRY_OUT" {
		if position.Type == "DEAL_TYPE_BUY" {
			actionType = "POSITION_TYPE_SELL"
		}
	}
	breakevenPrice := calculateNewStopLossPriceForBreakeven(position.OpenPrice, actionType, position.Symbol)
	distance := breakevenPrice - position.StopLoss
	//round
	distance = math.Round(distance*100) / 100
	distance = math.Abs(distance)
	pointSize := getCurrencyPointSize(position.Symbol)
	margin := pointSize * 2
	if distance <= margin {
		return true
	}
	return false
}

type MetaApiAccountInformation struct {
	Platform                    string  `json:"platform,omitempty"`
	Broker                      string  `json:"broker,omitempty"`
	Currency                    string  `json:"currency,omitempty"`
	Server                      string  `json:"server,omitempty"`
	Balance                     float64 `json:"balance,omitempty"`
	Equity                      float64 `json:"equity,omitempty"`
	Margin                      float64 `json:"margin,omitempty"`
	FreeMargin                  float64 `json:"freeMargin,omitempty"`
	Leverage                    float64 `json:"leverage,omitempty"`
	MarginLevel                 float64 `json:"marginLevel,omitempty"`
	TradeAllowed                bool    `json:"tradeAllowed,omitempty"`
	MarginMode                  string  `json:"marginMode,omitempty"`
	Name                        string  `json:"name,omitempty"`
	Login                       int     `json:"login,omitempty"`
	Credit                      float64 `json:"credit,omitempty"`
	Type                        string  `json:"type,omitempty"`
	AccountCurrencyExchangeRate float64 `json:"accountCurrencyExchangeRate,omitempty"`
}

func (tgBot *TgBot) getAccountInformation() (MetaApiAccountInformation, error) {
	url := fmt.Sprintf("%s/users/current/accounts/%s/account-information", tgBot.AppConfig.MetaApiEndpoint, tgBot.AppConfig.MetaApiAccountID)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("auth-token", tgBot.AppConfig.MetaApiToken)
	req.Header.Add("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return MetaApiAccountInformation{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return MetaApiAccountInformation{}, errors.New("failed to fetch account information")
	}

	var accountInformation MetaApiAccountInformation
	err = json.NewDecoder(resp.Body).Decode(&accountInformation)
	if err != nil {
		return MetaApiAccountInformation{}, err
	}

	return accountInformation, nil
}

// curl --location --request POST 'https://mt-client-api-v1.london.agiliumtrade.ai/users/current/accounts/<string>/deploy?executeForAllReplicas=true' \
// --header 'auth-token: <string>' \
// --header 'Accept: */*'
// deploy account
func (tgBot *TgBot) deployAccount() error {
	url := fmt.Sprintf("%s/users/current/accounts/%s/deploy?executeForAllReplicas=true", tgBot.AppConfig.MetaApiEndpoint, tgBot.AppConfig.MetaApiAccountID)

	req, _ := http.NewRequest("POST", url, nil)
	req.Header.Add("auth-token", tgBot.AppConfig.MetaApiToken)
	req.Header.Add("Accept", "*/*")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return errors.New("failed to deploy account")
	}

	return nil
}

// undeploy
// //curl --location --request POST 'https://mt-client-api-v1.london.agiliumtrade.ai/users/current/accounts/<string>/deploy?executeForAllReplicas=true' \
// //--header 'auth-token: <string>' \
// //--header 'Accept: */*'
func (tgBot *TgBot) undeployAccount() error {
	url := fmt.Sprintf("%s/users/current/accounts/%s/undeploy?executeForAllReplicas=true", tgBot.AppConfig.MetaApiEndpoint, tgBot.AppConfig.MetaApiAccountID)

	req, _ := http.NewRequest("POST", url, nil)
	req.Header.Add("auth-token", tgBot.AppConfig.MetaApiToken)
	req.Header.Add("Accept", "*/*")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return errors.New("failed to undeploy account")
	}

	return nil
}

type MetaApiAccount struct {
	ID               string `json:"_id"`
	State            string `json:"state"`
	ConnectionStatus string `json:"connectionStatus"`
}

// curl --location 'https://mt-client-api-v1.london.agiliumtrade.ai/users/current/accounts/<string>' \
// --header 'auth-token: <string>' \
// --header 'Accept: */*'
func (tgBot *TgBot) getAccount() (MetaApiAccount, error) {
	url := fmt.Sprintf("%s/users/current/accounts/%s", tgBot.AppConfig.MetaApiEndpoint, tgBot.AppConfig.MetaApiAccountID)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("auth-token", tgBot.AppConfig.MetaApiToken)
	req.Header.Add("Accept", "*/*")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return MetaApiAccount{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return MetaApiAccount{}, errors.New("failed to fetch account")
	}

	var account MetaApiAccount
	err = json.NewDecoder(resp.Body).Decode(&account)
	if err != nil {
		return MetaApiAccount{}, err
	}

	return account, nil
}

// get the total possible loss of the day
func (tgBot *TgBot) getOngoingLossRiskTotal(todayPositions []MetaApiPosition) float64 {
	// get today positiions from metaapi
	if todayPositions != nil && len(todayPositions) == 0 {
		pos, errP := tgBot.currentUserPositions(tgBot.AppConfig.MetaApiEndpoint, tgBot.AppConfig.MetaApiAccountID, tgBot.AppConfig.MetaApiToken)
		if errP != nil {
			println("Error getting today positions: ", errP)
		}
		if pos != nil {
			todayPositions = pos
		}
	}

	totalLoss := 0.0
	for _, position := range todayPositions {
		pipsLoss := calculatePips(position.OpenPrice, position.StopLoss, position.Symbol)
		// loss in money
		totalLoss += pipsLoss * position.Volume
	}

	return totalLoss
}
