package tgbot

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
type TradeUpdateRequest struct {
	UpdateType string   `json:"updateType,omitempty"`
	Value      *float64 `json:"value,omitempty"`
}

const (
	TP1 = iota
	TP2 = iota
	TP3 = iota
)
