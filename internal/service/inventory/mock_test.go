package inventory

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

	items := []domain.OrderItem{{ID: "i-1", SKU: "SKU", Qty: 1, PriceMinor: 100}}
	if err := mock.Reserve("o-1", items); err != nil {
		t.Fatalf("unexpected reserve error: %v", err)
	}
	if err := mock.Release("o-1", items); err != nil {
		t.Fatalf("unexpected release error: %v", err)
	}
	if mock.ReserveCalls != 1 || mock.ReleaseCalls != 1 {
		t.Fatalf("unexpected call counters: reserve=%d release=%d", mock.ReserveCalls, mock.ReleaseCalls)
	}

	mock.ReserveErr = errors.New("reserve failed")
	mock.ReleaseErr = errors.New("release failed")
	if err := mock.Reserve("o-2", items); err == nil {
		t.Fatal("expected reserve error")
	}
	if err := mock.Release("o-2", items); err == nil {
		t.Fatal("expected release error")
	}
}
