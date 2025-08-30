package storage

import (
	"time"
)

// RateData combines all rate-related data into a single structure
type RateData struct {
	Pair           RateRec
	ValidUntil     time.Time
	RevisionNumber int
}
