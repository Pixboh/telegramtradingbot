package tgbot

import (
	"strconv"
	"time"
)

type TraderScore struct {
	Score      float64   // Score du trader
	LastUpdate time.Time // Dernière mise à jour du score
}

// calculate profit rate and update trader scores based on a list of positions
func (tgBot *TgBot) updateTraderScores() {
	positions, err := tgBot.getMonthPositions()
	if err != nil {
		return
	}
	scores := tgBot.updateScores(positions)
	tgBot.RedisClient.SaveChannelScore(scores)
}

// updateScores met à jour les scores des traders en fonction de leurs positions et du temps.
func (tgBot *TgBot) updateScores(positions []MetaApiPosition) map[string]float64 {
	traderScores := make(map[string]float64)
	channels := tgBot.RedisClient.GetChannels()
	for _, chanId := range channels {
		traderScores[strconv.FormatInt(chanId, 10)] = 12
	}
	now := time.Now()
	traderLastTradeTime := make(map[string]string)
	for _, pos := range positions {
		// Récupérer l'ID du trader
		traderIDint := extractChannelIDFromClientId(pos.ClientID)
		if traderIDint == 0 {
			continue
		}
		traderID := strconv.Itoa(traderIDint)
		if !tgBot.RedisClient.IsChannelExist(int64(traderIDint)) {
			continue
		}

		// Appliquer les règles de scoring
		if pos.isBreakeven() {
			continue
		}
		tpNumber := extractTPFromClientId(pos.ClientID)
		if pos.Profit > 0 {
			if tpNumber == 1 {
				traderScores[traderID] += 1
			} else if tpNumber == 2 {
				traderScores[traderID] += 2
			} else if tpNumber == 3 {
				traderScores[traderID] += 3
			}
		} else if pos.Profit < -1 {
			traderScores[traderID] += -2
		}
		// minimum score is 18
		traderLastTradeTime[traderID] = pos.Time
	}

	// Appliquer une récompense progressive ou une réduction temporelle
	for id, lastPosTime := range traderLastTradeTime {

		if traderScores[id] < 0 {
			tradeTimeTime, err := time.Parse(time.RFC3339, lastPosTime)
			if err != nil {

			}
			elapsed := now.Sub(tradeTimeTime).Hours() / 24 // Nombre de jours depuis la dernière mise à jour

			// Augmenter légèrement le lastPosTime chaque jour (progression lente)
			if elapsed > 0 {
				traderScores[id] = traderScores[id] + elapsed*3
			}
		}
	}
	return traderScores
}
