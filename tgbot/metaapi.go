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

func (tgBot *TgBot) HandleTradeRequest(ctx context.Context, message *tg.Message, openaiApiKey, metaApiAccountId,
	metaApiToken string, volume float64, parentRequest *TradeRequest, channel tg.Channel) (*TradeRequest, *[]TradeResponse, error) {
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
		strategy := tgBot.RedisClient.GetStrategy()

		errTrade := validateTradeValue(tradeRequest, strategy)
		// check if the same trade already exist
		if tgBot.RedisClient.IsTradeKeyExist(tradeRequest.GenerateTradeRequestKey()) {
			log.Printf("Trade already placed")
			//niapeu xonam xawgama ray
			return nil, nil, errors.New("trade already exist")
		}

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
		metaApiRequests := ConvertToMetaApiTradeRequests(*tradeRequest, strategy)
		// trade response list
		var tradeResponses []TradeResponse
		tradeSuccess := false
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
			takeProfit := 0.0
			if i == 0 {
				takeProfit = tradeRequest.TakeProfit1
			} else if i == 1 {
				takeProfit = tradeRequest.TakeProfit2
			} else if i == 2 {
				takeProfit = tradeRequest.TakeProfit3
			}
			tpNumber := i + 1
			metaApiRequest.Volume = &volume
			// concat channel id and channel initial
			chanelInitials := GenerateInitials(channel.Title) + "@" + strconv.Itoa(int(channel.ID))
			clientId := fmt.Sprintf("%s_%s_%s", chanelInitials, strconv.Itoa(message.ID), "TP"+strconv.Itoa(i+1))
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
						 Buy EURUSD
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
			tradeRequest.MessageId = &message.ID
			tradeRbytes, errJ := json.Marshal(tradeRequest)
			if errJ == nil {
				tgBot.RedisClient.SetTradeRequest(int64(message.ID), tradeRbytes)
				// add trade key
				tgBot.RedisClient.AddTradeKey(tradeRequest.GenerateTradeRequestKey())
				return tradeRequest, &tradeResponses, nil
			}
		} else {
			return nil, nil, errors.New("trade not placed")
		}
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
			// do breakeven
			//	if tp1 hit modify SL to tp 1 value on all other TP
			if parentRequest.TakeProfit1 == -1 {
				// manual close tp1
				positionTP1 := getPositionByMessageIdAndTP(currentMessagePositions, *parentRequest.MessageId, 1)
				if positionTP1 != nil && positionTP1.TakeProfit == 0 {
					customPositions := []*MetaApiPosition{positionTP1}
					_ = tgBot.doCloseTrade(parentRequest, customPositions, metaApiAccountId, metaApiToken)
					positions, err = currentUserPositions(tgBot.AppConfig.MetaApiEndpoint, metaApiAccountId, metaApiToken)
					if err != nil {
						return nil, nil, err
					}
					currentMessagePositions = getPositionsByMessageId(positions, *parentRequest.MessageId)
				}

			}
			tgBot.doBreakeven(parentRequest, &channel, currentMessagePositions, *parentRequest.MessageId, 1)
		} else if tradeUpdate.UpdateType == "TP2_HIT" {
			// do breakeven
			//	if tp1 hit modify SL to tp 1 value on all other TP
			if parentRequest.TakeProfit2 == -1 {
				// manual close tp1
				positionTP1 := getPositionByMessageIdAndTP(currentMessagePositions, *parentRequest.MessageId, 2)
				if positionTP1 != nil && positionTP1.TakeProfit == 0 {
					customPositions := []*MetaApiPosition{positionTP1}
					_ = tgBot.doCloseTrade(parentRequest, customPositions, metaApiAccountId, metaApiToken)
					positions, err = currentUserPositions(tgBot.AppConfig.MetaApiEndpoint, metaApiAccountId, metaApiToken)
					if err != nil {
						return nil, nil, err
					}
					currentMessagePositions = getPositionsByMessageId(positions, *parentRequest.MessageId)
				}

			}
			tgBot.doBreakeven(parentRequest, &channel, currentMessagePositions, *parentRequest.MessageId, 2)
		} else if tradeUpdate.UpdateType == "TP3_HIT" {
			// do breakeven
			//	if tp1 hit modify SL to tp 1 value on all other TP
			if parentRequest.TakeProfit3 == -1 {
				// manual close tp1
				positionTP1 := getPositionByMessageIdAndTP(currentMessagePositions, *parentRequest.MessageId, 3)
				if positionTP1 != nil && positionTP1.TakeProfit == 0 {
					customPositions := []*MetaApiPosition{positionTP1}
					_ = tgBot.doCloseTrade(parentRequest, customPositions, metaApiAccountId, metaApiToken)
					positions, err = currentUserPositions(tgBot.AppConfig.MetaApiEndpoint, metaApiAccountId, metaApiToken)
					if err != nil {
						return nil, nil, err
					}
					currentMessagePositions = getPositionsByMessageId(positions, *parentRequest.MessageId)
				}

			}
			tgBot.doBreakeven(parentRequest, &channel, currentMessagePositions, *parentRequest.MessageId, 3)

		} else if tradeUpdate.UpdateType == "CLOSE_TRADE" {
			// close all positions
			errClodeTrade := tgBot.doCloseTrade(parentRequest, currentMessagePositions, metaApiAccountId, metaApiToken)
			if errClodeTrade != nil {

			}
		} else if tradeUpdate.UpdateType == "MODIFY_STOPLOSS" {
			// modify stop loss to the value given
			errModifyStopLoss := tgBot.doModifyStopLoss(parentRequest, tradeUpdate, currentMessagePositions, metaApiAccountId, metaApiToken)
			if errModifyStopLoss != nil {

			}

		} else if tradeUpdate.UpdateType == "SL_TO_ENTRY_PRICE" {
			// modify stop loss to the value given
			errSlToEntryPrice := tgBot.doSlToEntryPrice(parentRequest, currentMessagePositions, metaApiAccountId, metaApiToken)
			if errSlToEntryPrice != nil {

			}

		}
		//get list active positions

		return nil, &tradeResponses, nil
	}
	return nil, nil, errors.New("trade not placed")
}

func (tgBot *TgBot) doSlToEntryPrice(request *TradeRequest, positions []*MetaApiPosition, id string, token string) error {
	// get entry price base on positions
	tradeSuccess := false
	// generate a telegram response for the bot
	botMessage := fmt.Sprintf("✅ SL to Entry Price 🎉")
	for _, position := range positions {
		//
		positionMessageId := tgBot.RedisClient.GetPositionMessageId(position.ID)
		metaApiRequest := MetaApiTradeRequest{
			ActionType: "POSITION_MODIFY",
			StopLoss:   &position.OpenPrice,
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
func (tgBot *TgBot) doBreakeven(tradeRequest *TradeRequest, channel *tg.Channel, currentMessagePositions []*MetaApiPosition, messageId int, tpHitNumber int) error {
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
		entryPrice = entryPrice + 0.04
		positionMessageId := tgBot.RedisClient.GetPositionMessageId(position.ID)
		metaApiRequest := MetaApiTradeRequest{
			ActionType: "POSITION_MODIFY",
			StopLoss:   &entryPrice,
			PositionID: &position.ID,
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

func (tgBot *TgBot) doModifyStopLoss(request *TradeRequest, update *TradeUpdateRequest, positions []*MetaApiPosition, id string, token string) error {
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
func (tgBot *TgBot) doCloseTrade(request *TradeRequest, positions []*MetaApiPosition, id string, token string) error {
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

func validateTradeValue(r *TradeRequest, strategy string) error {
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
	if r.TakeProfit1 == -1 {
		if strategy == "TP2" {
			if r.TakeProfit2 == 0 {
				r.TakeProfit2 = -1
			}
		}
		if strategy == "3TP" || strategy == "TP3" {
			if r.TakeProfit2 == 0 {
				r.TakeProfit2 = -1
			}
			if r.TakeProfit3 == 0 {
				r.TakeProfit3 = -1
			}
		}
	}
	return nil

}

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
func getPositionsByMessageId(positions []MetaApiPosition, messageID int) []*MetaApiPosition {
	// search for position wich clientID start with messageID
	var result []*MetaApiPosition
	for _, position := range positions {
		if position.ClientID != "" {
			// regex to check client id contain message
			//	chanelInitials := GenerateInitials(channel.Title) + "@" + strconv.Itoa(int(channel.ID))
			//			clientId := fmt.Sprintf("%s_%s_%s", chanelInitials, strconv.Itoa(message.ID), "TP"+strconv.Itoa(i+1))
			// Example : channelTitle@ChannelID_MessageID_TP2
			// Example = TPA@564464_46456_TP2
			re := regexp.MustCompile(`^[^@]+@\d+_` + strconv.Itoa(messageID) + `_TP\d+`)
			if re.MatchString(position.ClientID) {
				result = append(result, &position)
			}
		}
	}
	return result
}

func getPositionByMessageIdAndTP(positions []*MetaApiPosition, messageID int, tpNumber int) *MetaApiPosition {
	// search for position wich clientID start with messageID
	for _, position := range positions {
		if position.ClientID != "" {
			// regex to check if clientId contain message id and tp number
			re := regexp.MustCompile(`^[^@]+@\d+_` + strconv.Itoa(messageID) + `_TP` + strconv.Itoa(tpNumber))
			if re.MatchString(position.ClientID) {
				return position
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
