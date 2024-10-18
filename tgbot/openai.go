package tgbot

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sashabaranov/go-openai"
	"log"
	"strings"
)

func (tgBot *TgBot) GptParseNewMessage(message string, apiKey string, symbols []string) (*TradeRequest, error) {
	// Créer des exemples d'instructions avec actionType et zone d'entrée inclus
	// Créer une requête ChatCompletion pour le message à analyser
	clientOpenApi := openai.NewClient(apiKey)
	f := fmt.Sprintf("Voici les symboles disponibles: %v", symbols)
	if f == "" {

	}
	resp, err := clientOpenApi.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				// inform ai about available symbols
				{
					Role:    "user",
					Content: fmt.Sprintf("Voici les symboles disponibles: %v", symbols),
				},
				{
					Role:    "assistant",
					Content: `D'accord je vais les prendre en compte`,
				},
				{
					Role:    "user",
					Content: "Exemple: 🛑 JE VENDS BTCUSD \n\nZone d’entrée : 62 700 - 62 650\n\n⚠️ Adaptez le lot en fonction de votre capital, Appliquez la stratégie des 3TP\n\n🎯 TP1 : 62 500\n🎯 TP2 : 62 000\n🎯 TP3 : Ouvert\n\nSL : 63 700 🔒",
				},
				{
					Role: "assistant",
					Content: `{
  "actionType": "ORDER_TYPE_SELL",
  "symbol": "BTCUSD",
  "stopLoss": 63700,
  "takeProfit1": 62500,
  "takeProfit2": 62000,
  "takeProfit3": -1, 
  "entryZoneMin": 62650,
  "entryZoneMax": 62700
}`,
				},
				{
					Role:    "user",
					Content: "BUY BTCUSD \n\nEntry price 62300\n\n🔴 SL : 61300\n\n🟢 TP1 : 62500\n\n🟢 TP2 : 62800\n\n🟢 TP3 : 63300\n\n⚠️ DISCLAIMER : Il ne s’agit en aucun cas d’un conseil en investissement, mais uniquement d’une alerte à titre éducatif",
				},
				{
					Role: "assistant",
					Content: `{
  "actionType": "ORDER_TYPE_BUY",
  "symbol": "BTCUSD",
  "stopLoss": 61300,
  "takeProfit1": 62500,
  "takeProfit2": 62800,
  "takeProfit3": 63300", 
  "entryZoneMin": 62300,
  "entryZoneMax": -1 
}`,
				},
				{
					Role:    "user",
					Content: "BUY BTCUSD \n\nEntry price 62200\n\n🔴 SL : 61300\n\n🟢 TP1 : 62500\n\n🟢 TP2 : 62800\n\n🟢 TP3 : 63300\n\n⚠️ DISCLAIMER : Il ne s’agit en aucun cas d’un conseil en investissement, mais uniquement d’une alerte à titre éducatif",
				},
				{
					Role: "assistant",
					Content: `{
  "actionType": "ORDER_TYPE_BUY",
  "symbol": "BTCUSD",
  "stopLoss": 61300,
  "takeProfit1": 62500,
  "takeProfit2": 62800,
  "takeProfit3": 63300, 
  "entryZoneMin": 62200,
  "entryZoneMax": -1 
}`,
				},
				{
					Role:    "user",
					Content: "SELL BTCUSD\n\nEntry price 61340\n\n🔴 SL : 62340\n\n🟢 TP1 : 61100\n\n🟢 TP2 : 60830\n\n🟢 TP3 : 60340\n\n⚠️ DISCLAIMER : Il ne s’agit en aucun cas d’un conseil en investissement, mais uniquement d’une alerte à titre éducatif",
				},
				{
					Role: "assistant",
					Content: `{
  "actionType": "ORDER_TYPE_SELL",
  "symbol": "BTCUSD",
  "stopLoss": 62340,
  "takeProfit1": 61100,
  "takeProfit2": 60830,
  "takeProfit3": 60340, 
  "entryZoneMin": 61340,
  "entryZoneMax": -1 
}`,
				},
				{
					Role:    "user",
					Content: "🔴 VENTE GOLD (2)🍯 \n\nZone d'entrée : 2667 - 2666.5\n\n🚨 Lot à adapter selon votre capital \n\n🙉 TP1 : 2663\n🙊 TP2 : 2661\n🙈 TP3 : OUVERT\n\n🔴 SL : 2671",
				},
				{
					Role: "assistant",
					Content: `{
				 "actionType": "ORDER_TYPE_SELL",
				 "symbol": "XAUUSD",
				 "stopLoss": 2671,
				 "takeProfit1": 2663,
				 "takeProfit2": 2661,
				 "takeProfit3": -1,
				 "entryZoneMin": 2666.5,
				 "entryZoneMax": 2667
				}`,
				},
				{
					Role:    "user",
					Content: "USDJPY BUY Entry at 148.15\n\n🔴Stop loss :  147.60\n\n🟢Take profit 1 = 148.53\n🟢Take profit 2 = 148.83\n🟢Take profit 3 = 149.33\n\n⚠️ DISCLAIMER : Il ne s’agit en aucun cas d’un conseil en investissement, mais uniquement d’une alerte à titre éducatif",
				},
				{
					Role: "assistant",
					Content: ` {
  "actionType": "ORDER_TYPE_BUY",
  "symbol": "USDJPY",
  "stopLoss": 147.60,
  "takeProfit1": 148.53,
  "takeProfit2": 148.83,
  "takeProfit3": 149.33, 
  "entryZoneMin": 148.15,
  "entryZoneMax": -1
}
`,
				},
				{
					Role:    "user",
					Content: "USDJPY BUY Entry at 148.20\n\n🔴Stop loss :  147.60\n\n🟢Take profit 1 = 148.53\n🟢Take profit 2 = 148.83\n🟢Take profit 3 = 149.33\n\n⚠️ DISCLAIMER : Il ne s’agit en aucun cas d’un conseil en investissement, mais uniquement d’une alerte à titre éducatif",
				},
				{
					Role: "assistant",
					Content: ` {
  "actionType": "ORDER_TYPE_BUY",
  "symbol": "USDJPY",
  "stopLoss": 147.60,
  "takeProfit1": 148.53,
  "takeProfit2": 148.83,
  "takeProfit3": 149.33, 
  "entryZoneMin": 148.20,
  "entryZoneMax": -1 
}
`,
				},
				{
					Role:    "user",
					Content: message, // Message reçu à analyser
				},
			},
		},
	)

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return nil, err
	} else {
		// Afficher la réponse du modèle
		fmt.Println("Réponse du modèle :", resp.Choices[0].Message.Content)
	}

	// Extraction de la réponse
	parsedContent := resp.Choices[0].Message.Content

	// Initialiser un objet TradeRequest
	var tradeRequest TradeRequest

	// Parser la réponse JSON dans l'objet TradeRequest
	errJson := json.Unmarshal([]byte(parsedContent), &tradeRequest)
	if errJson != nil {
		return nil, errJson
	}

	// if generated symbol is not in list of symbol take the symbole that contain the full symbol name in the list
	if !StringInSlice(tradeRequest.Symbol, symbols) {
		for _, symbol := range symbols {
			// check if generated symbol is contain example "XAUUSD" is in "XAUUSD-STD" do a string search comparaison
			if strings.Contains(symbol, tradeRequest.Symbol) {
				tradeRequest.Symbol = symbol
				break
			}

		}
	}
	/// display trade log in perfect json readable
	log.Println("TradeRequest struct: %+v\n", tradeRequest)

	return &tradeRequest, errJson
}

func StringInSlice(symbol string, symbols []string) bool {
	for _, s := range symbols {
		if s == symbol {
			return true
		}
	}
	return false
}

func GptParseUpdateMessage(message string, apiKey string) (*TradeUpdateRequest, error) {
	// here we parse message made for an update on a current position to modify or close trade
	clientOpenApi := openai.NewClient(apiKey)
	resp, err := clientOpenApi.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role: "user",
					Content: "Voici les different update types qui existe : TP1_HIT , TP2_HIT, TP3_HIT, TP4_HIT ," +
						" STOPLOSS_HIT, CLOSE_TRADE, CLOSE_TO_ENTRY_PRICE, MODIFY_STOPLOSS , SL_TO_ENTRY_PRICE",
				},
				{
					Role:    "assistant",
					Content: `D'accord je ne mettrais que ces updates types dans les json que je vais generer'`,
				},
				{
					Role:    "user",
					Content: "J'attend une reponse en JSON avec les exemple que je vais te proposer",
				},
				{
					Role:    "assistant",
					Content: `Ok je vais repondre en json`,
				},
				{
					Role:    "user",
					Content: "Exemple: TP1 TOUCHÉ 💸\n\nSL AU PRIX D'ENTRÉE",
				},
				{
					Role: "assistant",
					Content: `{
  "updateType": "TP1_HIT"
}`,
				},
				{
					Role:    "user",
					Content: "Exemple: TP2 TOUCHÉ 💸",
				},
				{
					Role: "assistant",
					Content: `{
  "updateType": "TP2_HIT"
}`,
				},
				{
					Role:    "user",
					Content: "Exemple: BTCUSD - TP1 HIT ✅",
				},
				{
					Role: "assistant",
					Content: `{
  "updateType": "TP1_HIT"
}`,
				},
				{
					Role:    "user",
					Content: "Exemple: BTCUSD - TP2 HIT ✅",
				},
				{
					Role: "assistant",
					Content: `{
  "updateType": "TP2_HIT"
}`,
				},
				{
					Role:    "user",
					Content: "Exemple: BTCUSD - TP3 HIT ✅",
				},
				{
					Role: "assistant",
					Content: `{
  "updateType": "TP3_HIT"
}`,
				},
				{
					Role:    "user",
					Content: "Exemple: SL HIT✖️ ",
				},
				{
					Role: "assistant",
					Content: `{
  "updateType": "STOPLOSS_HIT"
}`,
				},
				{
					Role:    "user",
					Content: "Exemple: SL TOUCHE✖️ ",
				},
				{
					Role: "assistant",
					Content: `{
  "updateType": "STOPLOSS_HIT"
}`,
				},

				{
					Role:    "user",
					Content: "Exemple: Fermez le trade \nmaintenant au prix d'entrée ✅",
				},
				{
					Role: "assistant",
					Content: `{
  "updateType": "CLOSE_TO_ENTRY_PRICE"
}`,
				},
				{
					Role:    "user",
					Content: "Exemple: PRENEZ LE TP1 MAINTENANT À 2660.5$ +25 PIPS ✔️\n\nSL AU PRIX D’ENTRÉE.",
				},
				{
					Role: "assistant",
					Content: `{
  "updateType": "TP1_HIT"
}`,
				}, {
					Role:    "user",
					Content: "Exemple: PRENEZ LE TP3 MAINTENANT À 2669.5$ +25 PIPS ✔️\n\nSL AU PRIX D’ENTRÉE.",
				},
				{
					Role: "assistant",
					Content: `{
  "updateType": "TP3_HIT"
}`,
				},
				{
					Role:    "user",
					Content: "Exemple: PRENEZ LE TP2 MAINTENANT À 2665.5$ +25 PIPS ✔️\n\nSL AU PRIX D’ENTRÉE.",
				},
				{
					Role: "assistant",
					Content: `{
  "updateType": "TP2_HIT"
}`,
				},
				{
					Role:    "user",
					Content: "Exemple: EURAUD - TP1 HIT✅",
				},
				{
					Role: "assistant",
					Content: `{
  "updateType": "TP1_HIT"
}`,
				},
				{
					Role:    "user",
					Content: "Exemple: Fermez le trade \nmaintenant à 60220$",
				},
				{
					Role: "assistant",
					Content: `{
  "updateType": "CLOSE_TRADE"
}`,
				},
				{
					Role:    "user",
					Content: "Exemple: 🎯 TP1 +90PIPS",
				},
				{
					Role: "assistant",
					Content: `{
  "updateType": "TP1_HIT"
}`,
				},
				{
					Role:    "user",
					Content: "Exemple: ⚠️ SL : 61700",
				},
				{
					Role: "assistant",
					Content: `{
  "updateType": "MODIFY_STOPLOSS",
  "value" : 61700
}`,
				},
				{
					Role:    "user",
					Content: "Exemple: ⚠️ Securisez le trade",
				},
				{
					Role: "assistant",
					Content: `{
  "updateType": "SL_TO_ENTRY_PRICE"
}`,
				},
				{
					Role:    "user",
					Content: message,
				},
			},
		},
	)

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return nil, err
	} else {
		// Afficher la réponse du modèle
		fmt.Println("Réponse du modèle :", resp.Choices[0].Message.Content)
	}

	// Extraction de la réponse
	parsedContent := resp.Choices[0].Message.Content

	// Initialiser un objet TradeRequest
	var tradeRequest TradeUpdateRequest

	// Parser la réponse JSON dans l'objet TradeRequest
	errJson := json.Unmarshal([]byte(parsedContent), &tradeRequest)
	if errJson != nil {
		return nil, errJson
	}

	// Afficher l'objet TradeRequest
	fmt.Printf("TradeRequest struct: %+v\n", tradeRequest)
	return &tradeRequest, errJson
}
