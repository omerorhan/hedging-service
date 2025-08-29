package cache

import (
	"encoding/json"
	"time"
)

type GiveMeRateResp struct {
	Rate           float64 `json:"rate"`
	Type           string  `json:"type"`
	RevisionNumber int     `json:"revisionNumber"`
	Explain        string  `json:"explain"`
}

type BoolRequired struct {
	Value   bool `json:"value"`
	Present bool `json:"present"`
}

// UnmarshalJSON implements custom unmarshaling for BoolRequired
func (br *BoolRequired) UnmarshalJSON(data []byte) error {
	br.Present = true
	return json.Unmarshal(data, &br.Value)
}

type HedgeCalcReq struct {
	AgencyId             int          `json:"agencyId"`
	From                 string       `json:"from"`
	To                   string       `json:"to"`
	BookingCreatedAt     time.Time    `json:"bookingCreatedAt"`
	CancellationDeadline time.Time    `json:"cancellationDeadline"`
	CheckIn              string       `json:"checkIn"`
	CheckOut             string       `json:"checkOut"`
	Nonrefundable        BoolRequired `json:"nonrefundable"`
}
