package omsv1

import (
	"reflect"
	"strings"
	"testing"
)

func TestOrderStatusGeneratedHelpers(t *testing.T) {
	s := OrderStatus_ORDER_STATUS_PAID
	if got := s.Enum(); got == nil || *got != s {
		t.Fatalf("Enum() mismatch: got %v want %v", got, s)
	}
	if s.String() == "" {
		t.Fatalf("String() must not be empty")
	}
	if s.Type() == nil {
		t.Fatalf("Type() must not be nil")
	}
	if s.Descriptor() == nil {
		t.Fatalf("Descriptor() must not be nil")
	}
	_ = s.Number()
	_, _ = s.EnumDescriptor()

	unknown := OrderStatus(999)
	if unknown.String() == "" {
		t.Fatalf("unknown enum string must not be empty")
	}
}

func TestCourierVehicleTypeGeneratedHelpers(t *testing.T) {
	s := CourierVehicleType_COURIER_VEHICLE_TYPE_BIKE
	if got := s.Enum(); got == nil || *got != s {
		t.Fatalf("Enum() mismatch: got %v want %v", got, s)
	}
	if s.String() == "" {
		t.Fatalf("String() must not be empty")
	}
	if s.Type() == nil {
		t.Fatalf("Type() must not be nil")
	}
	if s.Descriptor() == nil {
		t.Fatalf("Descriptor() must not be nil")
	}
	_ = s.Number()
	_, _ = s.EnumDescriptor()
}

func TestCourierSlotStatusGeneratedHelpers(t *testing.T) {
	s := CourierSlotStatus_COURIER_SLOT_STATUS_PLANNED
	if got := s.Enum(); got == nil || *got != s {
		t.Fatalf("Enum() mismatch: got %v want %v", got, s)
	}
	if s.String() == "" {
		t.Fatalf("String() must not be empty")
	}
	if s.Type() == nil {
		t.Fatalf("Type() must not be nil")
	}
	if s.Descriptor() == nil {
		t.Fatalf("Descriptor() must not be nil")
	}
	_ = s.Number()
	_, _ = s.EnumDescriptor()
}

func TestCourierRatingTagGeneratedHelpers(t *testing.T) {
	s := CourierRatingTag_COURIER_RATING_TAG_ON_TIME
	if got := s.Enum(); got == nil || *got != s {
		t.Fatalf("Enum() mismatch: got %v want %v", got, s)
	}
	if s.String() == "" {
		t.Fatalf("String() must not be empty")
	}
	if s.Type() == nil {
		t.Fatalf("Type() must not be nil")
	}
	if s.Descriptor() == nil {
		t.Fatalf("Descriptor() must not be nil")
	}
	_ = s.Number()
	_, _ = s.EnumDescriptor()
}

func TestGeneratedMessageHelpers(t *testing.T) {
	messages := []any{
		&Money{Currency: "USD", AmountMinor: 100},
		&OrderItem{Sku: "SKU-1", Qty: 1, Price: &Money{Currency: "USD", AmountMinor: 100}},
		&Order{Id: "order-1", CustomerId: "cust-1", Status: OrderStatus_ORDER_STATUS_PENDING, Amount: &Money{Currency: "USD", AmountMinor: 100}, Items: []*OrderItem{{Sku: "SKU-1", Qty: 1, Price: &Money{Currency: "USD", AmountMinor: 100}}}, Version: 1, Currency: "USD"},
		&TimelineEvent{Type: "OrderCreated", Reason: "test", UnixTime: 1},
		&CreateOrderRequest{CustomerId: "cust-1", Currency: "USD", Items: []*OrderItem{{Sku: "SKU-1", Qty: 1, Price: &Money{Currency: "USD", AmountMinor: 100}}}},
		&CreateOrderResponse{Order: &Order{Id: "order-1"}},
		&GetOrderRequest{OrderId: "order-1"},
		&GetOrderResponse{Order: &Order{Id: "order-1"}, Timeline: []*TimelineEvent{{Type: "OrderCreated", UnixTime: 1}}},
		&ListOrdersRequest{CustomerId: "cust-1", PageSize: 10, PageToken: "token", FilterStatuses: []OrderStatus{OrderStatus_ORDER_STATUS_PENDING}},
		&ListOrdersResponse{Orders: []*Order{{Id: "order-1"}}, NextPageToken: "next"},
		&PayOrderRequest{OrderId: "order-1"},
		&PayOrderResponse{OrderId: "order-1", Status: OrderStatus_ORDER_STATUS_PAID},
		&CancelOrderRequest{OrderId: "order-1", Reason: "user-request"},
		&CancelOrderResponse{OrderId: "order-1", Status: OrderStatus_ORDER_STATUS_CANCELED},
		&RefundOrderRequest{OrderId: "order-1", Amount: &Money{Currency: "USD", AmountMinor: 50}, Reason: "partial"},
		&RefundOrderResponse{OrderId: "order-1", Status: OrderStatus_ORDER_STATUS_REFUNDED},
		&CourierZoneInput{ZoneId: "msk-cao-arbat", IsPrimary: true},
		&CourierZone{ZoneId: "msk-cao-arbat", IsPrimary: true, AssignedAtUnix: 1},
		&Courier{
			Id:          "courier-1",
			Phone:       "+79990000001",
			FirstName:   "Ivan",
			LastName:    "Petrov",
			VehicleType: CourierVehicleType_COURIER_VEHICLE_TYPE_BIKE,
			IsActive:    true,
			Zones:       []*CourierZone{{ZoneId: "msk-cao-arbat", IsPrimary: true, AssignedAtUnix: 1}},
		},
		&CourierSlot{
			Id:            "slot-1",
			CourierId:     "courier-1",
			SlotStartUnix: 10,
			SlotEndUnix:   20,
			DurationHours: 4,
			Status:        CourierSlotStatus_COURIER_SLOT_STATUS_PLANNED,
		},
		&RegisterCourierRequest{
			CourierId:   "courier-1",
			Phone:       "+79990000001",
			FirstName:   "Ivan",
			LastName:    "Petrov",
			VehicleType: CourierVehicleType_COURIER_VEHICLE_TYPE_BIKE,
			Zones:       []*CourierZoneInput{{ZoneId: "msk-cao-arbat", IsPrimary: true}},
		},
		&RegisterCourierResponse{Courier: &Courier{Id: "courier-1"}},
		&GetCourierRequest{CourierId: "courier-1"},
		&GetCourierResponse{Courier: &Courier{Id: "courier-1"}},
		&ListCouriersByZoneRequest{ZoneId: "msk-cao-arbat", Limit: 10},
		&ListCouriersByZoneResponse{Couriers: []*Courier{{Id: "courier-1"}}},
		&ReplaceCourierZonesRequest{
			CourierId: "courier-1",
			Zones:     []*CourierZoneInput{{ZoneId: "msk-cao-arbat", IsPrimary: true}},
		},
		&ReplaceCourierZonesResponse{CourierId: "courier-1", Zones: []*CourierZone{{ZoneId: "msk-cao-arbat", IsPrimary: true, AssignedAtUnix: 1}}},
		&CreateCourierSlotRequest{
			CourierId:     "courier-1",
			SlotStartUnix: 10,
			SlotEndUnix:   20,
			DurationHours: 4,
		},
		&CreateCourierSlotResponse{Slot: &CourierSlot{Id: "slot-1"}},
		&ListCourierSlotsRequest{CourierId: "courier-1", FromUnix: 1, ToUnix: 2},
		&ListCourierSlotsResponse{Slots: []*CourierSlot{{Id: "slot-1"}}},
		&SubmitCourierRatingRequest{
			RatingId:  "rating-1",
			CourierId: "courier-1",
			Score:     5,
			Tags:      []CourierRatingTag{CourierRatingTag_COURIER_RATING_TAG_ON_TIME},
			Comment:   "Great",
		},
		&SubmitCourierRatingResponse{
			RatingId:  "rating-1",
			CourierId: "courier-1",
		},
		&GetCourierRatingSummaryRequest{
			CourierId: "courier-1",
		},
		&CourierRatingSummary{
			CourierId:       "courier-1",
			RatingsCount:    2,
			AverageScore:    4.5,
			LowRatingsCount: 0,
			Score_5Count:    1,
			Score_4Count:    1,
			OnTimeCount:     1,
			PoliteCount:     1,
			LastRatingUnix:  100,
		},
		&GetCourierRatingSummaryResponse{
			Summary: &CourierRatingSummary{CourierId: "courier-1"},
		},
	}

	for _, msg := range messages {
		t.Run(reflect.TypeOf(msg).Elem().Name(), func(t *testing.T) {
			exerciseGeneratedMessage(t, msg)
		})
	}
}

func TestFileDescriptorMetadata(t *testing.T) {
	fd := File_proto_oms_v1_order_service_proto
	if fd.Path() == "" {
		t.Fatalf("descriptor path must not be empty")
	}
	if fd.Messages().Len() == 0 {
		t.Fatalf("expected non-empty message descriptors")
	}
	if fd.Enums().Len() == 0 {
		t.Fatalf("expected non-empty enum descriptors")
	}
	if fd.Services().Len() == 0 {
		t.Fatalf("expected non-empty service descriptors")
	}
	if got := fd.Services().Get(0).Name(); got == "" {
		t.Fatalf("expected service name, got empty")
	}
}

func exerciseGeneratedMessage(t *testing.T, msg any) {
	t.Helper()

	v := reflect.ValueOf(msg)

	callNoArg(t, v, "String")
	callNoArg(t, v, "ProtoReflect")
	callNoArg(t, v, "Descriptor")
	callNoArg(t, v, "Reset")
	callGetterMethods(t, v)

	nilReceiver := reflect.Zero(v.Type())
	callNoArg(t, nilReceiver, "ProtoReflect")
	callNoArg(t, nilReceiver, "Descriptor")
	callGetterMethods(t, nilReceiver)
}

func callGetterMethods(t *testing.T, v reflect.Value) {
	t.Helper()

	typ := v.Type()
	for i := 0; i < typ.NumMethod(); i++ {
		m := typ.Method(i)
		if !strings.HasPrefix(m.Name, "Get") {
			continue
		}
		if m.Type.NumIn() != 1 || m.Type.NumOut() != 1 {
			continue
		}
		callNoArg(t, v, m.Name)
	}
}

func callNoArg(t *testing.T, v reflect.Value, method string) {
	t.Helper()

	mv := v.MethodByName(method)
	if !mv.IsValid() {
		return
	}
	if mv.Type().NumIn() != 0 {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("method %s panicked: %v", method, r)
		}
	}()

	_ = mv.Call(nil)
}
