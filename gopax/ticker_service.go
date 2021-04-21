package gopax

import (
	"context"
	"encoding/json"
	"fmt"
)

type (
	GetTickerService struct {
		c               *Client
		TradingPairName string
	}

	TickerResponse struct {
		Price       int64   `json:"price"`
		Ask         int64   `json:"ask"`
		AskVolume   float64 `json:"askVolume"`
		Bid         int64   `json:"bid"`
		BidVolume   float64 `json:"bidVolume"`
		QuoteVolume float64 `json:"quoteVolume"`
		Time        string  `json:"time"`
	}
)

func (s *GetTickerService) Symbol(symbol string) *GetTickerService {
	s.TradingPairName = symbol
	return s
}

func (s *GetTickerService) Do(ctx context.Context, opts ...RequestOption) (res *TickerResponse, err error) {
	_endpoint := "/trading-pairs/%s/ticker"
	if s.TradingPairName != "" {
		_endpoint = fmt.Sprintf(_endpoint, s.TradingPairName)
	} else {
		_endpoint = "/trading-pairs/ETH-KRW/ticker"
	}

	r := &request{
		method:   "GET",
		endpoint: _endpoint,
		secType:  secTypeNone,
	}

	data, err := s.c.callAPI(ctx, r, opts...)
	if err != nil {
		return nil, err
	}
	// fmt.Println(string(data))
	// _data := string(data)
	// _data = strings.ReplaceAll(_data, `"stopPrice":undefined`, `"stopPrice":0`)
	// data = []byte(_data)

	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}
