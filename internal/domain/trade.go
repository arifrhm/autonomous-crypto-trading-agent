package domain

import "time"

// Trade represents an executed trade on the exchange.
type Trade struct {
	EventTime    time.Time `json:"event_time"`
	TradeTime    time.Time `json:"trade_time"`
	Symbol       string    `json:"symbol"`
	TradeID      int64     `json:"trade_id"`
	Price        float64   `json:"price"`
	Quantity     float64   `json:"quantity"`
	BuyerOrderID int64     `json:"buyer_order_id,omitempty"`
	SellerOrderID int64    `json:"seller_order_id,omitempty"`
	IsBuyerMaker bool      `json:"is_buyer_maker"`
}
