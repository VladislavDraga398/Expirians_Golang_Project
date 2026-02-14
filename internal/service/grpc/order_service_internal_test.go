package grpcsvc

import (
	"context"
	"errors"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/vladislavdragonenkov/oms/internal/domain"
	omsv1 "github.com/vladislavdragonenkov/oms/proto/oms/v1"
)

type stubOrderRepository struct {
	createFn func(domain.Order) error
	getFn    func(string) (domain.Order, error)
	listFn   func(string, int) ([]domain.Order, error)
	saveFn   func(domain.Order) error
}

func (s *stubOrderRepository) Create(order domain.Order) error {
	if s.createFn != nil {
		return s.createFn(order)
	}
	return nil
}

func (s *stubOrderRepository) Get(id string) (domain.Order, error) {
	if s.getFn != nil {
		return s.getFn(id)
	}
	return domain.Order{}, domain.ErrOrderNotFound
}

func (s *stubOrderRepository) ListByCustomer(customerID string, limit int) ([]domain.Order, error) {
	if s.listFn != nil {
		return s.listFn(customerID, limit)
	}
	return nil, nil
}

func (s *stubOrderRepository) Save(order domain.Order) error {
	if s.saveFn != nil {
		return s.saveFn(order)
	}
	return nil
}

type stubTimelineRepository struct {
	appendFn func(domain.TimelineEvent) error
	listFn   func(string) ([]domain.TimelineEvent, error)
}

func (s *stubTimelineRepository) Append(event domain.TimelineEvent) error {
	if s.appendFn != nil {
		return s.appendFn(event)
	}
	return nil
}

func (s *stubTimelineRepository) List(orderID string) ([]domain.TimelineEvent, error) {
	if s.listFn != nil {
		return s.listFn(orderID)
	}
	return nil, nil
}

type stubIdempotencyRepository struct {
	markDoneFn   func(string, []byte, int) error
	markFailedFn func(string, []byte, int) error
}

func (s *stubIdempotencyRepository) CreateProcessing(string, string, time.Time) (domain.IdempotencyRecord, error) {
	return domain.IdempotencyRecord{}, errors.New("not implemented")
}

func (s *stubIdempotencyRepository) Get(string) (domain.IdempotencyRecord, error) {
	return domain.IdempotencyRecord{}, errors.New("not implemented")
}

func (s *stubIdempotencyRepository) MarkDone(key string, body []byte, code int) error {
	if s.markDoneFn != nil {
		return s.markDoneFn(key, body, code)
	}
	return nil
}

func (s *stubIdempotencyRepository) MarkFailed(key string, body []byte, code int) error {
	if s.markFailedFn != nil {
		return s.markFailedFn(key, body, code)
	}
	return nil
}

func (s *stubIdempotencyRepository) DeleteExpired(time.Time, int) (int, error) {
	return 0, nil
}

func newInternalTestService(repo domain.OrderRepository) *OrderService {
	return NewOrderService(
		repo,
		&stubTimelineRepository{},
		nil,
		nil,
		log.New().WithField("test", "internal"),
	)
}

func mustStatusCode(t *testing.T, err error, expected codes.Code) {
	t.Helper()
	if status.Code(err) != expected {
		t.Fatalf("expected code %s, got %s (err=%v)", expected, status.Code(err), err)
	}
}

func validCreateRequest() *omsv1.CreateOrderRequest {
	return &omsv1.CreateOrderRequest{
		CustomerId: "customer-1",
		Currency:   "USD",
		Items: []*omsv1.OrderItem{
			{
				Sku: "sku-1",
				Qty: 2,
				Price: &omsv1.Money{
					Currency:    "USD",
					AmountMinor: 100,
				},
			},
		},
	}
}

func TestNewOrderService_NilLogger(t *testing.T) {
	service := NewOrderService(&stubOrderRepository{}, &stubTimelineRepository{}, nil, nil, nil)
	if service.logger == nil {
		t.Fatal("logger must be initialized when nil logger is provided")
	}
}

func TestCreateOrderInternal_ValidationErrors(t *testing.T) {
	service := newInternalTestService(&stubOrderRepository{})

	tests := []struct {
		name string
		req  *omsv1.CreateOrderRequest
	}{
		{name: "nil request", req: nil},
		{name: "customer required", req: &omsv1.CreateOrderRequest{Currency: "USD", Items: []*omsv1.OrderItem{{Qty: 1, Price: &omsv1.Money{Currency: "USD", AmountMinor: 1}}}}},
		{name: "currency required", req: &omsv1.CreateOrderRequest{CustomerId: "c", Items: []*omsv1.OrderItem{{Qty: 1, Price: &omsv1.Money{Currency: "USD", AmountMinor: 1}}}}},
		{name: "items required", req: &omsv1.CreateOrderRequest{CustomerId: "c", Currency: "USD"}},
		{name: "nil item", req: &omsv1.CreateOrderRequest{CustomerId: "c", Currency: "USD", Items: []*omsv1.OrderItem{nil}}},
		{name: "price required", req: &omsv1.CreateOrderRequest{CustomerId: "c", Currency: "USD", Items: []*omsv1.OrderItem{{Qty: 1}}}},
		{name: "currency mismatch", req: &omsv1.CreateOrderRequest{CustomerId: "c", Currency: "USD", Items: []*omsv1.OrderItem{{Qty: 1, Price: &omsv1.Money{Currency: "EUR", AmountMinor: 1}}}}},
		{name: "qty invalid", req: &omsv1.CreateOrderRequest{CustomerId: "c", Currency: "USD", Items: []*omsv1.OrderItem{{Qty: 0, Price: &omsv1.Money{Currency: "USD", AmountMinor: 1}}}}},
		{name: "price invalid", req: &omsv1.CreateOrderRequest{CustomerId: "c", Currency: "USD", Items: []*omsv1.OrderItem{{Qty: 1, Price: &omsv1.Money{Currency: "USD", AmountMinor: -1}}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.createOrderInternal(context.Background(), tt.req)
			if err == nil {
				t.Fatal("expected validation error")
			}
			mustStatusCode(t, err, codes.InvalidArgument)
		})
	}
}

func TestCreateOrderInternal_CreateErrorMapping(t *testing.T) {
	tests := []struct {
		name     string
		createFn func(domain.Order) error
		code     codes.Code
	}{
		{
			name: "version conflict",
			createFn: func(domain.Order) error {
				return domain.ErrOrderVersionConflict
			},
			code: codes.AlreadyExists,
		},
		{
			name: "internal error",
			createFn: func(domain.Order) error {
				return errors.New("db down")
			},
			code: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newInternalTestService(&stubOrderRepository{createFn: tt.createFn})
			_, err := service.createOrderInternal(context.Background(), validCreateRequest())
			if err == nil {
				t.Fatal("expected create error")
			}
			mustStatusCode(t, err, tt.code)
		})
	}
}

func TestListOrders_Branches(t *testing.T) {
	service := newInternalTestService(&stubOrderRepository{})

	_, err := service.ListOrders(context.Background(), nil)
	mustStatusCode(t, err, codes.InvalidArgument)

	_, err = service.ListOrders(context.Background(), &omsv1.ListOrdersRequest{})
	mustStatusCode(t, err, codes.InvalidArgument)

	var gotLimit int
	service = newInternalTestService(&stubOrderRepository{
		listFn: func(customerID string, limit int) ([]domain.Order, error) {
			gotLimit = limit
			return []domain.Order{
				{
					ID:          "order-1",
					CustomerID:  customerID,
					Status:      domain.OrderStatusPending,
					Currency:    "USD",
					AmountMinor: 100,
					Items:       []domain.OrderItem{{ID: "i1", SKU: "sku", Qty: 1, PriceMinor: 100, CreatedAt: time.Now().UTC()}},
					CreatedAt:   time.Now().UTC(),
					UpdatedAt:   time.Now().UTC(),
				},
			}, nil
		},
	})

	resp, err := service.ListOrders(context.Background(), &omsv1.ListOrdersRequest{CustomerId: "customer-1"})
	if err != nil {
		t.Fatalf("ListOrders returned error: %v", err)
	}
	if gotLimit != defaultListOrdersLimit {
		t.Fatalf("expected default limit %d, got %d", defaultListOrdersLimit, gotLimit)
	}
	if len(resp.Orders) != 1 {
		t.Fatalf("expected one order, got %d", len(resp.Orders))
	}

	service = newInternalTestService(&stubOrderRepository{
		listFn: func(string, int) ([]domain.Order, error) {
			return nil, errors.New("list failed")
		},
	})
	_, err = service.ListOrders(context.Background(), &omsv1.ListOrdersRequest{CustomerId: "customer-1", PageSize: 5})
	mustStatusCode(t, err, codes.Internal)
}

func TestLoadOrderAndSaveOrder_ErrorMappings(t *testing.T) {
	order := domain.Order{ID: "order-1"}

	service := newInternalTestService(&stubOrderRepository{
		getFn: func(string) (domain.Order, error) { return order, nil },
	})
	if _, err := service.loadOrder("order-1", "GetOrder"); err != nil {
		t.Fatalf("expected successful load, got %v", err)
	}

	service = newInternalTestService(&stubOrderRepository{
		getFn: func(string) (domain.Order, error) { return domain.Order{}, domain.ErrOrderNotFound },
	})
	_, err := service.loadOrder("missing", "GetOrder")
	mustStatusCode(t, err, codes.NotFound)

	service = newInternalTestService(&stubOrderRepository{
		getFn: func(string) (domain.Order, error) { return domain.Order{}, errors.New("db read failed") },
	})
	_, err = service.loadOrder("broken", "GetOrder")
	mustStatusCode(t, err, codes.Internal)

	saveCases := []struct {
		name string
		err  error
		code codes.Code
	}{
		{name: "not found", err: domain.ErrOrderNotFound, code: codes.NotFound},
		{name: "version conflict", err: domain.ErrOrderVersionConflict, code: codes.Aborted},
		{name: "internal", err: errors.New("db write failed"), code: codes.Internal},
	}

	for _, tc := range saveCases {
		t.Run(tc.name, func(t *testing.T) {
			service := newInternalTestService(&stubOrderRepository{
				saveFn: func(domain.Order) error { return tc.err },
			})
			err := service.saveOrder(order, "Save", "save failed")
			mustStatusCode(t, err, tc.code)
		})
	}

	service = newInternalTestService(&stubOrderRepository{
		saveFn: func(domain.Order) error { return nil },
	})
	if err := service.saveOrder(order, "Save", "save failed"); err != nil {
		t.Fatalf("expected save success, got %v", err)
	}
}

func TestIdempotencyFailureHelpers(t *testing.T) {
	var gotKey string
	var gotPayload []byte
	var gotStatus int

	idem := &stubIdempotencyRepository{
		markFailedFn: func(key string, payload []byte, statusCode int) error {
			gotKey = key
			gotPayload = append([]byte(nil), payload...)
			gotStatus = statusCode
			return nil
		},
	}

	service := NewOrderService(
		&stubOrderRepository{},
		&stubTimelineRepository{},
		idem,
		nil,
		log.New().WithField("test", "idempotency"),
	)

	service.cacheIdempotencyFailure("idem-1", status.Error(codes.FailedPrecondition, "failed before commit"))
	if gotKey != "idem-1" {
		t.Fatalf("expected key idem-1, got %s", gotKey)
	}
	if gotStatus != int(codes.FailedPrecondition) {
		t.Fatalf("expected code %d, got %d", int(codes.FailedPrecondition), gotStatus)
	}
	if len(gotPayload) == 0 {
		t.Fatal("expected non-empty payload")
	}

	service.idemRepo = &stubIdempotencyRepository{
		markFailedFn: func(string, []byte, int) error { return errors.New("store failed") },
	}
	service.cacheIdempotencyFailure("idem-2", nil)
}

func TestDecodeIdempotencyFailure_Branches(t *testing.T) {
	err := decodeIdempotencyFailure(domain.IdempotencyRecord{
		ResponseBody: []byte(`{"code":3,"message":"payload mismatch"}`),
	})
	mustStatusCode(t, err, codes.InvalidArgument)
	if status.Convert(err).Message() != "payload mismatch" {
		t.Fatalf("unexpected message: %s", status.Convert(err).Message())
	}

	err = decodeIdempotencyFailure(domain.IdempotencyRecord{
		ResponseBody: []byte(`{"code":0,"message":""}`),
	})
	mustStatusCode(t, err, codes.Internal)

	err = decodeIdempotencyFailure(domain.IdempotencyRecord{
		ResponseBody: []byte("broken-json"),
		HTTPStatus:   int(codes.Aborted),
	})
	mustStatusCode(t, err, codes.Aborted)

	err = decodeIdempotencyFailure(domain.IdempotencyRecord{
		ResponseBody: []byte("broken-json"),
		HTTPStatus:   int(codes.OK),
	})
	mustStatusCode(t, err, codes.Internal)
}

func TestUtilityHelpers(t *testing.T) {
	hash, err := buildIdempotencyRequestHash(grpcMethodCreateOrder, validCreateRequest())
	if err != nil {
		t.Fatalf("build hash failed: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}

	_, err = buildIdempotencyRequestHash(grpcMethodCreateOrder, nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}

	msg := joinErrors([]error{errors.New("first"), errors.New("second")})
	if msg != "first; second" {
		t.Fatalf("unexpected joined message: %q", msg)
	}

	msg = joinErrors(nil)
	if msg != "" {
		t.Fatalf("expected empty message, got %q", msg)
	}

	if toProtoStatus(domain.OrderStatus("something-else")) != omsv1.OrderStatus_ORDER_STATUS_UNSPECIFIED {
		t.Fatal("unknown status must map to ORDER_STATUS_UNSPECIFIED")
	}
}

func TestTimelineHelpersAndRunSagaAsync(t *testing.T) {
	logger := log.New().WithField("test", "timeline")
	service := NewOrderService(&stubOrderRepository{}, nil, nil, nil, logger)

	service.appendTimelineEvent("order-1", "evt", "reason")
	service.appendStatusTimeline("order-1", domain.OrderStatusPending, time.Time{})
	if got := service.buildTimeline("order-1"); got != nil {
		t.Fatalf("expected nil timeline when repository is nil, got %v", got)
	}

	service.timeline = &stubTimelineRepository{
		appendFn: func(domain.TimelineEvent) error { return errors.New("append failed") },
		listFn:   func(string) ([]domain.TimelineEvent, error) { return nil, errors.New("list failed") },
	}
	service.appendTimelineEvent("order-1", "evt", "reason")
	service.appendStatusTimeline("order-1", domain.OrderStatusPending, time.Now().UTC())
	if got := service.buildTimeline("order-1"); got != nil {
		t.Fatalf("expected nil on timeline list error, got %v", got)
	}

	service.timeline = &stubTimelineRepository{
		listFn: func(string) ([]domain.TimelineEvent, error) {
			return []domain.TimelineEvent{{Type: "OrderStatusChanged", Reason: "pending", Occurred: time.Unix(100, 0).UTC()}}, nil
		},
	}
	tl := service.buildTimeline("order-1")
	if len(tl) != 1 || tl[0].UnixTime != 100 {
		t.Fatalf("unexpected timeline response: %+v", tl)
	}

	service.sagaClosed = true
	called := false
	service.runSagaAsync("order-1", func() { called = true })
	if called {
		t.Fatal("saga must not start after shutdown")
	}
}
