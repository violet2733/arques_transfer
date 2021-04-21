package gopax

import (
	"context"
	"encoding/json"
	"strings"
)

type (
	Balance struct {
		Asset             string  `json:"asset"`
		Avaliable         float32 `json:"avail"`
		Hold              float32 `json:"hold"`
		PendingWithdrawal float32 `json:"pendingWithdrawal"`
		LastUpdatedAt     string  `json:"lastUpdatedAt"`
	}

	GetBalancesService struct {
		c *Client
	}

	GetBalanceService struct {
		c      *Client
		symbol string
	}
)

func (s *GetBalancesService) Do(ctx context.Context, opts ...RequestOption) (res []*Balance, err error) {
	r := &request{
		method:     "GET",
		endpoint:   "/balances",
		secType:    secTypeSigned,
		recvWindow: -1,
	}

	data, err := s.c.callAPI(ctx, r, opts...)
	if err != nil {
		return nil, err
	}

	_data := string(data)
	_data = strings.ReplaceAll(_data, `"lastUpdatedAt":0`, `"lastUpdatedAt":"0"`)
	data = []byte(_data)

	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *GetBalanceService) Symbol(symbol string) *GetBalanceService {
	s.symbol = symbol
	return s
}

func (s *GetBalanceService) Do(ctx context.Context, opts ...RequestOption) (res *Balance, err error) {
	r := &request{
		method:     "GET",
		endpoint:   "/balances/" + s.symbol,
		secType:    secTypeSigned,
		recvWindow: -1,
	}

	data, err := s.c.callAPI(ctx, r, opts...)
	if err != nil {
		return nil, err
	}

	_data := string(data)
	_data = strings.ReplaceAll(_data, `"lastUpdatedAt":0`, `"lastUpdatedAt":"0"`)
	data = []byte(_data)

	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}
