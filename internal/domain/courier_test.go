package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

func TestCourierValidateInvariants(t *testing.T) {
	now := time.Now().UTC()
	c := domain.Courier{
		ID:          "courier-1",
		Phone:       "+79990000001",
		FirstName:   "Ivan",
		LastName:    "Petrov",
		VehicleType: domain.VehicleTypeCar,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if errs := c.ValidateInvariants(); len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}

	invalid := c
	invalid.Phone = "abc"
	invalid.VehicleType = domain.VehicleType("train")
	if errs := invalid.ValidateInvariants(); len(errs) == 0 {
		t.Fatal("expected validation errors for invalid courier")
	}
}

func TestVehicleTypeHelpers(t *testing.T) {
	tests := []struct {
		name             string
		vehicleType      domain.VehicleType
		valid            bool
		allowsMultiZones bool
	}{
		{name: "scooter", vehicleType: domain.VehicleTypeScooter, valid: true, allowsMultiZones: false},
		{name: "bike", vehicleType: domain.VehicleTypeBike, valid: true, allowsMultiZones: false},
		{name: "car", vehicleType: domain.VehicleTypeCar, valid: true, allowsMultiZones: true},
		{name: "invalid", vehicleType: domain.VehicleType("truck"), valid: false, allowsMultiZones: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.vehicleType.Valid(); got != tt.valid {
				t.Fatalf("valid mismatch: got=%v want=%v", got, tt.valid)
			}
			if got := tt.vehicleType.AllowsMultipleZones(); got != tt.allowsMultiZones {
				t.Fatalf("allows multiple zones mismatch: got=%v want=%v", got, tt.allowsMultiZones)
			}
		})
	}
}

func TestNormalizePhone(t *testing.T) {
	normalized, err := domain.NormalizePhone("+7 (999) 000-00-01")
	if err != nil {
		t.Fatalf("normalize phone: %v", err)
	}
	if normalized != "+79990000001" {
		t.Fatalf("unexpected normalized phone: %s", normalized)
	}

	if _, err := domain.NormalizePhone("12345"); !errors.Is(err, domain.ErrCourierPhoneFormatInvalid) {
		t.Fatalf("expected ErrCourierPhoneFormatInvalid, got %v", err)
	}
}

func TestCourierZoneValidateInvariants(t *testing.T) {
	zone := domain.CourierZone{
		CourierID: "courier-1",
		ZoneID:    "msk-cao-arbat",
		IsPrimary: true,
	}
	if errs := zone.ValidateInvariants(); len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}

	zone.ZoneID = ""
	if errs := zone.ValidateInvariants(); len(errs) == 0 {
		t.Fatal("expected validation errors for zone")
	}

	zone.ZoneID = "msk-cao-unknown"
	if errs := zone.ValidateInvariants(); len(errs) == 0 {
		t.Fatal("expected validation errors for unknown zone")
	}
}

func TestCourierSlotValidateInvariants(t *testing.T) {
	start := time.Now().UTC().Round(time.Second)
	slot := domain.CourierSlot{
		ID:            "slot-1",
		CourierID:     "courier-1",
		SlotStart:     start,
		SlotEnd:       start.Add(4 * time.Hour),
		DurationHours: 4,
		Status:        domain.CourierSlotStatusPlanned,
		CreatedAt:     start,
		UpdatedAt:     start,
	}
	if errs := slot.ValidateInvariants(); len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}

	slot.DurationHours = 6
	if errs := slot.ValidateInvariants(); len(errs) == 0 {
		t.Fatal("expected validation errors for slot duration")
	}

	slot.DurationHours = 4
	slot.SlotEnd = slot.SlotStart.Add(5 * time.Hour)
	if errs := slot.ValidateInvariants(); len(errs) == 0 {
		t.Fatal("expected validation errors for slot duration mismatch")
	}
}

func TestCourierSlotStatusValid(t *testing.T) {
	validStatuses := []domain.CourierSlotStatus{
		domain.CourierSlotStatusPlanned,
		domain.CourierSlotStatusActive,
		domain.CourierSlotStatusCompleted,
		domain.CourierSlotStatusCanceled,
	}
	for _, st := range validStatuses {
		if !st.Valid() {
			t.Fatalf("expected status %q to be valid", st)
		}
	}

	if domain.CourierSlotStatus("unknown").Valid() {
		t.Fatal("expected unknown status to be invalid")
	}
}

func TestCourierRatingValidateInvariants(t *testing.T) {
	now := time.Now().UTC()

	valid := domain.CourierRating{
		ID:        "rating-1",
		CourierID: "courier-1",
		Score:     5,
		Tags: []domain.CourierRatingTag{
			domain.CourierRatingTagOnTime,
			domain.CourierRatingTagPolite,
		},
		Comment:   "Отлично",
		CreatedAt: now,
	}
	if errs := valid.ValidateInvariants(); len(errs) != 0 {
		t.Fatalf("expected no validation errors, got %v", errs)
	}

	lowWithoutReasons := valid
	lowWithoutReasons.ID = "rating-2"
	lowWithoutReasons.Score = 2
	lowWithoutReasons.Tags = nil
	if errs := lowWithoutReasons.ValidateInvariants(); len(errs) == 0 {
		t.Fatal("expected validation errors for low rating without reasons")
	}

	fiveWithNegative := valid
	fiveWithNegative.ID = "rating-3"
	fiveWithNegative.Tags = []domain.CourierRatingTag{
		domain.CourierRatingTagOnTime,
		domain.CourierRatingTagDelayedDelivery,
	}
	if errs := fiveWithNegative.ValidateInvariants(); len(errs) == 0 {
		t.Fatal("expected validation errors for 5-star rating with negative tags")
	}
}

func TestCourierRatingTagHelpers(t *testing.T) {
	if !domain.CourierRatingTagOnTime.Valid() || !domain.CourierRatingTagOnTime.IsPositive() {
		t.Fatal("expected on_time tag to be valid and positive")
	}
	if domain.CourierRatingTagOnTime.IsNegative() {
		t.Fatal("expected on_time tag to be non-negative")
	}

	if !domain.CourierRatingTagDelayedDelivery.Valid() || !domain.CourierRatingTagDelayedDelivery.IsNegative() {
		t.Fatal("expected delayed_delivery tag to be valid and negative")
	}
	if domain.CourierRatingTagDelayedDelivery.IsPositive() {
		t.Fatal("expected delayed_delivery tag to be non-positive")
	}

	if domain.CourierRatingTag("unknown").Valid() {
		t.Fatal("expected unknown rating tag to be invalid")
	}
}
