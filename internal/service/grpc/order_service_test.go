package grpcsvc_test

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/vladislavdragonenkov/oms/internal/domain"
	grpcsvc "github.com/vladislavdragonenkov/oms/internal/service/grpc"
	"github.com/vladislavdragonenkov/oms/internal/service/saga"
	"github.com/vladislavdragonenkov/oms/internal/storage/memory"
	omsv1 "github.com/vladislavdragonenkov/oms/proto/oms/v1"
)

const bufSize = 1024 * 1024

func idemCtx(key string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), "idempotency-key", key)
}

func newTestServer() (*grpc.ClientConn, func(), error) {
	listener := bufconn.Listen(bufSize)
	repo := memory.NewOrderRepository()
	logger := loggerForTests()
	orchestrator := saga.NewNoop(logger.WithField("layer", "saga"))
	service := grpcsvc.NewOrderService(repo, memory.NewTimelineRepository(), memory.NewIdempotencyRepository(), orchestrator, logger)

	server := grpc.NewServer()
	omsv1.RegisterOrderServiceServer(server, service)

	go func() {
		if err := server.Serve(listener); err != nil {
			logger.WithError(err).Error("grpc serve failed")
		}
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}

	//nolint:staticcheck // grpc.Dial is required for bufconn testing
	conn, err := grpc.Dial("bufnet", grpc.WithContextDialer(dialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		server.Stop()
		return nil, func() {}, err
	}

	cleanup := func() {
		_ = conn.Close()
		server.Stop()
	}

	return conn, cleanup, nil
}

func loggerForTests() *logrus.Entry {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{FullTimestamp: false, DisableTimestamp: true})
	logger.SetLevel(logrus.DebugLevel)
	return logger.WithField("component", "test")
}

type stubOrchestrator struct {
	mu       sync.Mutex
	started  []string
	canceled []string
	refunds  []struct {
		id     string
		amount int64
		reason string
	}
}

func (s *stubOrchestrator) Start(orderID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.started = append(s.started, orderID)
}

func (s *stubOrchestrator) Cancel(orderID, _ string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.canceled = append(s.canceled, orderID)
}

func (s *stubOrchestrator) Refund(orderID string, amountMinor int64, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refunds = append(s.refunds, struct {
		id     string
		amount int64
		reason string
	}{orderID, amountMinor, reason})
}

// Helper methods for safe reading in tests
func (s *stubOrchestrator) getStarted() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]string, len(s.started))
	copy(result, s.started)
	return result
}

func (s *stubOrchestrator) getCanceled() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]string, len(s.canceled))
	copy(result, s.canceled)
	return result
}

func (s *stubOrchestrator) getRefunds() []struct {
	id     string
	amount int64
	reason string
} {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]struct {
		id     string
		amount int64
		reason string
	}, len(s.refunds))
	copy(result, s.refunds)
	return result
}

type blockingOrchestrator struct {
	startCalled chan struct{}
	release     chan struct{}
}

func newBlockingOrchestrator() *blockingOrchestrator {
	return &blockingOrchestrator{
		startCalled: make(chan struct{}, 1),
		release:     make(chan struct{}),
	}
}

func (b *blockingOrchestrator) Start(string) {
	select {
	case b.startCalled <- struct{}{}:
	default:
	}
	<-b.release
}

func (b *blockingOrchestrator) Cancel(string, string) {}

func (b *blockingOrchestrator) Refund(string, int64, string) {}

func seedOrder(t *testing.T, repo domain.OrderRepository, status domain.OrderStatus) domain.Order {
	t.Helper()

	now := time.Now().UTC()
	order := domain.Order{
		ID:          "order-1",
		CustomerID:  "customer-1",
		Status:      status,
		Currency:    "USD",
		AmountMinor: 100,
		Items: []domain.OrderItem{{
			ID:         "item-1",
			SKU:        "sku-1",
			Qty:        1,
			PriceMinor: 100,
			CreatedAt:  now,
		}},
		Version:   0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	require.NoError(t, repo.Create(order))
	return order
}

func TestOrderService_CreateAndGet(t *testing.T) {
	conn, cleanup, err := newTestServer()
	require.NoError(t, err)
	defer cleanup()

	client := omsv1.NewOrderServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.CreateOrder(metadata.AppendToOutgoingContext(ctx, "idempotency-key", "create-order-1"), &omsv1.CreateOrderRequest{
		CustomerId: "customer-1",
		Currency:   "USD",
		Items: []*omsv1.OrderItem{
			{
				Sku: "sku-1",
				Qty: 2,
				Price: &omsv1.Money{
					Currency:    "USD",
					AmountMinor: 300,
				},
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.Order.Id)
	require.Equal(t, int64(600), resp.Order.Amount.AmountMinor)

	getResp, err := client.GetOrder(ctx, &omsv1.GetOrderRequest{OrderId: resp.Order.Id})
	require.NoError(t, err)
	require.NotNil(t, getResp)
	require.Equal(t, resp.Order.Id, getResp.Order.Id)
	require.Equal(t, resp.Order.Amount.AmountMinor, getResp.Order.Amount.AmountMinor)
}

func TestOrderService_CreateOrder_RequiresIdempotencyKey(t *testing.T) {
	repo := memory.NewOrderRepository()
	service := grpcsvc.NewOrderService(repo, memory.NewTimelineRepository(), memory.NewIdempotencyRepository(), nil, loggerForTests())

	_, err := service.CreateOrder(context.Background(), &omsv1.CreateOrderRequest{
		CustomerId: "customer-1",
		Currency:   "USD",
		Items: []*omsv1.OrderItem{
			{
				Sku: "sku-1",
				Qty: 1,
				Price: &omsv1.Money{
					Currency:    "USD",
					AmountMinor: 100,
				},
			},
		},
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestOrderService_CreateOrder_IdempotentReplay(t *testing.T) {
	repo := memory.NewOrderRepository()
	service := grpcsvc.NewOrderService(repo, memory.NewTimelineRepository(), memory.NewIdempotencyRepository(), nil, loggerForTests())

	req := &omsv1.CreateOrderRequest{
		CustomerId: "customer-1",
		Currency:   "USD",
		Items: []*omsv1.OrderItem{
			{
				Sku: "sku-1",
				Qty: 1,
				Price: &omsv1.Money{
					Currency:    "USD",
					AmountMinor: 100,
				},
			},
		},
	}

	first, err := service.CreateOrder(idemCtx("create-replay-1"), req)
	require.NoError(t, err)
	second, err := service.CreateOrder(idemCtx("create-replay-1"), req)
	require.NoError(t, err)

	require.Equal(t, first.Order.Id, second.Order.Id)

	orders, err := repo.ListByCustomer("customer-1", 10)
	require.NoError(t, err)
	require.Len(t, orders, 1)
}

func TestOrderService_CreateOrder_IdempotencyHashMismatch(t *testing.T) {
	repo := memory.NewOrderRepository()
	service := grpcsvc.NewOrderService(repo, memory.NewTimelineRepository(), memory.NewIdempotencyRepository(), nil, loggerForTests())

	_, err := service.CreateOrder(idemCtx("create-replay-2"), &omsv1.CreateOrderRequest{
		CustomerId: "customer-1",
		Currency:   "USD",
		Items: []*omsv1.OrderItem{
			{
				Sku: "sku-1",
				Qty: 1,
				Price: &omsv1.Money{
					Currency:    "USD",
					AmountMinor: 100,
				},
			},
		},
	})
	require.NoError(t, err)

	_, err = service.CreateOrder(idemCtx("create-replay-2"), &omsv1.CreateOrderRequest{
		CustomerId: "customer-1",
		Currency:   "USD",
		Items: []*omsv1.OrderItem{
			{
				Sku: "sku-2",
				Qty: 2,
				Price: &omsv1.Money{
					Currency:    "USD",
					AmountMinor: 100,
				},
			},
		},
	})
	require.Error(t, err)
	require.Equal(t, codes.AlreadyExists, status.Code(err))
}

func TestOrderService_PayOrder(t *testing.T) {
	repo := memory.NewOrderRepository()
	seedOrder(t, repo, domain.OrderStatusPending)
	stub := &stubOrchestrator{}
	service := grpcsvc.NewOrderService(repo, memory.NewTimelineRepository(), memory.NewIdempotencyRepository(), stub, loggerForTests())

	resp, err := service.PayOrder(idemCtx("pay-order-1"), &omsv1.PayOrderRequest{OrderId: "order-1"})
	require.NoError(t, err)
	require.Equal(t, "order-1", resp.OrderId)
	require.Equal(t, omsv1.OrderStatus_ORDER_STATUS_PENDING, resp.Status)

	// Даём время для асинхронного вызова
	time.Sleep(10 * time.Millisecond)
	require.Equal(t, []string{"order-1"}, stub.getStarted())
}

func TestOrderService_PayOrder_RejectsNonPendingStatus(t *testing.T) {
	repo := memory.NewOrderRepository()
	seedOrder(t, repo, domain.OrderStatusConfirmed)
	stub := &stubOrchestrator{}
	service := grpcsvc.NewOrderService(repo, memory.NewTimelineRepository(), memory.NewIdempotencyRepository(), stub, loggerForTests())

	_, err := service.PayOrder(idemCtx("pay-order-non-pending"), &omsv1.PayOrderRequest{OrderId: "order-1"})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	time.Sleep(10 * time.Millisecond)
	require.Empty(t, stub.getStarted())
}

func TestOrderService_CancelOrder(t *testing.T) {
	repo := memory.NewOrderRepository()
	seedOrder(t, repo, domain.OrderStatusReserved)
	service := grpcsvc.NewOrderService(repo, memory.NewTimelineRepository(), memory.NewIdempotencyRepository(), nil, loggerForTests())

	resp, err := service.CancelOrder(idemCtx("cancel-order-1"), &omsv1.CancelOrderRequest{OrderId: "order-1", Reason: "customer request"})
	require.NoError(t, err)
	require.Equal(t, omsv1.OrderStatus_ORDER_STATUS_CANCELED, resp.Status)

	stored, err := repo.Get("order-1")
	require.NoError(t, err)
	require.Equal(t, domain.OrderStatusCanceled, stored.Status)
	require.Equal(t, int64(1), stored.Version)
}

func TestOrderService_RefundOrder(t *testing.T) {
	repo := memory.NewOrderRepository()
	seedOrder(t, repo, domain.OrderStatusConfirmed)
	service := grpcsvc.NewOrderService(repo, memory.NewTimelineRepository(), memory.NewIdempotencyRepository(), nil, loggerForTests())

	resp, err := service.RefundOrder(idemCtx("refund-order-1"), &omsv1.RefundOrderRequest{OrderId: "order-1"})
	require.NoError(t, err)
	require.Equal(t, omsv1.OrderStatus_ORDER_STATUS_REFUNDED, resp.Status)

	stored, err := repo.Get("order-1")
	require.NoError(t, err)
	require.Equal(t, domain.OrderStatusRefunded, stored.Status)
	require.Equal(t, int64(1), stored.Version)
}

func TestOrderService_CancelOrder_TriggersSaga(t *testing.T) {
	repo := memory.NewOrderRepository()
	seedOrder(t, repo, domain.OrderStatusConfirmed)
	stub := &stubOrchestrator{}
	service := grpcsvc.NewOrderService(repo, memory.NewTimelineRepository(), memory.NewIdempotencyRepository(), stub, loggerForTests())

	_, err := service.CancelOrder(idemCtx("cancel-order-2"), &omsv1.CancelOrderRequest{OrderId: "order-1", Reason: "customer"})
	require.NoError(t, err)

	// Даём время для асинхронного вызова
	time.Sleep(10 * time.Millisecond)
	require.Equal(t, []string{"order-1"}, stub.getCanceled())
}

func TestOrderService_RefundOrder_TriggersSaga(t *testing.T) {
	repo := memory.NewOrderRepository()
	seedOrder(t, repo, domain.OrderStatusPaid)
	stub := &stubOrchestrator{}
	service := grpcsvc.NewOrderService(repo, memory.NewTimelineRepository(), memory.NewIdempotencyRepository(), stub, loggerForTests())

	_, err := service.RefundOrder(idemCtx("refund-order-2"), &omsv1.RefundOrderRequest{OrderId: "order-1", Amount: &omsv1.Money{AmountMinor: 50}})
	require.NoError(t, err)

	// Даём время для асинхронного вызова
	time.Sleep(10 * time.Millisecond)
	refunds := stub.getRefunds()
	require.Len(t, refunds, 1)
	require.Equal(t, "order-1", refunds[0].id)
	require.Equal(t, int64(50), refunds[0].amount)
}

func TestOrderService_GetOrder_WithTimeline(t *testing.T) {
	repo := memory.NewOrderRepository()
	timeline := memory.NewTimelineRepository()
	seedOrder(t, repo, domain.OrderStatusConfirmed)
	service := grpcsvc.NewOrderService(repo, timeline, memory.NewIdempotencyRepository(), nil, loggerForTests())

	// Добавляем события в timeline вручную для теста
	_ = timeline.Append(domain.TimelineEvent{
		OrderID:  "order-1",
		Type:     "OrderStatusChanged",
		Reason:   "pending",
		Occurred: time.Now().Add(-2 * time.Minute),
	})
	_ = timeline.Append(domain.TimelineEvent{
		OrderID:  "order-1",
		Type:     "OrderStatusChanged",
		Reason:   "confirmed",
		Occurred: time.Now().Add(-1 * time.Minute),
	})

	resp, err := service.GetOrder(context.Background(), &omsv1.GetOrderRequest{OrderId: "order-1"})
	require.NoError(t, err)
	require.NotNil(t, resp.Order)
	require.Equal(t, "order-1", resp.Order.Id)

	// Проверяем timeline
	require.NotNil(t, resp.Timeline)
	require.Len(t, resp.Timeline, 2)
	require.Equal(t, "OrderStatusChanged", resp.Timeline[0].Type)
	require.Equal(t, "pending", resp.Timeline[0].Reason)
	require.Equal(t, "OrderStatusChanged", resp.Timeline[1].Type)
	require.Equal(t, "confirmed", resp.Timeline[1].Reason)
}

func TestOrderService_Shutdown_WaitsForSaga(t *testing.T) {
	repo := memory.NewOrderRepository()
	seedOrder(t, repo, domain.OrderStatusPending)

	orch := newBlockingOrchestrator()
	service := grpcsvc.NewOrderService(repo, memory.NewTimelineRepository(), memory.NewIdempotencyRepository(), orch, loggerForTests())

	_, err := service.PayOrder(idemCtx("pay-order-shutdown"), &omsv1.PayOrderRequest{OrderId: "order-1"})
	require.NoError(t, err)

	select {
	case <-orch.startCalled:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected saga to start")
	}

	shutdownResult := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		shutdownResult <- service.Shutdown(ctx)
	}()

	select {
	case err := <-shutdownResult:
		t.Fatalf("shutdown should wait for saga completion, got early result: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(orch.release)

	select {
	case err := <-shutdownResult:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("shutdown did not finish after saga completion")
	}
}

func TestOrderService_Shutdown_SkipsNewSagaDispatch(t *testing.T) {
	repo := memory.NewOrderRepository()
	seedOrder(t, repo, domain.OrderStatusPending)

	stub := &stubOrchestrator{}
	service := grpcsvc.NewOrderService(repo, memory.NewTimelineRepository(), memory.NewIdempotencyRepository(), stub, loggerForTests())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, service.Shutdown(ctx))

	_, err := service.PayOrder(idemCtx("pay-order-shutdown-skip"), &omsv1.PayOrderRequest{OrderId: "order-1"})
	require.NoError(t, err)

	time.Sleep(20 * time.Millisecond)
	require.Empty(t, stub.getStarted())
}
