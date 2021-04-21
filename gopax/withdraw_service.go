package gopax

import (
	"context"
	"encoding/json"
	"strings"
)

type (
	GetDepositWithdrawalStatusService struct {
		c *Client
	}

	DepositWithdrawalStatus struct {
		ID                 int64   `json:"id"`
		Asset              string  `json:"asset"`
		Type               string  `json:"type"`
		NetAmount          float64 `json:"netAmount"`
		FeeAmount          float64 `json:"feeAmount"`
		Status             string  `json:"status"`
		ReviewStartedAt    int64   `json:"reviewStartedAt"`
		CompletedAt        int64   `json:"completedAt"`
		TxId               string  `json:"txId"`
		SourceAddress      string  `json:"sourceAddress"`
		DestinationAddress string  `json:"destinationAddress"`
		DestinationMemoId  string  `json:"destinationMemoId"`
	}
)

func (s *GetDepositWithdrawalStatusService) Do(ctx context.Context, opts ...RequestOption) (res *[]DepositWithdrawalStatus, err error) {
	r := &request{
		method:     "GET",
		endpoint:   "/deposit-withdrawal-status",
		secType:    secTypeSigned,
		recvWindow: -1,
	}

	data, err := s.c.callAPI(ctx, r, opts...)
	if err != nil {
		return nil, err
	}

	_data := string(data)
	// fmt.Println(_data)
	_data = strings.ReplaceAll(_data, `"netAmount":null`, `"netAmount":0`)
	_data = strings.ReplaceAll(_data, `"feeAmount":null`, `"feeAmount":0`)
	_data = strings.ReplaceAll(_data, `"txId":null`, `"txId":"null"`)
	_data = strings.ReplaceAll(_data, `"sourceAddress":null`, `"sourceAddress":"null"`)
	_data = strings.ReplaceAll(_data, `"destinationAddress":null`, `"destinationAddress":"null"`)
	_data = strings.ReplaceAll(_data, `"destinationMemoId":null`, `"destinationMemoId":"null"`)
	data = []byte(_data)

	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

type (
	GetCryptoDepositAddressService struct {
		c *Client
	}

	GetCryptoWithdrawalAddressService struct {
		c *Client
	}

	Address struct {
		Asset     string `json:"asset"`
		Address   string `json:"address"`
		MemoID    string `json:"memoId"`
		Nickname  string `json:"nickname"`
		CreatedAt int64  `json:"createdAt"`
	}
)

func (s *GetCryptoDepositAddressService) Do(ctx context.Context, opts ...RequestOption) (res []*Address, err error) {
	r := &request{
		method:     "GET",
		endpoint:   "/crypto-deposit-addresses",
		secType:    secTypeSigned,
		recvWindow: -1,
	}

	data, err := s.c.callAPI(ctx, r, opts...)
	if err != nil {
		return nil, err
	}

	_data := string(data)
	// fmt.Println(_data)
	_data = strings.ReplaceAll(_data, `"memoId":null`, `"memoId":""`)
	data = []byte(_data)

	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *GetCryptoWithdrawalAddressService) Do(ctx context.Context, opts ...RequestOption) (res []*Address, err error) {
	r := &request{
		method:     "GET",
		endpoint:   "/crypto-withdrawal-addresses",
		secType:    secTypeSigned,
		recvWindow: -1,
	}

	data, err := s.c.callAPI(ctx, r, opts...)
	if err != nil {
		return nil, err
	}

	_data := string(data)
	// fmt.Println(_data)
	_data = strings.ReplaceAll(_data, `"memoId":null`, `"memoId":""`)
	data = []byte(_data)

	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}
