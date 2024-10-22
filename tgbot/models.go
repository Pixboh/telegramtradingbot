package tgbot

import (
	"strconv"
	"time"
)

type ChatGPTRequest struct {
	Model    string        `json:"model,omitempty"`
	Messages []ChatMessage `json:"messages,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type ChatGPTResponse struct {
	Choices []Choice `json:"choices,omitempty"`
}

type Choice struct {
	Message ChatMessage `json:"message,omitempty"`
}

type TradeRequest struct {
	ActionType   string  `json:"actionType,omitempty"`
	Symbol       string  `json:"symbol,omitempty"`
	Volume       float64 `json:"volume,omitempty"`
	StopLoss     float64 `json:"stopLoss,omitempty"`
	TakeProfit1  float64 `json:"takeProfit1,omitempty"`
	TakeProfit2  float64 `json:"takeProfit2,omitempty"`
	TakeProfit3  float64 `json:"takeProfit3,omitempty"`
	EntryZoneMin float64 `json:"entryZoneMin,omitempty"`
	EntryZoneMax float64 `json:"entryZoneMax,omitempty"`
	MessageId    *int    `json:"messageId,omitempty"`
}

// generate a trade request unique identifier base on field values without volumes

func (tr *TradeRequest) GenerateTradeRequestKey() string {
	tp1String := strconv.FormatFloat(tr.TakeProfit1, 'f', -1, 64)
	tp2String := strconv.FormatFloat(tr.TakeProfit2, 'f', -1, 64)
	tp3String := strconv.FormatFloat(tr.TakeProfit3, 'f', -1, 64)
	slString := strconv.FormatFloat(tr.StopLoss, 'f', -1, 64)
	ezMinString := strconv.FormatFloat(tr.EntryZoneMin, 'f', -1, 64)
	ezMaxString := strconv.FormatFloat(tr.EntryZoneMax, 'f', -1, 64)
	// today date in format DD-MM
	today := time.Now()
	todayString := today.Format("02-01")
	return todayString + tr.ActionType + tr.Symbol + tp1String + tp2String + tp3String + slString + ezMinString + ezMaxString
}

type TradeUpdateRequest struct {
	UpdateType string   `json:"updateType,omitempty"`
	Value      *float64 `json:"value,omitempty"`
}

const (
	TP1 = iota
	TP2 = iota
	TP3 = iota
)
