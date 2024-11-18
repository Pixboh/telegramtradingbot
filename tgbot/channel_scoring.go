package tgbot

import (
	"fmt"
	"strconv"
	"time"
)

type TraderScore struct {
	Score      float64   // Score du trader
	LastUpdate time.Time // Dernière mise à jour du score
}

// calculate profit rate and update trader scores based on a list of positions
func (tgBot *TgBot) updateTraderScores(positions []MetaApiPosition, traderScores map[string]*TraderScore) {
	updateScores(positions, traderScores)
}

// updateScores met à jour les scores des traders en fonction de leurs positions et du temps.
func updateScores(positions []MetaApiPosition, traderScores map[string]*TraderScore) {
	now := time.Now()

	for _, pos := range positions {
		// Récupérer l'ID du trader
		traderIDint := extractChannelIDFromClientId(pos.ClientID)
		if traderIDint == 0 {
			continue
		}
		traderID := strconv.Itoa(traderIDint)

		// Initialiser le score si nécessaire
		if _, exists := traderScores[traderID]; !exists {
			traderScores[traderID] = &TraderScore{Score: 0, LastUpdate: now}
		}

		trader := traderScores[traderID]

		// Appliquer les règles de scoring
		if pos.Profit > 0 {
			trader.Score += pos.Profit * 0.5 // Récompense pour les gains
		} else if pos.Profit < 0 {
			trader.Score += pos.Profit * 1.5 // Pénalité pour les pertes
		}

		// Mettre à jour la date du dernier trade
		trader.LastUpdate = now
	}

	// Appliquer une récompense progressive ou une réduction temporelle
	for id, score := range traderScores {
		elapsed := now.Sub(score.LastUpdate).Hours() / 24 // Nombre de jours depuis la dernière mise à jour

		// Augmenter légèrement le score chaque jour (progression lente)
		if elapsed > 0 {
			score.Score += elapsed * 0.1
		}

		// Réduire le score des traders avec de mauvaises performances
		if score.Score < 0 {
			score.Score += elapsed * 0.05 // Pénalité négative diminue avec le temps
		}

		// Empêcher les scores excessivement bas ou hauts
		if score.Score < -100 {
			score.Score = -100
		} else if score.Score > 1000 {
			score.Score = 1000
		}

		// Afficher les résultats intermédiaires
		fmt.Printf("Trader %s: Score=%.2f\n", id, score.Score)
	}
}
