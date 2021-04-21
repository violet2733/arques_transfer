package gopax

import (
	"crypto/hmac"
	"crypto/sha512"
	base64 "encoding/base64"
	"net/url"
	"strconv"
	"time"

	"arques.com/transfer/config"
)

var (
	baseURL = "wss://wsapi.gopax.co.kr"
	// WebsocketTimeout is an interval for sending ping/pong messages if WebsocketKeepalive is enabled
	WebsocketTimeout = time.Second * 60
	// WebsocketKeepalive enables sending ping/pong messages to check the connection stability
	WebsocketKeepalive = false
)

type (
	WsPublicRequest struct {
		Name string              `json:"n"`
		Data WsPublicRequestData `json:"o"`
	}

	WsPublicRequestData struct {
		TradingPairName string `json:"tradingPairName"`
	}

	WsPublicTradeEvent struct {
		Info int               `json:"i"`
		Name string            `json:"n"`
		Data WsPublicTradeData `json:"o"`
	}

	WsPublicTradeData struct {
		TradeId         int64   `json:"tradeId"`
		BaseAmount      float64 `json:"baseAmount"`
		QuoteAmount     float64 `json:"quoteAmount"`
		Price           int64   `json:"price"`
		IsBuy           bool    `json:"isBuy"`
		OccurredAt      int64   `json:"occurredAt"`
		TradingPairName string  `json:"tradingPairName"`
	}

	WsPublicOrderBookDetail struct {
		EntryId   int64   `json:"entryId"`
		Price     float64 `json:"price"`
		Volume    float64 `json:"volume"`
		UpdatedAt float64 `json:"updatedAt"`
	}

	WsPublicOrderBookData struct {
		Ask             []*WsPublicOrderBookDetail `json:"ask"`
		Bid             []*WsPublicOrderBookDetail `json:"bid"`
		TradingPairName string                     `json:"tradingPairName"`
		MaxEntryId      int                        `json:"maxEntryId"`
	}

	WsPublicOrderBookEvent struct {
		Info int                   `json:"i"`
		Name string                `json:"n"`
		Data WsPublicOrderBookData `json:"o"`
	}

	WsPrivateRequest struct {
		Name string      `json:"n"`
		Data interface{} `json:"o"`
	}

	WsPrivateEvent struct {
		Name string `json:"n"`
	}

	WsPrivateOrdersResponseEvent struct {
		Name string                 `json:"n"`
		Data []*WsPrivateOrdersData `json:"o"`
	}

	WsPrivateOrdersPushEvent struct {
		Info int                  `json:"i"`
		Name string               `json:"n"`
		Data *WsPrivateOrdersData `json:"o"`
	}

	WsPrivateOrdersData struct {
		OrderID                int64   `json:"orderId"`
		Status                 int16   `json:"status"`
		Side                   int16   `json:"side"`
		Type                   int16   `json:"type"`
		Price                  int64   `json:"price"`
		OrgAmount              float64 `json:"orgAmount"`
		RemainAmount           float64 `json:"remainAmount"`
		CreatedAt              int64   `json:"createdAt"`
		UpdatedAt              int64   `json:"updatedAt"`
		TradeBaseAmount        float64 `json:"tradeBaseAmount"`
		TradeQuoteAmount       float64 `json:"tradeQuoteAmount"`
		FeeAmount              float64 `json:"feeAmount"`
		RewardAmount           float64 `json:"rewardAmount"`
		TimeInForce            int16   `json:"timeInForce"`
		Protection             int16   `json:"protection"`
		ForcedCompletionReason int16   `json:"forcedCompletionReason"`
		StopPrice              int64   `json:"stopPrice"`
		TakerFeeAmount         float64 `json:"takerFeeAmount"`
		TradingPairName        string  `json:"tradingPairName"`
	}

	WsPrivateTradesResponseEvent struct {
		Name string      `json:"n"`
		Data interface{} `json:"o"`
	}

	WsPrivateTradesPushEvent struct {
		Info int64            `json:"i"`
		Name string           `json:"n"`
		Data *WsPrivateTrades `json:"o"`
	}

	WsPrivateTrades struct {
		TradeId         int64   `json:"tradeId"`
		OrderId         int64   `json:"orderId"`
		Side            int16   `json:"side"`
		Type            int16   `json:"type"`
		BaseAmount      float64 `json:"baseAmount"`
		QuoteAmount     float64 `json:"quoteAmount"`
		Fee             float64 `json:"fee"`
		Price           int64   `json:"price"`
		IsSelfTrade     bool    `json:"isSelfTrade"`
		OccurredAt      int64   `json:"occurredAt"`
		TradingPairName string  `json:"tradingPairName"`
	}
)

type SideType string

const (
	SideTypeBuy  SideType = "BUY"
	SideTypeSell SideType = "SELL"
)

type WsPublicTradeHandler func(event *WsPublicTradeEvent)

func setEndPoint() string {
	timestamp := strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10)
	msg := "t" + timestamp
	key, _ := base64.StdEncoding.DecodeString(config.Get().Gopax.SecretKey)
	mac := hmac.New(sha512.New, key)
	mac.Write([]byte(msg))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	url := "wss://wsapi.gopax.co.kr" +
		"?apiKey=" + url.QueryEscape(config.Get().Gopax.ApiKey) +
		"&timestamp=" + timestamp +
		"&signature=" + url.QueryEscape(signature)

	return url
}

func WsPublicTradeServe(symbol string, handler WsHandler, errHandler ErrHandler) (doneC, stopC chan struct{}, err error) {
	endpoint := setEndPoint()
	// fmt.Println(endpoint)
	cfg := newWsConfig(endpoint)

	requestData := &WsPublicRequestData{
		TradingPairName: symbol,
	}

	cfg.PublicRequest = &WsPublicRequest{
		Name: "SubscribeToTradingPair",
		Data: *requestData,
	}

	return wsServe(cfg, handler, errHandler)
}

func WsPublicOrderBookServe(symbol string, handler WsHandler, errHandler ErrHandler) (doneC, stopC chan struct{}, err error) {
	endpoint := setEndPoint()
	// fmt.Println(endpoint)
	cfg := newWsConfig(endpoint)

	requestData := &WsPublicRequestData{
		TradingPairName: symbol,
	}

	cfg.PublicRequest = &WsPublicRequest{
		Name: "SubscribeToOrderBook",
		Data: *requestData,
	}

	return wsServe(cfg, handler, errHandler)
}

func (w *WsPublicOrderBookData) Filter(sideType SideType, d *WsPublicOrderBookDetail) (index int, data *WsPublicOrderBookDetail) {
	var list []*WsPublicOrderBookDetail
	if sideType == SideTypeBuy {
		list = w.Bid
	} else {
		list = w.Ask
	}

	for i, s := range list {
		if d.Price == s.Price {
			s.Volume = d.Volume
			s.UpdatedAt = d.UpdatedAt
			return i, s
		}
	}

	return -1, nil
}

func (w *WsPublicOrderBookData) Remove(sideType SideType, idx int) *WsPublicOrderBookData {
	var list []*WsPublicOrderBookDetail
	if sideType == SideTypeBuy {
		list = w.Bid
	} else {
		list = w.Ask
	}

	_list := append(list[:idx], list[idx+1:]...)

	if sideType == SideTypeBuy {
		w.Bid = _list
	} else {
		w.Ask = _list
	}

	return w
}

func WsPrivateOrdersServe(handler WsHandler, errHandler ErrHandler) (doneC, stopC chan struct{}, err error) {
	endpoint := setEndPoint()
	// fmt.Println(endpoint)
	cfg := newWsConfig(endpoint)

	cfg.PrivateRequest = &WsPrivateRequest{
		Name: "SubscribeToOrders",
	}

	return wsServe(cfg, handler, errHandler)
}

func WsPrivateUserTradesServe(handler WsHandler, errHandler ErrHandler) (doneC, stopC chan struct{}, err error) {
	endpoint := setEndPoint()
	// fmt.Println(endpoint)
	cfg := newWsConfig(endpoint)

	cfg.PrivateRequest = &WsPrivateRequest{
		Name: "SubscribeToTrades",
	}

	return wsServe(cfg, handler, errHandler)
}
