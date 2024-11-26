package tgbot

import (
	"fmt"
	"math"
	"strconv"
)

// calculate profit real price in dollar price or loss price on tradeRequest
func calculateProfitPriceInDollar(openPrice float64, closeProfit float64, takeProfit float64, volume float64, symbole string) float64 {
	currencyPointSize := getCurrencyPointSize(symbole)

	profit := (closeProfit - openPrice) / currencyPointSize * volume
	return profit

}

func calculatePips(openPrice float64, closePrice float64, symbol string) float64 {
	currencyPointSize := getCurrencyPointSize(symbol)
	pips := (closePrice - openPrice) / currencyPointSize
	// use absolute value
	if pips < 0 {
		pips = math.Abs(pips)
	}
	return pips
}

// caculate volume size for trade request, minimum volume is 0.01
func (tgBot *TgBot) calculateVolumeSizeForTradeRequest(stopLossDistancePips float64, riskPercentage float64, accountBalance float64) float64 {
	// Calcul du risque en dollar
	riskInDollar := accountBalance * (riskPercentage / 100)
	// Calcul du volume en fonction du risque en dollar
	volume := riskInDollar / stopLossDistancePips
	return math.Round(volume*100) / 100
}
func (tgBot *TgBot) calculateVolumeSizeForTradeRequestByProfit(stopLossDistancePips float64, accountBalance float64, riskInDollar float64) float64 {
	// Calcul du volume en fonction du risque en dollar
	volume := riskInDollar / stopLossDistancePips
	return math.Round(volume*100) / 100
}

func getCurrencyPointSize(symbol string) float64 {
	currencyPointSizes := map[string]float64{
		"EURUSD":   0.0001,
		"USDJPY":   0.01,
		"GBPUSD":   0.0001,
		"USDCHF":   0.0001,
		"USDCAD":   0.0001,
		"AUDUSD":   0.0001,
		"NZDUSD":   0.0001,
		"EURJPY":   0.01,
		"GBPJPY":   0.01,
		"EURGBP":   0.0001,
		"EURCHF":   0.0001,
		"XAUUSD":   0.01,  // GOLD
		"XAGUSD":   0.001, // SILVER
		"BTCUSD":   1,     // Bitcoin
		"ETHUSD":   0.01,  // Ethereum
		"USOUSD":   0.01,  // WTI Crude Oil
		"BrentUSD": 0.01,  // Brent Crude Oil

	}
	// in case symbole contains suffix like XAUUSD-STD or XAUUSD-ECN
	if currencyPointSizes[symbol] == 0 {
		for key, value := range currencyPointSizes {
			if key == symbol[:len(key)] {
				return value
			}
		}

	}
	return currencyPointSizes[symbol]
}

func isTradeValidWith3TP(entryPrice, stopLoss, tp1, tp2, tp3, minRiskRewardRatio float64) bool {
	// Calcul du risque initial (différence entre le prix d'entrée et le stop loss)
	risk := entryPrice - stopLoss
	if risk <= 0 {
		fmt.Println("Erreur : le stop loss doit être inférieur au prix d'entrée.")
		return false
	}
	// Calcul des récompenses potentielles pour les TPs valides uniquement
	totalReward := 0.0
	validTPCount := 0

	// TP1 est toujours actif
	rewardTP1 := tp1 - entryPrice
	totalReward += rewardTP1
	validTPCount++

	// TP2 est pris en compte seulement s'il est valide (différent de -1)
	if tp2 != -1 {
		rewardTP2 := tp2 - entryPrice
		totalReward += rewardTP2
		validTPCount++
	}

	// TP3 est pris en compte seulement s'il est valide (différent de -1)
	if tp3 != -1 {
		rewardTP3 := tp3 - entryPrice
		totalReward += rewardTP3
		validTPCount++
	}

	// Si aucun TP valide n'est défini, on ne peut pas exécuter le trade
	if validTPCount == 0 {
		fmt.Println("Erreur : aucun TP valide n'est défini.")
		return false
	}

	// Calcul du ratio risque/récompense global en utilisant la moyenne des récompenses valides
	averageReward := totalReward / float64(validTPCount)
	globalRiskRewardRatio := averageReward / risk

	fmt.Printf("Ratio Risque/Récompense global : %.2f\n", globalRiskRewardRatio)

	// Vérification si le ratio global est suffisant
	return globalRiskRewardRatio >= minRiskRewardRatio
}

// get trading dynamic volume
func (tgBot *TgBot) GetTradingDynamicVolume(request *TradeRequest, price float64, accountBalance float64, channelId int, maxRiskableProfit float64) float64 {
	stopLoss := request.StopLoss
	riskPerTradePercentage := tgBot.RedisClient.GetRiskPercentage()
	entryPrice := price
	pipsToStopLoss := calculatePips(entryPrice, stopLoss, request.Symbol)
	// here we will evaluate the risk management of the trade request
	maxVolume := tgBot.RedisClient.GetTradingVolume(channelId)
	if maxRiskableProfit == -1 {
		// stopLoss distance in pips
		// dynamic maxVolume calculation
		dynamicVolume := tgBot.calculateVolumeSizeForTradeRequest(pipsToStopLoss, riskPerTradePercentage, accountBalance)
		if dynamicVolume <= maxVolume {
			// recorrection of maxVolume
			maxVolume = dynamicVolume
		}
	} else if maxVolume >= 0 {
		// calculate volume based on maxRiskableProfit
		dynammcVolume := tgBot.calculateVolumeSizeForTradeRequestByProfit(pipsToStopLoss, accountBalance, maxRiskableProfit)
		if dynammcVolume <= maxVolume {
			// recorrection of maxVolume
			maxVolume = dynammcVolume
		}
	}
	allocVolume := tgBot.calculateAllocatedVolume(channelId)
	if allocVolume <= maxVolume {
		maxVolume = allocVolume
	}
	return maxVolume
}
func (tgBot *TgBot) calculateAllocatedVolume(channelID int) float64 {
	channelScores, _ := tgBot.RedisClient.GetAllChannelScores()
	maxVolume := tgBot.RedisClient.GetDefaultTradingVolume()

	// Trouver le score maximum parmi tous les canaux
	var maxScore float64
	for _, score := range channelScores {
		if score > maxScore {
			maxScore = score
		}
	}
	// Calculer la proportion du volume alloué
	allocatedVolume := (channelScores[strconv.Itoa(channelID)] / maxScore) * maxVolume
	return allocatedVolume
}

// get traderequest possible loss in usd
func (tgBot *TgBot) GetTradeRequestPossibleLoss(request *TradeRequest, price float64) float64 {
	// here we will evaluate the risk management of the trade request
	entryPrice := price
	volume := request.Volume
	strategy := tgBot.RedisClient.GetStrategy()
	// stopLoss distance in pips
	pipsToStopLoss := 0.0
	// stack stoploss distance base on TPs if tp positive
	if request.TakeProfit1 > 0 {
		pipsToStopLoss = pipsToStopLoss + calculatePips(entryPrice, request.StopLoss, request.Symbol)
	}
	if strategy == "3TP" || strategy == "TP2" {
		if request.TakeProfit2 > 0 {
			pipsToStopLoss = pipsToStopLoss + calculatePips(entryPrice, request.StopLoss, request.Symbol)
		}
		if request.TakeProfit3 > 0 && strategy == "3TP" {
			pipsToStopLoss = pipsToStopLoss + calculatePips(entryPrice, request.StopLoss, request.Symbol)
		}
	}
	// loss calculation
	possibleLoss := pipsToStopLoss * volume
	return possibleLoss
}
