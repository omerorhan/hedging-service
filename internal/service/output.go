package service

import (
	"time"
)

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
