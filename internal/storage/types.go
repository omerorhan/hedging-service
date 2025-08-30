package storage

import (
	"time"
)

// Data structures from source code for bulk loading
type PairKey string
type RateRec struct {
	Spot   *float64 `json:"spot"`
	Tenors []Tenor  `json:"tenors"`
}

type Tenor struct {
	Days int     `json:"days"`
	Rate float64 `json:"rate"`
}

type AgencyPaymentTerm struct {
	AgencyId              int `json:"agencyId"`
	BaseForPaymentDueDate int `json:"baseForPaymentDueDate"`
	PaymentFrequency      int `json:"paymentFrequency"`
	DaysAfter             int `json:"daysAfterPaymentPeriod"`
}

type TermsCacheData struct {
	ByAgency    map[int]AgencyPaymentTerm `json:"by_agency"`
	BpddNames   map[int]string            `json:"bpdd_names"`
	FreqNames   map[int]string            `json:"freq_names"`
	LastRefresh time.Time                 `json:"last_refresh"`
}

// Request/Response types
type GiveMeRateReq struct {
	AgencyId             int
	From                 string
	To                   string
	BookingCreatedAt     time.Time
	CancellationDeadline time.Time
	CheckIn              string
	CheckOut             string
	Nonrefundable        bool
}

type GiveMeRateResp struct {
	From         string
	To           string
	Rate         float64
	IsRefundable bool
	RevisionId   int
	DueDate      time.Time
	ValidUntil   time.Time
	Type         string
	Explain      string
}

// RateData combines all rate-related data into a single structure
type RateData struct {
	Pair           RateRec
	ValidUntil     time.Time
	RevisionNumber int
}

// Provider data structures for backup storage
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
