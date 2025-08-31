package service

import (
	"github.com/omerorhan/hedging-service/internal/storage"
)

const CheckIn = "CheckIn"
const CheckInAndNonRefundable = "CheckInAndNonRefundable"
const CheckOut = "CheckOut"
const CheckOutAndNonRefundable = "CheckOutAndNonRefundable"
const CreationDate = "CreationDate"
const CancellationDeadline = "CancellationDeadline"
const CancellationDeadlineAndNonRefundable = "CancellationDeadlineAndNonRefundable"
const Default = "Default"
const Monthly = "Monthly"
const Weekly = "Weekly"
const BiWeekly = "BiWeekly"
const Daily = "Daily"
const SpecificDaysOfWeek = "SpecificDaysOfWeek"
const Spot = "Spot"
const Hedged = "Hedged"

// Data structures from your source code for API responses
// Use types from storage package for consistency
type RatesEnvelope = storage.RatesEnvelope
type PaymentTermsEnvelope = storage.PaymentTermsEnvelope
type CurrencyCollection = storage.CurrencyCollection
type HedgedPair = storage.HedgedPair
type SpotPair = storage.SpotPair
type GiveMeRateReq = storage.GiveMeRateReq
type GiveMeRateResp = storage.GiveMeRateResp
