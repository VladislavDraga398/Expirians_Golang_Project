package payment

import (
	"errors"
	"testing"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

func TestMockService(t *testing.T) {
	mock := NewMockService()
	if mock == nil {
		t.Fatal("expected non-nil mock")
	}

	status, err := mock.Pay("o-1", 100, "USD")
	if err != nil {
		t.Fatalf("unexpected pay error: %v", err)
	}
	if status != domain.PaymentStatusCaptured {
		t.Fatalf("unexpected pay status: %s", status)
	}

	refundStatus, err := mock.Refund("o-1", 100, "USD")
	if err != nil {
		t.Fatalf("unexpected refund error: %v", err)
	}
	if refundStatus != domain.PaymentStatusRefunded {
		t.Fatalf("unexpected refund status: %s", refundStatus)
	}

	mock.PayStatus = domain.PaymentStatusFailed
	mock.PayErr = errors.New("pay failed")
	mock.RefundStatus = domain.PaymentStatusFailed
	mock.RefundErr = errors.New("refund failed")

	if _, err := mock.Pay("o-2", 100, "USD"); err == nil {
		t.Fatal("expected pay error")
	}
	if _, err := mock.Refund("o-2", 100, "USD"); err == nil {
		t.Fatal("expected refund error")
	}

	if mock.PayCalls != 2 || mock.RefundCalls != 2 {
		t.Fatalf("unexpected call counters: pay=%d refund=%d", mock.PayCalls, mock.RefundCalls)
	}
}
