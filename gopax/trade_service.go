package gopax

import (
	"context"
	"encoding/json"
)

type (
	Trade struct {
		ID              int64   `json:"id"`
		OrderID         int64   `json:"orderId"`
		BaseAmount      float64 `json:"baseAmount"`
		QuoteAmount     float64 `json:"quoteAmount"`
		Fee             float64 `json:"fee"`
		Price           float64 `json:"price"`
		Timestamp       string  `json:"timestamp"`
		Side            string  `json:"side"`
		TradingPairName string  `json:"tradingPairName"`
		Position        string  `json:"position"`
	}

	GetTradesService struct {
		c               *Client
		limit           int
		pastmax         int
		latestmin       int
		after           int
		before          int
		deepSearch      bool
		tradingPairName string
	}
)

func (s *GetTradesService) Do(ctx context.Context, opts ...RequestOption) (res []*Trade, err error) {
	r := &request{
		method:     "GET",
		endpoint:   "/trades",
		secType:    secTypeSigned,
		recvWindow: -1,
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
