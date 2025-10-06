package domain

import (
	"testing"
	"time"
)

func TestPayment_Validate(t *testing.T) {
	tests := []struct {
		name    string
		payment *Payment
		wantErr bool
		errCount int
	}{
		{
			name: "valid payment",
			payment: &Payment{
				OrderID:     "order-123",
				Provider:    "stripe",
				AmountMinor: 1000,
				Status:      PaymentStatusPending,
				CreatedAt:   time.Now(),
			},
			wantErr: false,
			errCount: 0,
		},
		{
			name: "missing order ID",
			payment: &Payment{
				Provider:    "stripe",
				AmountMinor: 1000,
			},
			wantErr: true,
			errCount: 1,
		},
		{
			name: "missing provider",
			payment: &Payment{
				OrderID:     "order-123",
				AmountMinor: 1000,
			},
			wantErr: true,
			errCount: 1,
		},
		{
			name: "negative amount",
			payment: &Payment{
				OrderID:     "order-123",
				Provider:    "stripe",
				AmountMinor: -100,
			},
			wantErr: true,
			errCount: 1,
		},
		{
			name: "multiple errors",
			payment: &Payment{
				AmountMinor: -100,
			},
			wantErr: true,
			errCount: 1, // switch stops at first case
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.payment.Validate()
			
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

func TestPayment_ValidateZeroAmount(t *testing.T) {
	payment := &Payment{
		OrderID:     "order-123",
		Provider:    "stripe",
		AmountMinor: 0, // zero is valid
	}
	
	errs := payment.Validate()
	if len(errs) > 0 {
		t.Errorf("zero amount should be valid, got errors: %v", errs)
	}
}
