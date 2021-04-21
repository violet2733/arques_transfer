package model

import (
	"sync"
)

type (
	TransferWorkInfo struct {
		GimchiPremium float64          `json:"gimchiPremium"`
		WorkPrice     int              `json:"workPrice"`
		WorkSymbol    string           `json:"workSymbol"`
		ThreadPrice   int              `json:"threadPrice"`
		EqualParts    int              `json:"equalParts"`
		ZeroStep      *ZeroStepWorks   `json:"-"`
		FirstStep     *FirstStepWorks  `json:"-"`
		SecondStep    *SecondStepWorks `json:"-"`
	}

	ZeroStepWorks struct {
		CheckRate float64
		FindRate  float64
		CreatedAt int64
		UpdatedAt int64
	}

	FirstStepWorks struct {
		WorkLength         int
		FinishLength       int
		GoPaxWorks         []*OrderWork
		BinanceFutureWorks []*OrderWork
		CreatedAt          int64
		UpdatedAt          int64
		WG                 *sync.WaitGroup
	}

	SecondStepWorks struct {
		WorkLength         int
		FinishLength       int
		BinanceSpotWorks   []*OrderWork
		BinanceFutureWorks []*OrderWork
		CreatedAt          int64
		UpdatedAt          int64
		WG                 *sync.WaitGroup
	}

	OrderWork struct {
		OrderId     int64
		OrderPrice  float64
		OrderAmount float64
		IsFinished  bool
		CreatedAt   int64
		UpdatedAt   int64
	}
)
