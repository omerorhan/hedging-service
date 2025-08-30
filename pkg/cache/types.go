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
