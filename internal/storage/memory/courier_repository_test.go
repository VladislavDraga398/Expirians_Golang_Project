package memory_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/vladislavdragonenkov/oms/internal/domain"
	"github.com/vladislavdragonenkov/oms/internal/storage/memory"
)

func sampleCourier(id, phone string, vehicle domain.VehicleType, createdAt time.Time) domain.Courier {
	return domain.Courier{
		ID:          id,
		Phone:       phone,
		FirstName:   "Ivan",
		LastName:    "Petrov",
		VehicleType: vehicle,
		IsActive:    true,
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}
}

func TestCourierRepository_CreateGetSaveAndGetByPhone(t *testing.T) {
	repo := memory.NewCourierRepository()
	now := time.Now().UTC()

	courier := sampleCourier("courier-1", "+79990000001", domain.VehicleTypeCar, now)
	if err := repo.Create(courier); err != nil {
		t.Fatalf("create courier: %v", err)
	}

	got, err := repo.Get(courier.ID)
	if err != nil {
		t.Fatalf("get courier: %v", err)
	}
	if got.Phone != courier.Phone {
		t.Fatalf("unexpected phone: got=%s want=%s", got.Phone, courier.Phone)
	}

	gotByPhone, err := repo.GetByPhone(courier.Phone)
	if err != nil {
		t.Fatalf("get by phone: %v", err)
	}
	if gotByPhone.ID != courier.ID {
		t.Fatalf("unexpected courier by phone: got=%s want=%s", gotByPhone.ID, courier.ID)
	}

	got.FirstName = "Petr"
	if err := repo.Save(got); err != nil {
		t.Fatalf("save courier: %v", err)
	}

	updated, err := repo.Get(courier.ID)
	if err != nil {
		t.Fatalf("get updated courier: %v", err)
	}
	if updated.FirstName != "Petr" {
		t.Fatalf("unexpected first name: got=%s want=Petr", updated.FirstName)
	}
}

func TestCourierRepository_PhoneNormalization(t *testing.T) {
	repo := memory.NewCourierRepository()
	now := time.Now().UTC()

	courier := sampleCourier("courier-phone", "+7 (999) 000-00-55", domain.VehicleTypeCar, now)
	if err := repo.Create(courier); err != nil {
		t.Fatalf("create courier: %v", err)
	}

	got, err := repo.Get("courier-phone")
	if err != nil {
		t.Fatalf("get courier: %v", err)
	}
	if got.Phone != "+79990000055" {
		t.Fatalf("expected normalized phone +79990000055, got %s", got.Phone)
	}

	if _, err := repo.GetByPhone("+79990000055"); err != nil {
		t.Fatalf("get by normalized phone: %v", err)
	}
}

func TestCourierRepository_ZonesAndListByZone(t *testing.T) {
	repo := memory.NewCourierRepository()
	now := time.Now().UTC()

	car := sampleCourier("courier-car", "+79990000002", domain.VehicleTypeCar, now)
	scooter := sampleCourier("courier-scooter", "+79990000003", domain.VehicleTypeScooter, now.Add(time.Minute))
	if err := repo.Create(car); err != nil {
		t.Fatalf("create car courier: %v", err)
	}
	if err := repo.Create(scooter); err != nil {
		t.Fatalf("create scooter courier: %v", err)
	}

	if err := repo.ReplaceZones(car.ID, []domain.CourierZone{
		{ZoneID: "msk-cao-arbat", IsPrimary: true},
		{ZoneID: "msk-cao-tverskoy", IsPrimary: false},
	}); err != nil {
		t.Fatalf("replace zones for car: %v", err)
	}

	if err := repo.ReplaceZones(scooter.ID, []domain.CourierZone{
		{ZoneID: "msk-cao-arbat", IsPrimary: true},
		{ZoneID: "msk-cao-tverskoy", IsPrimary: false},
	}); !errors.Is(err, domain.ErrCourierZoneLimitExceeded) {
		t.Fatalf("expected ErrCourierZoneLimitExceeded, got %v", err)
	}

	if err := repo.ReplaceZones(scooter.ID, []domain.CourierZone{
		{ZoneID: "msk-cao-arbat", IsPrimary: true},
	}); err != nil {
		t.Fatalf("replace zones for scooter: %v", err)
	}

	inArbat, err := repo.ListByZone("msk-cao-arbat", 10)
	if err != nil {
		t.Fatalf("list by zone: %v", err)
	}
	if len(inArbat) != 2 {
		t.Fatalf("expected 2 couriers in zone, got %d", len(inArbat))
	}

	carZones, err := repo.ListZones(car.ID)
	if err != nil {
		t.Fatalf("list zones for car: %v", err)
	}
	if len(carZones) != 2 {
		t.Fatalf("expected 2 zones, got %d", len(carZones))
	}
	if !carZones[0].IsPrimary {
		t.Fatal("expected first zone to be primary")
	}
}

func TestCourierRepository_ZoneValidationErrors(t *testing.T) {
	repo := memory.NewCourierRepository()
	now := time.Now().UTC()
	courier := sampleCourier("courier-zones", "+79990000006", domain.VehicleTypeCar, now)
	if err := repo.Create(courier); err != nil {
		t.Fatalf("create courier: %v", err)
	}

	if err := repo.ReplaceZones(courier.ID, nil); !errors.Is(err, domain.ErrCourierZonesRequired) {
		t.Fatalf("expected ErrCourierZonesRequired, got %v", err)
	}

	if err := repo.ReplaceZones(courier.ID, []domain.CourierZone{
		{ZoneID: "msk-cao-arbat", IsPrimary: true},
		{ZoneID: "msk-cao-arbat", IsPrimary: false},
	}); !errors.Is(err, domain.ErrCourierZoneDuplicate) {
		t.Fatalf("expected ErrCourierZoneDuplicate, got %v", err)
	}

	if err := repo.ReplaceZones(courier.ID, []domain.CourierZone{
		{ZoneID: "msk-cao-arbat", IsPrimary: true},
		{ZoneID: "msk-cao-tverskoy", IsPrimary: true},
	}); !errors.Is(err, domain.ErrCourierPrimaryZoneConflict) {
		t.Fatalf("expected ErrCourierPrimaryZoneConflict, got %v", err)
	}
}

func TestCourierRepository_ZoneCapacityLimit(t *testing.T) {
	repo := memory.NewCourierRepository()
	now := time.Now().UTC()
	zoneID := "msk-cao-arbat"

	for i := 0; i < domain.MaxCouriersPerZoneDefault; i++ {
		id := fmt.Sprintf("courier-%d", i)
		phone := fmt.Sprintf("+7999001%04d", i)
		courier := sampleCourier(id, phone, domain.VehicleTypeScooter, now.Add(time.Duration(i)*time.Second))
		if err := repo.Create(courier); err != nil {
			t.Fatalf("create courier %s: %v", id, err)
		}
		if err := repo.ReplaceZones(id, []domain.CourierZone{
			{ZoneID: zoneID, IsPrimary: true},
		}); err != nil {
			t.Fatalf("assign zone for %s: %v", id, err)
		}
	}

	overflow := sampleCourier("courier-overflow", "+79990019999", domain.VehicleTypeScooter, now.Add(time.Hour))
	if err := repo.Create(overflow); err != nil {
		t.Fatalf("create overflow courier: %v", err)
	}
	if err := repo.ReplaceZones(overflow.ID, []domain.CourierZone{
		{ZoneID: zoneID, IsPrimary: true},
	}); !errors.Is(err, domain.ErrCourierZoneCapacityExceeded) {
		t.Fatalf("expected ErrCourierZoneCapacityExceeded, got %v", err)
	}
}

func TestCourierRepository_Slots(t *testing.T) {
	repo := memory.NewCourierRepository()
	now := time.Now().UTC().Round(time.Second)

	courier := sampleCourier("courier-slot", "+79990000004", domain.VehicleTypeBike, now)
	if err := repo.Create(courier); err != nil {
		t.Fatalf("create courier: %v", err)
	}

	slot1 := domain.CourierSlot{
		ID:            "slot-1",
		CourierID:     courier.ID,
		SlotStart:     now.Add(9 * time.Hour),
		SlotEnd:       now.Add(13 * time.Hour),
		DurationHours: 4,
	}
	if err := repo.CreateSlot(slot1); err != nil {
		t.Fatalf("create slot1: %v", err)
	}

	slotConflict := domain.CourierSlot{
		ID:            "slot-2",
		CourierID:     courier.ID,
		SlotStart:     now.Add(12 * time.Hour),
		SlotEnd:       now.Add(20 * time.Hour),
		DurationHours: 8,
	}
	if err := repo.CreateSlot(slotConflict); !errors.Is(err, domain.ErrCourierSlotConflict) {
		t.Fatalf("expected ErrCourierSlotConflict, got %v", err)
	}

	slot3 := domain.CourierSlot{
		ID:            "slot-3",
		CourierID:     courier.ID,
		SlotStart:     now.Add(13 * time.Hour),
		SlotEnd:       now.Add(17 * time.Hour),
		DurationHours: 4,
	}
	if err := repo.CreateSlot(slot3); err != nil {
		t.Fatalf("create slot3: %v", err)
	}

	slots, err := repo.ListSlots(courier.ID, now.Add(8*time.Hour), now.Add(18*time.Hour))
	if err != nil {
		t.Fatalf("list slots: %v", err)
	}
	if len(slots) != 2 {
		t.Fatalf("expected 2 slots, got %d", len(slots))
	}
	if slots[0].ID != "slot-1" || slots[1].ID != "slot-3" {
		t.Fatalf("unexpected slots order: %+v", slots)
	}
}
