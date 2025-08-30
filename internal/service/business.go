package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/omerorhan/hedging-service/internal/storage"
)

func chooseBaseDate(req GiveMeRateReq, bpddCode int, bpddName string) (time.Time, string, error) {
	switch bpddName {
	case CheckIn:
		if req.CheckIn == "" {
			return time.Time{}, "", errors.New("checkIn required by BPDD=" + bpddName)
		}
		t := mustParseISODate(req.CheckIn)
		if t.IsZero() {
			return time.Time{}, "", errors.New("invalid checkIn format")
		}

		return t, bpddName, nil
	case Default, CheckInAndNonRefundable:
		if req.CheckIn == "" {
			return time.Time{}, "", errors.New("checkIn required by BPDD=" + bpddName)
		}
		t := mustParseISODate(req.CheckIn)
		if t.IsZero() {
			return time.Time{}, "", errors.New("invalid checkIn format")
		}
		// If nonrefundable check...
		if req.Nonrefundable {
			t = req.BookingCreatedAt.UTC()
			if t.IsZero() {
				return time.Time{}, "", errors.New("invalid bookingCreatedAt format")
			}
		}
		return t, bpddName, nil
	case CheckOut:
		if req.CheckOut == "" {
			return time.Time{}, "", errors.New("checkOut required by BPDD=" + bpddName)
		}
		t := mustParseISODate(req.CheckOut)
		if t.IsZero() {
			return time.Time{}, "", errors.New("invalid checkOut format")
		}
		return t, bpddName, nil
	case CheckOutAndNonRefundable:
		if req.CheckOut == "" {
			return time.Time{}, "", errors.New("checkOut required by BPDD=" + bpddName)
		}
		t := mustParseISODate(req.CheckOut)
		if t.IsZero() {
			return time.Time{}, "", errors.New("invalid checkOut format")
		}
		// If nonrefundable check...
		if req.Nonrefundable {
			t = req.BookingCreatedAt.UTC()
			if t.IsZero() {
				return time.Time{}, "", errors.New("invalid bookingCreatedAt format")
			}
		}
		return t, bpddName, nil
	case CreationDate:
		if req.BookingCreatedAt.IsZero() {
			return time.Time{}, "", errors.New("invalid bookingCreatedAt format")
		}
		return req.BookingCreatedAt.UTC(), bpddName, nil
	case CancellationDeadline:
		if req.CancellationDeadline.IsZero() {
			return time.Time{}, "", errors.New("invalid cancellationDeadline format")
		}
		return req.CancellationDeadline.UTC(), bpddName, nil
	case CancellationDeadlineAndNonRefundable:
		t := req.CancellationDeadline.UTC()
		if req.CancellationDeadline.IsZero() {
			return time.Time{}, "", errors.New("invalid cancellationDeadline format")
		}
		// If nonrefundable check...
		if req.Nonrefundable {
			t = req.BookingCreatedAt.UTC()
			if t.IsZero() {
				return time.Time{}, "", errors.New("invalid bookingCreatedAt format")
			}
		}
		return t, bpddName, nil
	default:
		if req.CheckIn != "" {
			t := mustParseISODate(req.CheckIn)
			if !t.IsZero() {
				return t, fmt.Sprintf("Default(BPDD=%d)->CheckIn", bpddCode), nil
			}
		}
		return req.BookingCreatedAt.UTC(), fmt.Sprintf("Default(BPDD=%d)->CreatedAt", bpddCode), nil
	}
}

func daysDueFromFrequency(base time.Time, freqCode int, freqName string, daysAfter int) (int, string) {
	if base.IsZero() {
		return 0, "base date is zero; defaulting to 0 days"
	}
	base = base.UTC()
	y, m, d := base.Date()
	loc := base.Location()
	switch freqName {
	case Monthly, "":
		eom := endOfMonth(base)
		diff := int(eom.Sub(time.Date(y, m, d, 0, 0, 0, 0, loc)).Hours() / 24)
		return clampNonNegative(diff + daysAfter), fmt.Sprintf("Monthly: EOM(%s)-%s + %dd", eom.Format("2006-01-02"), base.Format("2006-01-02"), daysAfter)
	case Weekly:
		// month-specific “quarter” cutoffs: 7, 15, 23, EOM
		last := lastDayOfMonth(base)
		cutoffs := []int{7, 15, 23, last}
		var dayBoundary int
		for _, c := range cutoffs {
			if d <= c {
				dayBoundary = c
				break
			}
		}
		if dayBoundary == 0 { // theoretically not needed
			dayBoundary = last
		}
		boundary := time.Date(y, m, dayBoundary, 0, 0, 0, 0, loc)
		diff := int(boundary.Sub(base).Hours() / 24)
		return clampNonNegative(diff + daysAfter), fmt.Sprintf("Weekly (quarters): boundary(%s)-%s + %dd", boundary.Format("2006-01-02"), base.Format("2006-01-02"), daysAfter)
	case BiWeekly:
		last := lastDayOfMonth(base)
		midDay := midDayOfMonth(base)

		var boundaryDay int
		if d <= midDay {
			boundaryDay = midDay
		} else {
			boundaryDay = last
		}
		boundary := time.Date(y, m, boundaryDay, 0, 0, 0, 0, loc)
		diff := int(boundary.Sub(base).Hours() / 24)
		return clampNonNegative(diff + daysAfter), fmt.Sprintf("BiWeekly (mid/end): boundary(%s)-%s + %dd", boundary.Format("2006-01-02"), base.Format("2006-01-02"), daysAfter)
	case Daily:
		// Case D in doc: Use base BPDD date, then add DaysAfterPaymentPeriod ONLY
		return clampNonNegative(daysAfter), fmt.Sprintf("Daily: base(%s) + %dd", base.Format("2006-01-02"), daysAfter)
	case SpecificDaysOfWeek:
		// E: Take base BPDD date's weekday; choose the NEXT same weekday, then add DaysAfter
		// target = base.Weekday(); delta strictly next (never 0)
		delta := 7 // next same weekday
		return clampNonNegative(delta + daysAfter), fmt.Sprintf("SpecificDaysOfWeek: next %s in %dd + %dd", base.Weekday().String(), delta, daysAfter)
	default:
		eom := endOfMonth(base)
		diff := int(eom.Sub(time.Date(y, m, d, 0, 0, 0, 0, loc)).Hours() / 24)
		return clampNonNegative(diff + daysAfter), fmt.Sprintf("Default(Monthly for %q/%d): EOM(%s)-%s + %dd", freqName, freqCode, eom.Format("2006-01-02"), base.Format("2006-01-02"), daysAfter)
	}
}

func selectTenor(tenors []storage.Tenor, days int) (*storage.Tenor, error) {
	if len(tenors) == 0 {
		return &storage.Tenor{}, errors.New("no tenors")
	}
	choice := tenors[len(tenors)-1]
	for _, t := range tenors {
		if t.Days >= days {
			choice = t
			break
		}
	}
	return &choice, nil
}
