package model

type (
	CountryMoney struct {
		KRW float64
		USD float64
		EUR float64
		SGD float64
	}

	MoneyExchangeRate struct {
		Success   bool          `json:"success"`
		Timestamp int64         `json:"timestamp"`
		Base      string        `json:"base"`
		Date      string        `json:"date"`
		Countries *CountryMoney `json:"rates"`
	}
)
