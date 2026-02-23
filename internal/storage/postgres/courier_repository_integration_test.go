package postgres

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

func TestCourierRepository_PostgresCreateGetAndSave(t *testing.T) {
	store := openPostgresStoreForIntegrationTest(t)
	repo := NewCourierRepository(store)

	now := time.Now().UTC().Round(time.Second)
	courier := domain.Courier{
		ID:          "courier-1",
		Phone:       "+79990000001",
		FirstName:   "Ivan",
		LastName:    "Petrov",
		VehicleType: domain.VehicleTypeCar,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := repo.Create(courier); err != nil {
		t.Fatalf("create courier: %v", err)
	}

	got, err := repo.Get(courier.ID)
	if err != nil {
		t.Fatalf("get courier: %v", err)
	}
	if got.Phone != courier.Phone || got.VehicleType != domain.VehicleTypeCar {
		t.Fatalf("unexpected courier payload: %+v", got)
	}

	gotByPhone, err := repo.GetByPhone(courier.Phone)
	if err != nil {
		t.Fatalf("get by phone: %v", err)
	}
	if gotByPhone.ID != courier.ID {
		t.Fatalf("unexpected courier by phone: got=%s want=%s", gotByPhone.ID, courier.ID)
	}

	got.LastName = "Sidorov"
	got.UpdatedAt = now.Add(time.Minute)
	if err := repo.Save(got); err != nil {
		t.Fatalf("save courier: %v", err)
	}

	updated, err := repo.Get(courier.ID)
	if err != nil {
		t.Fatalf("get updated courier: %v", err)
	}
	if updated.LastName != "Sidorov" {
		t.Fatalf("unexpected last name after save: got=%s", updated.LastName)
	}
}

func TestCourierRepository_PostgresPhoneNormalization(t *testing.T) {
	store := openPostgresStoreForIntegrationTest(t)
	repo := NewCourierRepository(store)

	now := time.Now().UTC().Round(time.Second)
	courier := domain.Courier{
		ID:          "courier-phone",
		Phone:       "+7 (999) 000-00-77",
		FirstName:   "Nina",
		LastName:    "Phone",
		VehicleType: domain.VehicleTypeBike,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := repo.Create(courier); err != nil {
		t.Fatalf("create courier: %v", err)
	}

	got, err := repo.Get(courier.ID)
	if err != nil {
		t.Fatalf("get courier: %v", err)
	}
	if got.Phone != "+79990000077" {
		t.Fatalf("expected normalized phone +79990000077, got %s", got.Phone)
	}

	if _, err := repo.GetByPhone("+79990000077"); err != nil {
		t.Fatalf("get by normalized phone: %v", err)
	}
}

func TestCourierRepository_PostgresZones(t *testing.T) {
	store := openPostgresStoreForIntegrationTest(t)
	repo := NewCourierRepository(store)

	now := time.Now().UTC().Round(time.Second)
	car := domain.Courier{
		ID:          "courier-car",
		Phone:       "+79990000002",
		FirstName:   "Pavel",
		LastName:    "Car",
		VehicleType: domain.VehicleTypeCar,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	scooter := domain.Courier{
		ID:          "courier-scooter",
		Phone:       "+79990000003",
		FirstName:   "Sasha",
		LastName:    "Scooter",
		VehicleType: domain.VehicleTypeScooter,
		IsActive:    true,
		CreatedAt:   now.Add(time.Second),
		UpdatedAt:   now.Add(time.Second),
	}

	if err := repo.Create(car); err != nil {
		t.Fatalf("create car courier: %v", err)
	}
	if err := repo.Create(scooter); err != nil {
		t.Fatalf("create scooter courier: %v", err)
	}

	err := repo.ReplaceZones(scooter.ID, []domain.CourierZone{
		{ZoneID: "msk-cao-arbat", IsPrimary: true},
		{ZoneID: "msk-cao-tverskoy", IsPrimary: false},
	})
	if !errors.Is(err, domain.ErrCourierZoneLimitExceeded) {
		t.Fatalf("expected ErrCourierZoneLimitExceeded, got %v", err)
	}

	if err := repo.ReplaceZones(car.ID, []domain.CourierZone{
		{ZoneID: "msk-cao-arbat", IsPrimary: true},
		{ZoneID: "msk-cao-tverskoy", IsPrimary: false},
	}); err != nil {
		t.Fatalf("replace zones for car: %v", err)
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

	zones, err := repo.ListZones(car.ID)
	if err != nil {
		t.Fatalf("list zones: %v", err)
	}
	if len(zones) != 2 || !zones[0].IsPrimary {
		t.Fatalf("unexpected zones: %+v", zones)
	}
}

func TestCourierRepository_PostgresZoneValidationErrors(t *testing.T) {
	store := openPostgresStoreForIntegrationTest(t)
	repo := NewCourierRepository(store)

	now := time.Now().UTC().Round(time.Second)
	courier := domain.Courier{
		ID:          "courier-zone-errors",
		Phone:       "+79990000008",
		FirstName:   "Olga",
		LastName:    "Zones",
		VehicleType: domain.VehicleTypeCar,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
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

func TestCourierRepository_PostgresZoneCapacityLimit(t *testing.T) {
	store := openPostgresStoreForIntegrationTest(t)
	repo := NewCourierRepository(store)

	now := time.Now().UTC().Round(time.Second)
	zoneID := "msk-cao-arbat"

	for i := 0; i < domain.MaxCouriersPerZoneDefault; i++ {
		id := fmt.Sprintf("courier-cap-%d", i)
		phone := fmt.Sprintf("+7999111%04d", i)
		courier := domain.Courier{
			ID:          id,
			Phone:       phone,
			FirstName:   "Cap",
			LastName:    "Test",
			VehicleType: domain.VehicleTypeScooter,
			IsActive:    true,
			CreatedAt:   now.Add(time.Duration(i) * time.Second),
			UpdatedAt:   now.Add(time.Duration(i) * time.Second),
		}
		if err := repo.Create(courier); err != nil {
			t.Fatalf("create courier %s: %v", id, err)
		}
		if err := repo.ReplaceZones(courier.ID, []domain.CourierZone{
			{ZoneID: zoneID, IsPrimary: true},
		}); err != nil {
			t.Fatalf("assign zone for %s: %v", id, err)
		}
	}

	overflow := domain.Courier{
		ID:          "courier-cap-overflow",
		Phone:       "+79991119999",
		FirstName:   "Cap",
		LastName:    "Overflow",
		VehicleType: domain.VehicleTypeScooter,
		IsActive:    true,
		CreatedAt:   now.Add(time.Hour),
		UpdatedAt:   now.Add(time.Hour),
	}
	if err := repo.Create(overflow); err != nil {
		t.Fatalf("create overflow courier: %v", err)
	}

	if err := repo.ReplaceZones(overflow.ID, []domain.CourierZone{
		{ZoneID: zoneID, IsPrimary: true},
	}); !errors.Is(err, domain.ErrCourierZoneCapacityExceeded) {
		t.Fatalf("expected ErrCourierZoneCapacityExceeded, got %v", err)
	}
}

func TestCourierRepository_PostgresSlots(t *testing.T) {
	store := openPostgresStoreForIntegrationTest(t)
	repo := NewCourierRepository(store)

	now := time.Now().UTC().Round(time.Second)
	courier := domain.Courier{
		ID:          "courier-slot",
		Phone:       "+79990000004",
		FirstName:   "Dmitriy",
		LastName:    "Slot",
		VehicleType: domain.VehicleTypeBike,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
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

func TestCourierRepository_PostgresValidationAndNotFoundBranches(t *testing.T) {
	store := openPostgresStoreForIntegrationTest(t)
	repo := NewCourierRepository(store)
	now := time.Now().UTC().Round(time.Second)

	if _, err := repo.Get(""); !errors.Is(err, domain.ErrCourierIDRequired) {
		t.Fatalf("expected ErrCourierIDRequired for empty id, got %v", err)
	}
	if _, err := repo.Get("missing"); !errors.Is(err, domain.ErrCourierNotFound) {
		t.Fatalf("expected ErrCourierNotFound for missing id, got %v", err)
	}
	if _, err := repo.GetByPhone("bad-phone"); !errors.Is(err, domain.ErrCourierPhoneFormatInvalid) {
		t.Fatalf("expected ErrCourierPhoneFormatInvalid, got %v", err)
	}

	missing := domain.Courier{
		ID:          "missing-save",
		Phone:       "+79990000101",
		FirstName:   "No",
		LastName:    "Row",
		VehicleType: domain.VehicleTypeBike,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := repo.Save(missing); !errors.Is(err, domain.ErrCourierNotFound) {
		t.Fatalf("expected ErrCourierNotFound on save missing courier, got %v", err)
	}

	if _, err := repo.ListByZone("", 10); !errors.Is(err, domain.ErrCourierZoneRequired) {
		t.Fatalf("expected ErrCourierZoneRequired, got %v", err)
	}

	if err := repo.ReplaceZones("", []domain.CourierZone{{ZoneID: "msk-cao-arbat", IsPrimary: true}}); !errors.Is(err, domain.ErrCourierIDRequired) {
		t.Fatalf("expected ErrCourierIDRequired for empty courier id, got %v", err)
	}
	if err := repo.ReplaceZones("missing", []domain.CourierZone{{ZoneID: "msk-cao-arbat", IsPrimary: true}}); !errors.Is(err, domain.ErrCourierNotFound) {
		t.Fatalf("expected ErrCourierNotFound for missing courier in replace zones, got %v", err)
	}

	if _, err := repo.ListZones(""); !errors.Is(err, domain.ErrCourierIDRequired) {
		t.Fatalf("expected ErrCourierIDRequired for empty courier id in list zones, got %v", err)
	}
	if _, err := repo.ListZones("missing"); !errors.Is(err, domain.ErrCourierNotFound) {
		t.Fatalf("expected ErrCourierNotFound for missing courier in list zones, got %v", err)
	}

	if _, err := repo.ListSlots("", time.Time{}, time.Time{}); !errors.Is(err, domain.ErrCourierIDRequired) {
		t.Fatalf("expected ErrCourierIDRequired for empty courier id in list slots, got %v", err)
	}
	if _, err := repo.ListSlots("missing", time.Time{}, time.Time{}); !errors.Is(err, domain.ErrCourierNotFound) {
		t.Fatalf("expected ErrCourierNotFound for missing courier in list slots, got %v", err)
	}

	slot := domain.CourierSlot{
		ID:            "slot-missing-courier",
		CourierID:     "missing",
		SlotStart:     now.Add(9 * time.Hour),
		SlotEnd:       now.Add(13 * time.Hour),
		DurationHours: 4,
	}
	if err := repo.CreateSlot(slot); !errors.Is(err, domain.ErrCourierNotFound) {
		t.Fatalf("expected ErrCourierNotFound for missing courier in create slot, got %v", err)
	}
}

func TestCourierRepository_PostgresUniqueAndListSlotFilterBranches(t *testing.T) {
	store := openPostgresStoreForIntegrationTest(t)
	repo := NewCourierRepository(store)
	now := time.Now().UTC().Round(time.Second)

	base := domain.Courier{
		ID:          "courier-unique-base",
		Phone:       "+79990000111",
		FirstName:   "Base",
		LastName:    "Courier",
		VehicleType: domain.VehicleTypeCar,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := repo.Create(base); err != nil {
		t.Fatalf("create base courier: %v", err)
	}

	dupID := base
	dupID.Phone = "+79990000112"
	if err := repo.Create(dupID); !errors.Is(err, domain.ErrCourierAlreadyExists) {
		t.Fatalf("expected ErrCourierAlreadyExists for duplicate id, got %v", err)
	}

	dupPhone := base
	dupPhone.ID = "courier-unique-other"
	if err := repo.Create(dupPhone); !errors.Is(err, domain.ErrCourierPhoneAlreadyExists) {
		t.Fatalf("expected ErrCourierPhoneAlreadyExists for duplicate phone, got %v", err)
	}

	if err := repo.ReplaceZones(base.ID, []domain.CourierZone{{ZoneID: "msk-cao-arbat", IsPrimary: true}}); err != nil {
		t.Fatalf("assign zone to base courier: %v", err)
	}

	inZone, err := repo.ListByZone("msk-cao-arbat", 0)
	if err != nil {
		t.Fatalf("list by zone without limit: %v", err)
	}
	if len(inZone) == 0 {
		t.Fatal("expected at least one courier in zone")
	}

	slot1 := domain.CourierSlot{
		ID:            "slot-filter-1",
		CourierID:     base.ID,
		SlotStart:     now.Add(9 * time.Hour),
		SlotEnd:       now.Add(13 * time.Hour),
		DurationHours: 4,
	}
	slot2 := domain.CourierSlot{
		ID:            "slot-filter-2",
		CourierID:     base.ID,
		SlotStart:     now.Add(15 * time.Hour),
		SlotEnd:       now.Add(19 * time.Hour),
		DurationHours: 4,
	}
	if err := repo.CreateSlot(slot1); err != nil {
		t.Fatalf("create slot1: %v", err)
	}
	if err := repo.CreateSlot(slot2); err != nil {
		t.Fatalf("create slot2: %v", err)
	}

	duplicateSlotID := slot2
	duplicateSlotID.SlotStart = now.Add(20 * time.Hour)
	duplicateSlotID.SlotEnd = now.Add(24 * time.Hour)
	duplicateSlotID.DurationHours = 4
	duplicateSlotID.Status = domain.CourierSlotStatusPlanned
	duplicateSlotID.ID = slot1.ID
	if err := repo.CreateSlot(duplicateSlotID); !errors.Is(err, domain.ErrCourierSlotConflict) {
		t.Fatalf("expected ErrCourierSlotConflict for duplicate slot id, got %v", err)
	}

	slotsNoFilter, err := repo.ListSlots(base.ID, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("list slots without filters: %v", err)
	}
	if len(slotsNoFilter) != 2 {
		t.Fatalf("expected 2 slots without filters, got %d", len(slotsNoFilter))
	}

	fromFiltered, err := repo.ListSlots(base.ID, now.Add(14*time.Hour), time.Time{})
	if err != nil {
		t.Fatalf("list slots with from filter only: %v", err)
	}
	if len(fromFiltered) != 1 || fromFiltered[0].ID != slot2.ID {
		t.Fatalf("unexpected from-filtered slots: %+v", fromFiltered)
	}

	toFiltered, err := repo.ListSlots(base.ID, time.Time{}, now.Add(14*time.Hour))
	if err != nil {
		t.Fatalf("list slots with to filter only: %v", err)
	}
	if len(toFiltered) != 1 || toFiltered[0].ID != slot1.ID {
		t.Fatalf("unexpected to-filtered slots: %+v", toFiltered)
	}
}
