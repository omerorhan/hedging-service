package cache

import (
	"time"
)

type GiveMeRateResp struct {
	From         string    `json:"FromCurrency"`
	To           string    `json:"ToCurrency"`
	Rate         float64   `json:"ExchangeRate"`
	IsRefundable bool      `json:"IsRefundable"`
	RevisionId   int       `json:"RevisionId"`
	DueDate      time.Time `json:"DueDate"`
	ValidUntil   time.Time `json:"ValidUntil"`
	Type         string    `json:"Type"`
	Explain      string    `json:"Explain"`
}
type HedgeCalcReq struct {
	AgencyId             int       `json:"agencyId"`
	From                 string    `json:"from"`
	To                   string    `json:"to"`
	BookingCreatedAt     time.Time `json:"bookingCreatedAt"`
	CancellationDeadline time.Time `json:"cancellationDeadline"`
	CheckIn              string    `json:"checkIn"`
	CheckOut             string    `json:"checkOut"`
	Nonrefundable        bool      `json:"nonrefundable"`
}

// RateData combines all rate-related data into a single structure
type RateData struct {
	Pair           RateRec   `json:"pair"`
	ValidUntil     time.Time `json:"validUntil"`
	RevisionNumber int       `json:"revisionNumber"`
}

type RatesEnvelope struct {
	CurrencyCollection CurrencyCollection `json:"currencyCollection"`
	Revision           int                `json:"revision"`
	ValidUntilDate     string             `json:"validUntilDate"`
	TenorCalcDate      string             `json:"tenorCalculationDate"`
	IsSuccessful       bool               `json:"isSuccessful"`
	Error              any                `json:"error"`
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
