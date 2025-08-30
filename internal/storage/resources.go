package storage

type RatesEnvelope struct {
	CurrencyCollection CurrencyCollection `json:"CurrencyCollection"`
	Revision           int                `json:"Revision"`
	ValidUntilDate     string             `json:"ValidUntilDate"`
	TenorCalcDate      string             `json:"TenorCalculationDate"`
	IsSuccessful       bool               `json:"IsSuccessful"`
	Error              any                `json:"Error"`
}

type PaymentTermsEnvelope struct {
	AgencyPaymentTerms       []AgencyPaymentTerm `json:"agencyPaymentTerms"`
	BaseForPaymentDueDateMap []EnumMap           `json:"baseForPaymentDueDateMap"`
	PaymentFrequencyMap      []EnumMap           `json:"paymentFrequencyMap"`
	ErrorMessage             string              `json:"errorMessage"`
}

type EnumMap struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type CurrencyCollection struct {
	Hedged []HedgedPair `json:"hedged"`
	Spot   []SpotPair   `json:"spot"`
}

type HedgedPair struct {
	From string  `json:"fromCurrency"`
	To   string  `json:"toCurrency"`
	Ten  []Tenor `json:"tenors"`
}

type SpotPair struct {
	From string  `json:"fromCurrency"`
	To   string  `json:"toCurrency"`
	Rate float64 `json:"rate"`
}
