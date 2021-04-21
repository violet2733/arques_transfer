package gopax

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type (
	GetOrdersService struct {
		c *Client
	}

	GetOrderService struct {
		c             *Client
		OrderID       string
		ClientOrderID string
	}

	OrderResponse struct {
		Id                    string        `json:"id"`
		ClientOrderID         string        `json:"clientOrderId"`
		Status                string        `json:"status"`
		ForceCompletionReason string        `json:"forcedCompletionReason"`
		TradingPairName       string        `json:"tradingPairName"`
		Side                  string        `json:"side"`
		Type                  string        `json:"type"`
		Price                 float64       `json:"price"`
		StopPrice             float64       `json:"stopPrice"`
		Amount                float64       `json:"amount"`
		Remaining             float64       `json:"remaining"`
		Protection            string        `json:"protection"`
		TimeInForce           string        `json:"timeInForce"`
		CreatedAt             string        `json:"createdAt"`
		UpdatedAt             string        `json:"updatedAt"`
		BalanceChange         BalanceChange `json:"balanceChange"`
	}

	BalanceChange struct {
		BaseGross  float64 `json:"baseGross"`
		BaseFee    Fee     `json:"baseFee"`
		BaseNet    float64 `json:"baseNet"`
		QuoteGross float64 `json:"quoteGross"`
		QuoteFee   Fee     `json:"quoteFee"`
		QuoteNet   float64 `json:"quoteNet"`
	}

	Fee struct {
		Taking float64 `json:"taking"`
		Making float64 `json:"making"`
	}
)

func (s *GetOrdersService) Do(ctx context.Context, opts ...RequestOption) (res []*OrderResponse, err error) {
	r := &request{
		method:     "GET",
		endpoint:   "/orders",
		secType:    secTypeSigned,
		recvWindow: -1,
	}

	data, err := s.c.callAPI(ctx, r, opts...)
	if err != nil {
		return nil, err
	}

	_data := string(data)
	_data = strings.ReplaceAll(_data, `"stopPrice":undefined`, `"stopPrice":0`)
	data = []byte(_data)

	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *GetOrderService) ClientOrderId(clientOrderId string) *GetOrderService {
	s.ClientOrderID = clientOrderId
	return s
}

func (s *GetOrderService) OrderId(orderId string) *GetOrderService {
	s.OrderID = orderId
	return s
}

func (s *GetOrderService) Do(ctx context.Context, opts ...RequestOption) (res *OrderResponse, err error) {
	_endpoint := "/orders"
	if s.OrderID != "" {
		_endpoint = _endpoint + "/" + s.OrderID
	}

	if s.ClientOrderID != "" {
		_endpoint = _endpoint + "/clientOrderId/" + s.ClientOrderID
	}

	r := &request{
		method:     "GET",
		endpoint:   _endpoint,
		secType:    secTypeSigned,
		recvWindow: -1,
	}

	data, err := s.c.callAPI(ctx, r, opts...)
	if err != nil {
		return nil, err
	}

	_data := string(data)
	_data = strings.ReplaceAll(_data, `"stopPrice":undefined`, `"stopPrice":0`)
	data = []byte(_data)

	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

type (
	CreateOrderService struct {
		c               *Client
		clientOrderID   *string
		tradingPairName string
		side            string
		_type           string
		price           int64
		stopPrice       *int64
		amount          float64
		protection      *string
		timeInForce     *string
	}

	CreateOrderResponse struct {
		Id                    string        `json:"id"`
		ClientOrderID         string        `json:"clientOrderId"`
		Status                string        `json:"status"`
		ForceCompletionReason string        `json:"forcedCompletionReason"`
		TradingPairName       string        `json:"tradingPairName"`
		Side                  string        `json:"side"`
		Type                  string        `json:"type"`
		Price                 float64       `json:"price"`
		StopPrice             float64       `json:"stopPrice"`
		Amount                float64       `json:"amount"`
		Remaining             float64       `json:"remaining"`
		Protection            string        `json:"protection"`
		TimeInForce           string        `json:"timeInForce"`
		CreatedAt             string        `json:"createdAt"`
		BalanceChange         BalanceChange `json:"balanceChange"`
	}
)

func (s *CreateOrderService) ClientOrderID(clientOrderId string) *CreateOrderService {
	s.clientOrderID = &clientOrderId
	return s
}

func (s *CreateOrderService) TradingPairName(tradingPairName string) *CreateOrderService {
	s.tradingPairName = tradingPairName
	return s
}

func (s *CreateOrderService) Side(side string) *CreateOrderService {
	s.side = side
	return s
}

func (s *CreateOrderService) Type(_type string) *CreateOrderService {
	s._type = _type
	return s
}

func (s *CreateOrderService) Price(price int64) *CreateOrderService {
	s.price = price
	return s
}

func (s *CreateOrderService) StopPrice(stopPrice int64) *CreateOrderService {
	s.stopPrice = &stopPrice
	return s
}

func (s *CreateOrderService) Amount(amount float64) *CreateOrderService {
	s.amount = amount
	return s
}

func (s *CreateOrderService) Protection(protection string) *CreateOrderService {
	s.protection = &protection
	return s
}

func (s *CreateOrderService) TimeInForce(timeInForce string) *CreateOrderService {
	s.timeInForce = &timeInForce
	return s
}

func (s *CreateOrderService) Do(ctx context.Context, opts ...RequestOption) (res *CreateOrderResponse, err error) {
	r := &request{
		method:     "POST",
		endpoint:   "/orders",
		secType:    secTypeSigned,
		recvWindow: 2000,
	}

	m := params{
		"tradingPairName": s.tradingPairName,
		"side":            s.side,
		"type":            s._type,
		"price":           s.price,
		"amount":          s.amount,
	}

	if s.clientOrderID != nil {
		m["clientOrderId"] = *s.clientOrderID
	}
	if s.stopPrice != nil {
		m["stopPrice"] = *s.stopPrice
	}
	if s.protection != nil {
		m["protection"] = *s.protection
	}
	if s.timeInForce != nil {
		m["timeInForce"] = *s.timeInForce
	}

	// r.setFormParams(m)
	r.param = m

	data, err := s.c.callAPI(ctx, r, opts...)
	if err != nil {
		fmt.Println(string(data))
		return nil, err
	}

	_data := string(data)
	_data = strings.ReplaceAll(_data, `"stopPrice":undefined`, `"stopPrice":0`)
	data = []byte(_data)

	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

type (
	DeleteOrderService struct {
		c             *Client
		OrderID       string
		ClientOrderID string
	}
)

func (s *DeleteOrderService) ClientOrderId(clientOrderId string) *DeleteOrderService {
	s.ClientOrderID = clientOrderId
	return s
}

func (s *DeleteOrderService) OrderId(orderId string) *DeleteOrderService {
	s.OrderID = orderId
	return s
}

func (s *DeleteOrderService) Do(ctx context.Context, opts ...RequestOption) (res *interface{}, err error) {
	_endpoint := "/orders"
	if s.OrderID != "" {
		_endpoint = _endpoint + "/" + s.OrderID
	}

	if s.ClientOrderID != "" {
		_endpoint = _endpoint + "/clientOrderId/" + s.ClientOrderID
	}

	r := &request{
		method:     "DELETE",
		endpoint:   _endpoint,
		secType:    secTypeSigned,
		recvWindow: 2000,
	}

	data, err := s.c.callAPI(ctx, r, opts...)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}
