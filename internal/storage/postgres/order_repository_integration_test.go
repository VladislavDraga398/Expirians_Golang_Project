package postgres

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

func TestOrderRepository_PostgresCreateGetListAndSave(t *testing.T) {
	store := openPostgresStoreForIntegrationTest(t)
	repo := NewOrderRepository(store)

	now := time.Now().UTC().Round(time.Microsecond)
	order1 := sampleOrder("order-1", "customer-1", now.Add(-2*time.Minute))
	order2 := sampleOrder("order-2", "customer-1", now.Add(-time.Minute))

	if err := repo.Create(order1); err != nil {
		t.Fatalf("create order1: %v", err)
	}
	if err := repo.Create(order2); err != nil {
		t.Fatalf("create order2: %v", err)
	}

	got, err := repo.Get(order1.ID)
	if err != nil {
		t.Fatalf("get order1: %v", err)
	}
	if got.ID != order1.ID || got.CustomerID != order1.CustomerID || got.Status != order1.Status {
		t.Fatalf("unexpected order payload: %+v", got)
	}
	if len(got.Items) != len(order1.Items) {
		t.Fatalf("unexpected items count: got=%d want=%d", len(got.Items), len(order1.Items))
	}

	listed, err := repo.ListByCustomer("customer-1", 1)
	if err != nil {
		t.Fatalf("list by customer with limit: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != order2.ID {
		t.Fatalf("unexpected list result with limit: %+v", listed)
	}

	all, err := repo.ListByCustomer("customer-1", 0)
	if err != nil {
		t.Fatalf("list by customer without limit: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 orders, got %d", len(all))
	}

	got.Status = domain.OrderStatusPaid
	got.UpdatedAt = now.Add(time.Minute)
	if err := repo.Save(got); err != nil {
		t.Fatalf("save order: %v", err)
	}

	updated, err := repo.Get(order1.ID)
	if err != nil {
		t.Fatalf("get updated order: %v", err)
	}
	if updated.Status != domain.OrderStatusPaid {
		t.Fatalf("unexpected status after save: %s", updated.Status)
	}
	if updated.Version != got.Version+1 {
		t.Fatalf("unexpected version after save: got=%d want=%d", updated.Version, got.Version+1)
	}
}

func TestOrderRepository_PostgresErrors(t *testing.T) {
	store := openPostgresStoreForIntegrationTest(t)
	repo := NewOrderRepository(store)

	now := time.Now().UTC().Round(time.Microsecond)
	base := sampleOrder("order-errors", "customer-2", now)

	if _, err := repo.Get("missing-order"); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("expected ErrOrderNotFound, got %v", err)
	}

	if err := repo.Save(base); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("expected ErrOrderNotFound on save missing, got %v", err)
	}

	if err := repo.Create(base); err != nil {
		t.Fatalf("create base order: %v", err)
	}
	if err := repo.Create(base); !errors.Is(err, domain.ErrOrderVersionConflict) {
		t.Fatalf("expected ErrOrderVersionConflict on duplicate create, got %v", err)
	}

	stale := base
	stale.Status = domain.OrderStatusConfirmed
	stale.UpdatedAt = now.Add(time.Minute)
	stale.Version = 42
	if err := repo.Save(stale); !errors.Is(err, domain.ErrOrderVersionConflict) {
		t.Fatalf("expected ErrOrderVersionConflict on stale save, got %v", err)
	}
}

func TestIsUniqueViolation(t *testing.T) {
	if !isUniqueViolation(&pgconn.PgError{Code: "23505"}) {
		t.Fatal("expected unique violation for code 23505")
	}
	if isUniqueViolation(&pgconn.PgError{Code: "22001"}) {
		t.Fatal("unexpected unique violation for non-unique code")
	}
	if isUniqueViolation(errors.New("plain error")) {
		t.Fatal("plain error must not be unique violation")
	}
}

func sampleOrder(id, customerID string, createdAt time.Time) domain.Order {
	items := []domain.OrderItem{
		{
			ID:         id + "-item-1",
			SKU:        "SKU-1",
			Qty:        2,
			PriceMinor: 150,
			CreatedAt:  createdAt,
		},
	}

	return domain.Order{
		ID:          id,
		CustomerID:  customerID,
		Status:      domain.OrderStatusPending,
		Currency:    "USD",
		AmountMinor: 300,
		Items:       items,
		Version:     0,
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}
}
