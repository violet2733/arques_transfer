package model

import (
	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
)

type (
	BinanceSpotOrderBook struct {
		Data      *binance.DepthResponse
		ProcessID int64
	}

	BinanceFutureOrderBook struct {
		Data      *futures.DepthResponse
		ProcessID int64
	}

	EventTypeChecker struct {
		EventType       string `json:"e"`
		EventTime       int64  `json:"E"`
		TransactionTime int64  `json:"T"`
	}

	OrderUpdate struct {
		EventType       string       `json:"e"`
		EventTime       int64        `json:"E"`
		TransactionTime int64        `json:"T"`
		Result          *OrderResult `json:"o"`
	}

	OrderResult struct {
		Symbol                   string  `json:"s"`
		ClientOrderID            string  `json:"c"`
		Side                     string  `json:"S"`
		Type                     string  `json:"o"`
		TimeInForce              string  `json:"f"`
		OriginalQuantity         float64 `json:"q,string"`
		OriginalPrice            float64 `json:"p,string"`
		AveragePrice             float64 `json:"ap,string"`
		StopPrice                float64 `json:"sp,string"` // Stop Price. Please ignore with TRAILING_STOP_MARKET order
		ExecutionType            string  `json:"x"`
		OrderStatus              string  `json:"X"`
		OrderID                  int64   `json:"i"`
		OrderLastFilledQuantity  float64 `json:"l,string"`
		OrderFilledAccumQuantity float64 `json:"z,string"` // Order Filled Accumulated Quantity
		LastFilledPrice          float64 `json:"L,string"`
		CommissionAsset          string  `json:"N"`        // Commission Asset, will not push if no commission
		Commission               float64 `json:"n,string"` // Commission, will not push if no commission
		OrderTradeTime           int64   `json:"T"`
		TraceID                  int64   `json:"t"`
		BidsNotional             float64 `json:"b,string"`
		AskNotional              float64 `json:"a,string"`
		IsMakerSide              bool    `json:"m"` // Is this trade the maker side?
		IsReduceOnly             bool    `json:"R"` // Is this reduce only
		StopPriceWorkingType     string  `json:"wt"`
		OriginalOrderType        string  `json:"ot"`
		PositionSide             string  `json:"ps"`
		IsCloseConditional       bool    `json:"cp"`        // If Close-All, pushed with conditional order
		ActivationPrice          float64 `json:"AP,string"` // Activation Price, only puhed with TRAILING_STOP_MARKET order
		CallbackRate             float64 `json:"cr,string"` // Callback Rate, only puhed with TRAILING_STOP_MARKET order
		RealizedProfit           float64 `json:"rp,string"` // Realized Profit of the trade
	}

	SpotOrderResult struct {
		EventType                         string  `json:"e"`
		EventTime                         int64   `json:"E"`
		Symbol                            string  `json:"s"`
		ClientOrderID                     string  `json:"c"`
		Side                              string  `json:"S"`
		OrderType                         string  `json:"o"`
		TimeInForce                       string  `json:"f"`
		OrderQuantity                     float64 `json:"q,string"`
		OrderPrice                        float64 `json:"p,string"`
		StopPrice                         float64 `json:"P,string"`
		IcebergQuantity                   float64 `json:"F,string"`
		OrderListId                       int64   `json:"g"`
		OriginalClientOrderID             string  `json:"C"`
		ExecutionType                     string  `json:"x"`
		OrderStatus                       string  `json:"X"`
		RejectReason                      string  `json:"r"`
		OrderID                           int64   `json:"i"`
		OrderLastFilledQuantity           float64 `json:"l,string"`
		OrderFilledAccumQuantity          float64 `json:"z,string"` // Order Filled Accumulated Quantity
		LastFilledPrice                   float64 `json:"L,string"`
		CommissionAsset                   string  `json:"N"`        // Commission Asset, will not push if no commission
		Commission                        float64 `json:"n,string"` // Commission, will not push if no commission
		OrderTradeTime                    int64   `json:"T"`
		TraceID                           int64   `json:"t"`
		Ignore                            int64   `json:"I"`
		IsOnOrderBook                     bool    `json:"w"`
		IsMakerSide                       bool    `json:"m"`
		IsIgnore                          bool    `json:"M"`
		OrderCreatedTime                  int64   `json:"O"`
		CumulativeQuoteTransactedQuantity float64 `json:"Z,string"` // Order Filled Accumulated Quantity
		LastQuoteTransactedQuantity       float64 `json:"Y,string"`
		QuoteOrderQuntity                 float64 `json:"Q,string"`
	}

	Balance struct {
		Asset              string `json:"a"`
		WalletBalance      string `json:"wb"`
		CrossWalletBalance string `json:"cw"`
	}

	Position struct {
		Symbol         string  `json:"s"`
		PositionAmount float64 `json:"pa,string"`
		EntryPrice     float64 `json:"ep,string"`
		AccumRealized  float64 `json:"cr,string"` // pre-fee
		UnrealizedPnL  float64 `json:"up,string"`
		MarginType     string  `json:"mt"`
		IsolatedWallet float64 `json:"iw,string"` // if isolated position
		PositionSide   string  `json:"ps"`
	}

	Account struct {
		EventResponseType string      `json:"m"`
		Balances          []*Balance  `json:"B"`
		Positions         []*Position `json:"P"`
	}

	AccountUpdate struct {
		EventType       string   `json:"e"`
		EventTime       int64    `json:"E"`
		TransactionTime int64    `json:"T"`
		Account         *Account `json:"a"`
	}
)

func (c *BinanceSpotOrderBook) BinanceSpotOrderBookFilterBid(d *binance.Bid) (index int, data *binance.Bid) {
	for i, s := range c.Data.Bids {
		if d.Price == s.Price {
			s.Quantity = d.Quantity
			return i, &s
		}
	}
	return -1, nil
}

func (c *BinanceSpotOrderBook) BinanceSpotOrderBookFilterAsk(d *binance.Ask) (index int, data *binance.Ask) {
	for i, s := range c.Data.Asks {
		if d.Price == s.Price {
			s.Quantity = d.Quantity
			return i, &s
		}
	}
	return -1, nil
}

func (c *BinanceSpotOrderBook) BinanceSpotOrderBookRemoveBid(idx int) *BinanceSpotOrderBook {
	var list []binance.Bid
	list = c.Data.Bids

	_list := append(list[:idx], list[idx+1:]...)
	c.Data.Bids = _list

	return c
}

func (c *BinanceSpotOrderBook) BinanceSpotOrderBookRemoveAsk(idx int) *BinanceSpotOrderBook {
	var list []binance.Ask
	list = c.Data.Asks

	_list := append(list[:idx], list[idx+1:]...)
	c.Data.Asks = _list

	return c
}

//

func (c *BinanceFutureOrderBook) BinanceFutureOrderBookFilterBid(d *futures.Bid) (index int, data *futures.Bid) {
	for i, s := range c.Data.Bids {
		if d.Price == s.Price {
			s.Quantity = d.Quantity
			return i, &s
		}
	}
	return -1, nil
}

func (c *BinanceFutureOrderBook) BinanceFutureOrderBookFilterAsk(d *futures.Ask) (index int, data *futures.Ask) {
	for i, s := range c.Data.Asks {
		if d.Price == s.Price {
			s.Quantity = d.Quantity
			return i, &s
		}
	}
	return -1, nil
}

func (c *BinanceFutureOrderBook) BinanceFutureOrderBookRemoveBid(idx int) *BinanceFutureOrderBook {
	var list []futures.Bid
	list = c.Data.Bids

	_list := append(list[:idx], list[idx+1:]...)
	c.Data.Bids = _list

	return c
}

func (c *BinanceFutureOrderBook) BinanceFutureOrderBookRemoveAsk(idx int) *BinanceFutureOrderBook {
	var list []futures.Ask
	list = c.Data.Asks

	_list := append(list[:idx], list[idx+1:]...)
	c.Data.Asks = _list

	return c
}
