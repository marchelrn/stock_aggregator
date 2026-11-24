package dto

// Decimal fields contain printed amounts in IDR with a period as decimal separator.
// Empty fields (including JSON null from the extractor) need owner review.
type ReviewFill struct {
	Ref    string `json:"ref"`
	Ticker string `json:"ticker"`
	Side   string `json:"side"`
	Lots   string `json:"lots"`
	Shares string `json:"shares"`
	Price  string `json:"price"`
	Gross  string `json:"gross"`
}

type ReviewFee struct {
	Label string `json:"label"`
	Buy   string `json:"buy"`
	Sell  string `json:"sell"`
}

type TradeDraft struct {
	Date      string       `json:"date"`
	Rows      []ReviewFill `json:"rows"`
	Fees      []ReviewFee  `json:"fees"`
	BuyGross  string       `json:"buy_gross"`
	SellGross string       `json:"sell_gross"`
	BuyNet    string       `json:"buy_net"`
	SellNet   string       `json:"sell_net"`
}
