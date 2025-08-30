package service

import (
	"time"
)

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
