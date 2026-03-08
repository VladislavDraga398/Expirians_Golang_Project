package grpcsvc

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/vladislavdragonenkov/oms/internal/storage/memory"
	omsv1 "github.com/vladislavdragonenkov/oms/proto/oms/v1"
)

func TestCourierService_RegisterAndGetCourier(t *testing.T) {
	service := NewCourierService(memory.NewCourierRepository(), nil)

	registerResp, err := service.RegisterCourier(context.Background(), &omsv1.RegisterCourierRequest{
		Phone:       "+7 (999) 000-00-01",
		FirstName:   "Ivan",
		LastName:    "Petrov",
		VehicleType: omsv1.CourierVehicleType_COURIER_VEHICLE_TYPE_BIKE,
		Zones: []*omsv1.CourierZoneInput{
			{ZoneId: "msk-cao-arbat", IsPrimary: true},
		},
	})
	if err != nil {
		t.Fatalf("register courier: %v", err)
	}
	if registerResp.GetCourier().GetId() == "" {
		t.Fatal("expected generated courier id")
	}
	if registerResp.GetCourier().GetPhone() != "+79990000001" {
		t.Fatalf("unexpected normalized phone: %s", registerResp.GetCourier().GetPhone())
	}
	if len(registerResp.GetCourier().GetZones()) != 1 || !registerResp.GetCourier().GetZones()[0].GetIsPrimary() {
		t.Fatalf("unexpected zones payload: %+v", registerResp.GetCourier().GetZones())
	}

	getResp, err := service.GetCourier(context.Background(), &omsv1.GetCourierRequest{CourierId: registerResp.GetCourier().GetId()})
	if err != nil {
		t.Fatalf("get courier: %v", err)
	}
	if getResp.GetCourier().GetId() != registerResp.GetCourier().GetId() {
		t.Fatalf("unexpected courier id: got=%s want=%s", getResp.GetCourier().GetId(), registerResp.GetCourier().GetId())
	}
}

func TestCourierService_RegisterValidationError(t *testing.T) {
	service := NewCourierService(memory.NewCourierRepository(), nil)

	_, err := service.RegisterCourier(context.Background(), &omsv1.RegisterCourierRequest{
		Phone:     "+79990000001",
		FirstName: "Ivan",
		LastName:  "Petrov",
		Zones: []*omsv1.CourierZoneInput{
			{ZoneId: "msk-cao-arbat"},
		},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
	}
}

func TestCourierService_ReplaceZonesLimitExceededForScooter(t *testing.T) {
	service := NewCourierService(memory.NewCourierRepository(), nil)

	registerResp, err := service.RegisterCourier(context.Background(), &omsv1.RegisterCourierRequest{
		CourierId:   "courier-scooter",
		Phone:       "+79990000002",
		FirstName:   "Petr",
		LastName:    "Sidorov",
		VehicleType: omsv1.CourierVehicleType_COURIER_VEHICLE_TYPE_SCOOTER,
		Zones: []*omsv1.CourierZoneInput{
			{ZoneId: "msk-cao-arbat", IsPrimary: true},
		},
	})
	if err != nil {
		t.Fatalf("register courier: %v", err)
	}

	_, err = service.ReplaceCourierZones(context.Background(), &omsv1.ReplaceCourierZonesRequest{
		CourierId: registerResp.GetCourier().GetId(),
		Zones: []*omsv1.CourierZoneInput{
			{ZoneId: "msk-cao-arbat", IsPrimary: true},
			{ZoneId: "msk-cao-tverskoy"},
		},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
	}
}

func TestCourierService_CreateAndListCourierSlots(t *testing.T) {
	service := NewCourierService(memory.NewCourierRepository(), nil)

	registerResp, err := service.RegisterCourier(context.Background(), &omsv1.RegisterCourierRequest{
		CourierId:   "courier-slot",
		Phone:       "+79990000003",
		FirstName:   "Sergey",
		LastName:    "Volkov",
		VehicleType: omsv1.CourierVehicleType_COURIER_VEHICLE_TYPE_CAR,
		Zones: []*omsv1.CourierZoneInput{
			{ZoneId: "msk-cao-arbat", IsPrimary: true},
			{ZoneId: "msk-cao-tverskoy"},
		},
	})
	if err != nil {
		t.Fatalf("register courier: %v", err)
	}

	start := time.Now().UTC().Truncate(time.Second).Add(2 * time.Hour)
	end := start.Add(4 * time.Hour)
	createResp, err := service.CreateCourierSlot(context.Background(), &omsv1.CreateCourierSlotRequest{
		CourierId:     registerResp.GetCourier().GetId(),
		SlotStartUnix: start.Unix(),
		SlotEndUnix:   end.Unix(),
		DurationHours: 4,
	})
	if err != nil {
		t.Fatalf("create courier slot: %v", err)
	}
	if createResp.GetSlot().GetId() == "" {
		t.Fatal("expected generated slot id")
	}

	listResp, err := service.ListCourierSlots(context.Background(), &omsv1.ListCourierSlotsRequest{
		CourierId: registerResp.GetCourier().GetId(),
		FromUnix:  start.Add(-time.Minute).Unix(),
		ToUnix:    end.Add(time.Minute).Unix(),
	})
	if err != nil {
		t.Fatalf("list courier slots: %v", err)
	}
	if len(listResp.GetSlots()) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(listResp.GetSlots()))
	}
	if listResp.GetSlots()[0].GetStatus() != omsv1.CourierSlotStatus_COURIER_SLOT_STATUS_PLANNED {
		t.Fatalf("unexpected slot status: %s", listResp.GetSlots()[0].GetStatus().String())
	}
}
