package domain

import (
	"testing"
	"time"
)

func TestReservation_Validate(t *testing.T) {
	tests := []struct {
		name        string
		reservation *Reservation
		wantErr     bool
		errCount    int
	}{
		{
			name: "valid reservation",
			reservation: &Reservation{
				OrderID:   "order-123",
				SKU:       "SKU-001",
				Qty:       5,
				Status:    ReservationStatusReserved,
				CreatedAt: time.Now(),
			},
			wantErr:  false,
			errCount: 0,
		},
		{
			name: "missing order ID",
			reservation: &Reservation{
				SKU:    "SKU-001",
				Qty:    5,
				Status: ReservationStatusReserved,
			},
			wantErr:  true,
			errCount: 1,
		},
		{
			name: "missing SKU",
			reservation: &Reservation{
				OrderID: "order-123",
				Qty:     5,
				Status:  ReservationStatusReserved,
			},
			wantErr:  true,
			errCount: 1,
		},
		{
			name: "zero quantity",
			reservation: &Reservation{
				OrderID: "order-123",
				SKU:     "SKU-001",
				Qty:     0,
				Status:  ReservationStatusReserved,
			},
			wantErr:  true,
			errCount: 1,
		},
		{
			name: "negative quantity",
			reservation: &Reservation{
				OrderID: "order-123",
				SKU:     "SKU-001",
				Qty:     -5,
				Status:  ReservationStatusReserved,
			},
			wantErr:  true,
			errCount: 1,
		},
		{
			name: "multiple errors - missing order ID and SKU",
			reservation: &Reservation{
				Qty:    5,
				Status: ReservationStatusReserved,
			},
			wantErr:  true,
			errCount: 2,
		},
		{
			name: "all fields missing",
			reservation: &Reservation{
				Status: ReservationStatusReserved,
			},
			wantErr:  true,
			errCount: 3, // orderID, SKU, qty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.reservation.Validate()

			if tt.wantErr && len(errs) == 0 {
				t.Error("expected validation errors, got none")
			}

			if !tt.wantErr && len(errs) > 0 {
				t.Errorf("expected no errors, got %d: %v", len(errs), errs)
			}

			if tt.wantErr && len(errs) != tt.errCount {
				t.Errorf("expected %d errors, got %d: %v", tt.errCount, len(errs), errs)
			}
		})
	}
}

func TestReservation_ValidateStatuses(t *testing.T) {
	statuses := []ReservationStatus{
		ReservationStatusReserved,
		ReservationStatusReleased,
		ReservationStatusFailed,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			reservation := &Reservation{
				OrderID:   "order-123",
				SKU:       "SKU-001",
				Qty:       5,
				Status:    status,
				CreatedAt: time.Now(),
			}

			errs := reservation.Validate()
			if len(errs) > 0 {
				t.Errorf("valid reservation with status %s should not have errors, got: %v", status, errs)
			}
		})
	}
}
