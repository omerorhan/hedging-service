package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/omerorhan/hedging-service/internal/storage"
)

func TestChooseBaseDate(t *testing.T) {
	// Common test dates
	checkInDate := "2025-01-15"
	checkOutDate := "2025-01-20"
	bookingCreatedAt := time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC)
	cancellationDeadline := time.Date(2025, 1, 12, 18, 0, 0, 0, time.UTC)

	// Expected parsed dates
	expectedCheckIn := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	expectedCheckOut := time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name            string
		req             storage.GiveMeRateReq
		bpddCode        int
		bpddName        string
		expectedDate    time.Time
		expectedExplain string
		expectError     bool
	}{
		// CheckIn cases
		{
			name: "CheckIn - valid date",
			req: storage.GiveMeRateReq{
				CheckIn: checkInDate,
			},
			bpddCode:        1,
			bpddName:        CheckIn,
			expectedDate:    expectedCheckIn,
			expectedExplain: CheckIn,
			expectError:     false,
		},
		{
			name: "CheckIn - missing checkIn",
			req: storage.GiveMeRateReq{
				CheckIn: "",
			},
			bpddCode:    1,
			bpddName:    CheckIn,
			expectError: true,
		},
		{
			name: "CheckIn - invalid date format",
			req: storage.GiveMeRateReq{
				CheckIn: "invalid-date",
			},
			bpddCode:    1,
			bpddName:    CheckIn,
			expectError: true,
		},

		// Default cases
		{
			name: "Default - refundable booking",
			req: storage.GiveMeRateReq{
				CheckIn:          checkInDate,
				BookingCreatedAt: bookingCreatedAt,
				Nonrefundable:    false,
			},
			bpddCode:        2,
			bpddName:        Default,
			expectedDate:    expectedCheckIn,
			expectedExplain: Default,
			expectError:     false,
		},
		{
			name: "Default - nonrefundable booking",
			req: storage.GiveMeRateReq{
				CheckIn:          checkInDate,
				BookingCreatedAt: bookingCreatedAt,
				Nonrefundable:    true,
			},
			bpddCode:        2,
			bpddName:        Default,
			expectedDate:    bookingCreatedAt,
			expectedExplain: Default,
			expectError:     false,
		},
		{
			name: "Default - missing checkIn",
			req: storage.GiveMeRateReq{
				CheckIn: "",
			},
			bpddCode:    2,
			bpddName:    Default,
			expectError: true,
		},
		{
			name: "Default - nonrefundable with zero bookingCreatedAt",
			req: storage.GiveMeRateReq{
				CheckIn:       checkInDate,
				Nonrefundable: true,
			},
			bpddCode:    2,
			bpddName:    Default,
			expectError: true,
		},

		// CheckInAndNonRefundable cases
		{
			name: "CheckInAndNonRefundable - refundable",
			req: storage.GiveMeRateReq{
				CheckIn:          checkInDate,
				BookingCreatedAt: bookingCreatedAt,
				Nonrefundable:    false,
			},
			bpddCode:        3,
			bpddName:        CheckInAndNonRefundable,
			expectedDate:    expectedCheckIn,
			expectedExplain: CheckInAndNonRefundable,
			expectError:     false,
		},
		{
			name: "CheckInAndNonRefundable - nonrefundable",
			req: storage.GiveMeRateReq{
				CheckIn:          checkInDate,
				BookingCreatedAt: bookingCreatedAt,
				Nonrefundable:    true,
			},
			bpddCode:        3,
			bpddName:        CheckInAndNonRefundable,
			expectedDate:    bookingCreatedAt,
			expectedExplain: CheckInAndNonRefundable,
			expectError:     false,
		},

		// CheckOut cases
		{
			name: "CheckOut - valid date",
			req: storage.GiveMeRateReq{
				CheckOut: checkOutDate,
			},
			bpddCode:        4,
			bpddName:        CheckOut,
			expectedDate:    expectedCheckOut,
			expectedExplain: CheckOut,
			expectError:     false,
		},
		{
			name: "CheckOut - missing checkOut",
			req: storage.GiveMeRateReq{
				CheckOut: "",
			},
			bpddCode:    4,
			bpddName:    CheckOut,
			expectError: true,
		},

		// CheckOutAndNonRefundable cases
		{
			name: "CheckOutAndNonRefundable - refundable",
			req: storage.GiveMeRateReq{
				CheckOut:         checkOutDate,
				BookingCreatedAt: bookingCreatedAt,
				Nonrefundable:    false,
			},
			bpddCode:        5,
			bpddName:        CheckOutAndNonRefundable,
			expectedDate:    expectedCheckOut,
			expectedExplain: CheckOutAndNonRefundable,
			expectError:     false,
		},
		{
			name: "CheckOutAndNonRefundable - nonrefundable",
			req: storage.GiveMeRateReq{
				CheckOut:         checkOutDate,
				BookingCreatedAt: bookingCreatedAt,
				Nonrefundable:    true,
			},
			bpddCode:        5,
			bpddName:        CheckOutAndNonRefundable,
			expectedDate:    bookingCreatedAt,
			expectedExplain: CheckOutAndNonRefundable,
			expectError:     false,
		},

		// CreationDate cases
		{
			name: "CreationDate - valid",
			req: storage.GiveMeRateReq{
				BookingCreatedAt: bookingCreatedAt,
			},
			bpddCode:        6,
			bpddName:        CreationDate,
			expectedDate:    bookingCreatedAt,
			expectedExplain: CreationDate,
			expectError:     false,
		},
		{
			name: "CreationDate - zero time",
			req: storage.GiveMeRateReq{
				BookingCreatedAt: time.Time{},
			},
			bpddCode:    6,
			bpddName:    CreationDate,
			expectError: true,
		},

		// CancellationDeadline cases
		{
			name: "CancellationDeadline - valid",
			req: storage.GiveMeRateReq{
				CancellationDeadline: cancellationDeadline,
			},
			bpddCode:        7,
			bpddName:        CancellationDeadline,
			expectedDate:    cancellationDeadline,
			expectedExplain: CancellationDeadline,
			expectError:     false,
		},
		{
			name: "CancellationDeadline - zero time",
			req: storage.GiveMeRateReq{
				CancellationDeadline: time.Time{},
			},
			bpddCode:    7,
			bpddName:    CancellationDeadline,
			expectError: true,
		},

		// CancellationDeadlineAndNonRefundable cases
		{
			name: "CancellationDeadlineAndNonRefundable - refundable",
			req: storage.GiveMeRateReq{
				CancellationDeadline: cancellationDeadline,
				BookingCreatedAt:     bookingCreatedAt,
				Nonrefundable:        false,
			},
			bpddCode:        8,
			bpddName:        CancellationDeadlineAndNonRefundable,
			expectedDate:    cancellationDeadline,
			expectedExplain: CancellationDeadlineAndNonRefundable,
			expectError:     false,
		},
		{
			name: "CancellationDeadlineAndNonRefundable - nonrefundable",
			req: storage.GiveMeRateReq{
				CancellationDeadline: cancellationDeadline,
				BookingCreatedAt:     bookingCreatedAt,
				Nonrefundable:        true,
			},
			bpddCode:        8,
			bpddName:        CancellationDeadlineAndNonRefundable,
			expectedDate:    bookingCreatedAt,
			expectedExplain: CancellationDeadlineAndNonRefundable,
			expectError:     false,
		},
		{
			name: "CancellationDeadlineAndNonRefundable - nonrefundable with zero bookingCreatedAt",
			req: storage.GiveMeRateReq{
				CancellationDeadline: cancellationDeadline,
				Nonrefundable:        true,
			},
			bpddCode:    8,
			bpddName:    CancellationDeadlineAndNonRefundable,
			expectError: true,
		},

		// Default (unknown BPDD) cases
		{
			name: "Unknown BPDD - with valid checkIn",
			req: storage.GiveMeRateReq{
				CheckIn:          checkInDate,
				BookingCreatedAt: bookingCreatedAt,
			},
			bpddCode:        999,
			bpddName:        "UnknownBPDD",
			expectedDate:    expectedCheckIn,
			expectedExplain: "Default(BPDD=999)->CheckIn",
			expectError:     false,
		},
		{
			name: "Unknown BPDD - with invalid checkIn, fallback to createdAt",
			req: storage.GiveMeRateReq{
				CheckIn:          "invalid-date",
				BookingCreatedAt: bookingCreatedAt,
			},
			bpddCode:        999,
			bpddName:        "UnknownBPDD",
			expectedDate:    bookingCreatedAt,
			expectedExplain: "Default(BPDD=999)->CreatedAt",
			expectError:     false,
		},
		{
			name: "Unknown BPDD - no checkIn, fallback to createdAt",
			req: storage.GiveMeRateReq{
				CheckIn:          "",
				BookingCreatedAt: bookingCreatedAt,
			},
			bpddCode:        999,
			bpddName:        "UnknownBPDD",
			expectedDate:    bookingCreatedAt,
			expectedExplain: "Default(BPDD=999)->CreatedAt",
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultDate, resultExplain, err := chooseBaseDate(tt.req, tt.bpddCode, tt.bpddName)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if !resultDate.Equal(tt.expectedDate) {
				t.Errorf("Expected date %v, got %v", tt.expectedDate, resultDate)
			}

			if resultExplain != tt.expectedExplain {
				t.Errorf("Expected explain %q, got %q", tt.expectedExplain, resultExplain)
			}
		})
	}
}

func TestDaysDueFromFrequency(t *testing.T) {
	// Test date: January 15, 2025 (Wednesday)
	baseDate := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	// January 2025 has 31 days
	// Weekly boundaries: 7, 15, 23, 31
	// BiWeekly boundaries: mid=16, end=31
	// EOM: January 31, 2025

	tests := []struct {
		name            string
		base            time.Time
		freqCode        int
		freqName        string
		daysAfter       int
		expectedDays    int
		expectedExplain string
	}{
		// Zero/invalid base date
		{
			name:            "Zero base date",
			base:            time.Time{},
			freqCode:        1,
			freqName:        Monthly,
			daysAfter:       0,
			expectedDays:    0,
			expectedExplain: "base date is zero; defaulting to 0 days",
		},

		// Monthly cases
		{
			name:            "Monthly - middle of month",
			base:            baseDate,
			freqCode:        1,
			freqName:        Monthly,
			daysAfter:       0,
			expectedDays:    16, // Jan 31 - Jan 15 = 16 days
			expectedExplain: "Monthly: EOM(2025-01-31)-%s + 0d",
		},
		{
			name:            "Monthly - with daysAfter",
			base:            baseDate,
			freqCode:        1,
			freqName:        Monthly,
			daysAfter:       5,
			expectedDays:    21, // Jan 31 - Jan 15 + 5 = 21 days
			expectedExplain: "Monthly: EOM(2025-01-31)-%s + 5d",
		},
		{
			name:            "Monthly - end of month",
			base:            time.Date(2025, 1, 31, 12, 0, 0, 0, time.UTC),
			freqCode:        1,
			freqName:        Monthly,
			daysAfter:       0,
			expectedDays:    0, // Same day as EOM
			expectedExplain: "Monthly: EOM(2025-01-31)-%s + 0d",
		},
		{
			name:            "Monthly - empty freqName defaults to Monthly",
			base:            baseDate,
			freqCode:        1,
			freqName:        "",
			daysAfter:       0,
			expectedDays:    16,
			expectedExplain: "Monthly: EOM(2025-01-31)-%s + 0d",
		},

		// Weekly cases
		{
			name:            "Weekly - before first boundary (day 7)",
			base:            time.Date(2025, 1, 5, 12, 0, 0, 0, time.UTC),
			freqCode:        2,
			freqName:        Weekly,
			daysAfter:       0,
			expectedDays:    1, // Jan 7 00:00 - Jan 5 12:00 = 1.5 days -> truncated to 1
			expectedExplain: "Weekly (quarters): boundary(2025-01-07)-%s + 0d",
		},
		{
			name:            "Weekly - on boundary day 15",
			base:            baseDate, // Jan 15
			freqCode:        2,
			freqName:        Weekly,
			daysAfter:       0,
			expectedDays:    0, // Same day as boundary
			expectedExplain: "Weekly (quarters): boundary(2025-01-15)-%s + 0d",
		},
		{
			name:            "Weekly - between boundaries (day 16-22)",
			base:            time.Date(2025, 1, 20, 12, 0, 0, 0, time.UTC),
			freqCode:        2,
			freqName:        Weekly,
			daysAfter:       0,
			expectedDays:    2, // Jan 23 00:00 - Jan 20 12:00 = 2.5 days -> truncated to 2
			expectedExplain: "Weekly (quarters): boundary(2025-01-23)-%s + 0d",
		},
		{
			name:            "Weekly - after last boundary (day 24-31)",
			base:            time.Date(2025, 1, 28, 12, 0, 0, 0, time.UTC),
			freqCode:        2,
			freqName:        Weekly,
			daysAfter:       0,
			expectedDays:    2, // Jan 31 00:00 - Jan 28 12:00 = 2.5 days -> truncated to 2
			expectedExplain: "Weekly (quarters): boundary(2025-01-31)-%s + 0d",
		},

		// BiWeekly cases
		{
			name:            "BiWeekly - before mid month",
			base:            time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC),
			freqCode:        3,
			freqName:        BiWeekly,
			daysAfter:       0,
			expectedDays:    5, // Jan 16 00:00 - Jan 10 12:00 = 5.5 days -> truncated to 5
			expectedExplain: "BiWeekly (mid/end): boundary(2025-01-16)-%s + 0d",
		},
		{
			name:            "BiWeekly - after mid month",
			base:            time.Date(2025, 1, 20, 12, 0, 0, 0, time.UTC),
			freqCode:        3,
			freqName:        BiWeekly,
			daysAfter:       0,
			expectedDays:    10, // Jan 31 00:00 - Jan 20 12:00 = 10.5 days -> truncated to 10
			expectedExplain: "BiWeekly (mid/end): boundary(2025-01-31)-%s + 0d",
		},

		// Daily cases
		{
			name:            "Daily - no calculation needed",
			base:            baseDate,
			freqCode:        4,
			freqName:        Daily,
			daysAfter:       3,
			expectedDays:    3, // Only daysAfter
			expectedExplain: "Daily: base(%s) + 3d",
		},
		{
			name:            "Daily - zero daysAfter",
			base:            baseDate,
			freqCode:        4,
			freqName:        Daily,
			daysAfter:       0,
			expectedDays:    0,
			expectedExplain: "Daily: base(%s) + 0d",
		},

		// SpecificDaysOfWeek cases
		{
			name:            "SpecificDaysOfWeek - next same weekday",
			base:            baseDate, // Wednesday
			freqCode:        5,
			freqName:        SpecificDaysOfWeek,
			daysAfter:       2,
			expectedDays:    9, // 7 days + 2 daysAfter
			expectedExplain: "SpecificDaysOfWeek: next %s in 7d + 2d",
		},

		// Default/Unknown frequency cases
		{
			name:            "Unknown frequency - defaults to Monthly",
			base:            baseDate,
			freqCode:        999,
			freqName:        "UnknownFreq",
			daysAfter:       0,
			expectedDays:    16, // Same as Monthly
			expectedExplain: "Default(Monthly for \"UnknownFreq\"/999): EOM(2025-01-31)-%s + 0d",
		},

		// Negative daysAfter clamping
		{
			name:            "Negative result clamped to zero",
			base:            time.Date(2025, 1, 31, 12, 0, 0, 0, time.UTC), // EOM
			freqCode:        1,
			freqName:        Monthly,
			daysAfter:       -5, // Would result in -5 but clamped to 0
			expectedDays:    0,
			expectedExplain: "Monthly: EOM(2025-01-31)-%s + -5d",
		},

		// February test (shorter month)
		{
			name:            "February Monthly test",
			base:            time.Date(2025, 2, 15, 12, 0, 0, 0, time.UTC),
			freqCode:        1,
			freqName:        Monthly,
			daysAfter:       0,
			expectedDays:    13, // Feb 28 - Feb 15 = 13 days (2025 is not leap year)
			expectedExplain: "Monthly: EOM(2025-02-28)-%s + 0d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultDays, resultExplain := daysDueFromFrequency(tt.base, tt.freqCode, tt.freqName, tt.daysAfter)

			if resultDays != tt.expectedDays {
				t.Errorf("Expected days %d, got %d", tt.expectedDays, resultDays)
			}

			// For explain strings with date formatting, we need to format the expected string
			var expectedExplain string
			if tt.name == "Zero base date" {
				expectedExplain = tt.expectedExplain
			} else if strings.Contains(tt.expectedExplain, "%s") {
				if tt.freqName == SpecificDaysOfWeek {
					expectedExplain = fmt.Sprintf(tt.expectedExplain, tt.base.Weekday().String())
				} else {
					expectedExplain = fmt.Sprintf(tt.expectedExplain, tt.base.Format("2006-01-02"))
				}
			} else {
				expectedExplain = tt.expectedExplain
			}

			if resultExplain != expectedExplain {
				t.Errorf("Expected explain %q, got %q", expectedExplain, resultExplain)
			}
		})
	}
}

// Test helper functions used by the business logic
func TestHelperFunctions(t *testing.T) {
	t.Run("mustParseISODate", func(t *testing.T) {
		tests := []struct {
			input    string
			expected time.Time
		}{
			{"", time.Time{}},
			{"2025-01-15", time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)},
			{"2025-01-15T14:30:00Z", time.Date(2025, 1, 15, 14, 30, 0, 0, time.UTC)},
			{"invalid-date", time.Time{}},
		}

		for _, tt := range tests {
			result := mustParseISODate(tt.input)
			if !result.Equal(tt.expected) {
				t.Errorf("mustParseISODate(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		}
	})

	t.Run("clampNonNegative", func(t *testing.T) {
		tests := []struct {
			input    int
			expected int
		}{
			{-5, 0},
			{0, 0},
			{5, 5},
		}

		for _, tt := range tests {
			result := clampNonNegative(tt.input)
			if result != tt.expected {
				t.Errorf("clampNonNegative(%d) = %d, expected %d", tt.input, result, tt.expected)
			}
		}
	})

	t.Run("lastDayOfMonth", func(t *testing.T) {
		tests := []struct {
			input    time.Time
			expected int
		}{
			{time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC), 31}, // January
			{time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC), 28}, // February (non-leap)
			{time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC), 29}, // February (leap)
			{time.Date(2025, 4, 15, 0, 0, 0, 0, time.UTC), 30}, // April
		}

		for _, tt := range tests {
			result := lastDayOfMonth(tt.input)
			if result != tt.expected {
				t.Errorf("lastDayOfMonth(%v) = %d, expected %d", tt.input, result, tt.expected)
			}
		}
	})

	t.Run("midDayOfMonth", func(t *testing.T) {
		tests := []struct {
			input    time.Time
			expected int
		}{
			{time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC), 16}, // 31/2 = 15.5 -> ceil = 16
			{time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC), 14}, // 28/2 = 14 -> ceil = 14
			{time.Date(2025, 4, 15, 0, 0, 0, 0, time.UTC), 15}, // 30/2 = 15 -> ceil = 15
		}

		for _, tt := range tests {
			result := midDayOfMonth(tt.input)
			if result != tt.expected {
				t.Errorf("midDayOfMonth(%v) = %d, expected %d", tt.input, result, tt.expected)
			}
		}
	})

	t.Run("endOfMonth", func(t *testing.T) {
		input := time.Date(2025, 1, 15, 12, 30, 45, 0, time.UTC)
		expected := time.Date(2025, 1, 31, 23, 59, 59, 999999999, time.UTC)

		result := endOfMonth(input)
		if !result.Equal(expected) {
			t.Errorf("endOfMonth(%v) = %v, expected %v", input, result, expected)
		}
	})
}
