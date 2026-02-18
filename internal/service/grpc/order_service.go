package grpcsvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/vladislavdragonenkov/oms/internal/domain"
	"github.com/vladislavdragonenkov/oms/internal/service/saga"
	omsv1 "github.com/vladislavdragonenkov/oms/proto/oms/v1"
)

// OrderService реализует gRPC API поверх доменного репозитория заказов.
type OrderService struct {
	omsv1.UnimplementedOrderServiceServer

	repo     domain.OrderRepository
	timeline domain.TimelineRepository
	idemRepo domain.IdempotencyRepository
	logger   *log.Entry
	saga     saga.Orchestrator

	sagaMu     sync.Mutex
	sagaClosed bool
	sagaWG     sync.WaitGroup
}

const (
	grpcMethodCreateOrder = "/oms.v1.OrderService/CreateOrder"
	grpcMethodPayOrder    = "/oms.v1.OrderService/PayOrder"
	grpcMethodCancelOrder = "/oms.v1.OrderService/CancelOrder"
	grpcMethodRefundOrder = "/oms.v1.OrderService/RefundOrder"

	defaultListOrdersLimit = 100

	timelineEventOrderStatusChanged = "OrderStatusChanged"
	timelineEventOrderCanceled      = "OrderCanceled"
	timelineEventOrderRefunded      = "OrderRefunded"
)

// NewOrderService конструирует сервис с зависимостями.
func NewOrderService(
	repo domain.OrderRepository,
	timeline domain.TimelineRepository,
	idemRepo domain.IdempotencyRepository,
	orchestrator saga.Orchestrator,
	logger *log.Entry,
) *OrderService {
	if logger == nil {
		logger = log.New().WithField("component", "order-service")
	}
	return &OrderService{
		repo:     repo,
		timeline: timeline,
		idemRepo: idemRepo,
		saga:     orchestrator,
		logger:   logger,
	}
}

// CreateOrder создаёт заказ и запускает обработку.
func (s *OrderService) CreateOrder(ctx context.Context, req *omsv1.CreateOrderRequest) (*omsv1.CreateOrderResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	return withIdempotency(
		s,
		ctx,
		grpcMethodCreateOrder,
		req,
		func() *omsv1.CreateOrderResponse { return &omsv1.CreateOrderResponse{} },
		func(ctx context.Context) (*omsv1.CreateOrderResponse, error) {
			return s.createOrderInternal(ctx, req)
		},
	)
}

func (s *OrderService) createOrderInternal(_ context.Context, req *omsv1.CreateOrderRequest) (*omsv1.CreateOrderResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.CustomerId == "" {
		return nil, status.Error(codes.InvalidArgument, "customer_id is required")
	}
	if req.Currency == "" {
		return nil, status.Error(codes.InvalidArgument, "currency is required")
	}
	if len(req.Items) == 0 {
		return nil, status.Error(codes.InvalidArgument, "order must contain at least one item")
	}

	now := time.Now().UTC()
	items := make([]domain.OrderItem, 0, len(req.Items))
	var amountSum int64
	for idx, item := range req.Items {
		if item == nil {
			return nil, status.Errorf(codes.InvalidArgument, "item[%d] is nil", idx)
		}
		if item.Price == nil {
			return nil, status.Errorf(codes.InvalidArgument, "item[%d].price is required", idx)
		}
		if item.Price.Currency != req.Currency {
			return nil, status.Errorf(codes.InvalidArgument, "item[%d].price.currency mismatch", idx)
		}
		if item.Qty <= 0 {
			return nil, status.Errorf(codes.InvalidArgument, "item[%d].qty must be > 0", idx)
		}
		if item.Price.AmountMinor < 0 {
			return nil, status.Errorf(codes.InvalidArgument, "item[%d].price.amount must be >= 0", idx)
		}

		items = append(items, domain.OrderItem{
			ID:         uuid.NewString(),
			SKU:        item.Sku,
			Qty:        item.Qty,
			PriceMinor: item.Price.AmountMinor,
			CreatedAt:  now,
		})
		amountSum += int64(item.Qty) * item.Price.AmountMinor
	}

	order := domain.Order{
		ID:          uuid.NewString(),
		CustomerID:  req.CustomerId,
		Status:      domain.OrderStatusPending,
		Currency:    req.Currency,
		AmountMinor: amountSum,
		Items:       items,
		Version:     0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if errs := order.ValidateInvariants(); len(errs) > 0 {
		return nil, status.Error(codes.InvalidArgument, joinErrors(errs))
	}

	if err := s.repo.Create(order); err != nil {
		s.logger.WithError(err).Error("failed to create order")
		switch {
		case errors.Is(err, domain.ErrOrderVersionConflict):
			return nil, status.Error(codes.AlreadyExists, err.Error())
		default:
			return nil, status.Error(codes.Internal, "failed to persist order")
		}
	}

	// Запишем начальное событие статуса в timeline
	s.appendStatusTimeline(order.ID, order.Status, order.UpdatedAt)

	return &omsv1.CreateOrderResponse{Order: toProtoOrder(order)}, nil
}

// PayOrder инициирует платежную стадию.
func (s *OrderService) PayOrder(ctx context.Context, req *omsv1.PayOrderRequest) (*omsv1.PayOrderResponse, error) {
	if req == nil || req.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	return withIdempotency(
		s,
		ctx,
		grpcMethodPayOrder,
		req,
		func() *omsv1.PayOrderResponse { return &omsv1.PayOrderResponse{} },
		func(ctx context.Context) (*omsv1.PayOrderResponse, error) {
			return s.payOrderInternal(ctx, req)
		},
	)
}

func (s *OrderService) payOrderInternal(_ context.Context, req *omsv1.PayOrderRequest) (*omsv1.PayOrderResponse, error) {
	if req == nil || req.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	order, err := s.loadOrder(req.OrderId, "PayOrder")
	if err != nil {
		return nil, err
	}

	if s.saga != nil {
		s.runSagaAsync(order.ID, func() {
			s.saga.Start(order.ID)
		})
	}

	return &omsv1.PayOrderResponse{OrderId: order.ID, Status: toProtoStatus(order.Status)}, nil
}

// CancelOrder отменяет заказ или запускает компенсирующие действия.
func (s *OrderService) CancelOrder(ctx context.Context, req *omsv1.CancelOrderRequest) (*omsv1.CancelOrderResponse, error) {
	if req == nil || req.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	return withIdempotency(
		s,
		ctx,
		grpcMethodCancelOrder,
		req,
		func() *omsv1.CancelOrderResponse { return &omsv1.CancelOrderResponse{} },
		func(ctx context.Context) (*omsv1.CancelOrderResponse, error) {
			return s.cancelOrderInternal(ctx, req)
		},
	)
}

func (s *OrderService) cancelOrderInternal(_ context.Context, req *omsv1.CancelOrderRequest) (*omsv1.CancelOrderResponse, error) {
	if req == nil || req.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	order, err := s.loadOrder(req.OrderId, "CancelOrder")
	if err != nil {
		return nil, err
	}

	if s.saga != nil {
		s.runSagaAsync(order.ID, func() {
			s.saga.Cancel(order.ID, req.Reason)
		})
	} else if order.Status != domain.OrderStatusCanceled {
		order.Status = domain.OrderStatusCanceled
		order.UpdatedAt = time.Now().UTC()
		if err := s.saveOrder(order, "CancelOrder", "failed to cancel order"); err != nil {
			return nil, err
		}
		s.appendStatusTimeline(order.ID, order.Status, order.UpdatedAt)
		s.appendTimelineEvent(order.ID, timelineEventOrderCanceled, req.Reason)
	}

	updated, err := s.loadOrder(order.ID, "CancelOrderReload")
	if err != nil {
		return nil, err
	}

	return &omsv1.CancelOrderResponse{OrderId: updated.ID, Status: toProtoStatus(updated.Status)}, nil
}

// RefundOrder инициирует возврат средств.
func (s *OrderService) RefundOrder(ctx context.Context, req *omsv1.RefundOrderRequest) (*omsv1.RefundOrderResponse, error) {
	if req == nil || req.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	return withIdempotency(
		s,
		ctx,
		grpcMethodRefundOrder,
		req,
		func() *omsv1.RefundOrderResponse { return &omsv1.RefundOrderResponse{} },
		func(ctx context.Context) (*omsv1.RefundOrderResponse, error) {
			return s.refundOrderInternal(ctx, req)
		},
	)
}

func (s *OrderService) refundOrderInternal(_ context.Context, req *omsv1.RefundOrderRequest) (*omsv1.RefundOrderResponse, error) {
	if req == nil || req.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	order, err := s.loadOrder(req.OrderId, "RefundOrder")
	if err != nil {
		return nil, err
	}

	var amountMinor int64
	if req.Amount != nil {
		if req.Amount.AmountMinor < 0 {
			return nil, status.Error(codes.InvalidArgument, "amount.amount_minor must be >= 0")
		}
		if req.Amount.Currency != "" && req.Amount.Currency != order.Currency {
			return nil, status.Error(codes.InvalidArgument, "amount.currency must match order currency")
		}
		amountMinor = req.Amount.AmountMinor
	}

	if order.Status != domain.OrderStatusPaid && order.Status != domain.OrderStatusConfirmed {
		return nil, status.Error(codes.FailedPrecondition, "order is not eligible for refund")
	}

	if s.saga != nil {
		s.runSagaAsync(order.ID, func() {
			s.saga.Refund(order.ID, amountMinor, req.Reason)
		})
	} else {
		// Без saga просто меняем статус
		order.Status = domain.OrderStatusRefunded
		order.UpdatedAt = time.Now().UTC()
		if err := s.saveOrder(order, "RefundOrder", "failed to refund order"); err != nil {
			return nil, err
		}
		s.appendStatusTimeline(order.ID, order.Status, order.UpdatedAt)
		s.appendTimelineEvent(order.ID, timelineEventOrderRefunded, req.Reason)
	}

	updated, err := s.loadOrder(order.ID, "RefundOrderReload")
	if err != nil {
		return nil, err
	}

	return &omsv1.RefundOrderResponse{OrderId: updated.ID, Status: toProtoStatus(updated.Status)}, nil
}

// GetOrder возвращает состояние заказа и таймлайн событий.
func (s *OrderService) GetOrder(_ context.Context, req *omsv1.GetOrderRequest) (*omsv1.GetOrderResponse, error) {
	if req == nil || req.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	order, err := s.loadOrder(req.OrderId, "GetOrder")
	if err != nil {
		return nil, err
	}

	return &omsv1.GetOrderResponse{
		Order:    toProtoOrder(order),
		Timeline: s.buildTimeline(order.ID),
	}, nil
}

// ListOrders возвращает заказы клиента.
func (s *OrderService) ListOrders(_ context.Context, req *omsv1.ListOrdersRequest) (*omsv1.ListOrdersResponse, error) {
	if req == nil || req.CustomerId == "" {
		return nil, status.Error(codes.InvalidArgument, "customer_id is required")
	}

	limit := int(req.PageSize)
	if limit <= 0 {
		limit = defaultListOrdersLimit
	}

	orders, err := s.repo.ListByCustomer(req.CustomerId, limit)
	if err != nil {
		s.logger.WithError(err).Error("failed to list orders")
		return nil, status.Error(codes.Internal, "failed to list orders")
	}

	result := make([]*omsv1.Order, 0, len(orders))
	for _, order := range orders {
		result = append(result, toProtoOrder(order))
	}

	return &omsv1.ListOrdersResponse{Orders: result}, nil
}

func (s *OrderService) loadOrder(orderID, operation string) (domain.Order, error) {
	order, err := s.repo.Get(orderID)
	if err == nil {
		return order, nil
	}

	s.logger.WithError(err).WithFields(log.Fields{
		"operation": operation,
		"order_id":  orderID,
	}).Warn("failed to load order")

	switch {
	case errors.Is(err, domain.ErrOrderNotFound):
		return domain.Order{}, status.Error(codes.NotFound, domain.ErrOrderNotFound.Error())
	default:
		return domain.Order{}, status.Error(codes.Internal, "failed to load order")
	}
}

func (s *OrderService) saveOrder(order domain.Order, operation, internalMsg string) error {
	if err := s.repo.Save(order); err != nil {
		s.logger.WithError(err).WithFields(log.Fields{
			"operation": operation,
			"order_id":  order.ID,
		}).Error("failed to save order")

		switch {
		case errors.Is(err, domain.ErrOrderNotFound):
			return status.Error(codes.NotFound, domain.ErrOrderNotFound.Error())
		case errors.Is(err, domain.ErrOrderVersionConflict):
			return status.Error(codes.Aborted, domain.ErrOrderVersionConflict.Error())
		default:
			return status.Error(codes.Internal, internalMsg)
		}
	}

	return nil
}

const (
	idempotencyKeyHeader = "idempotency-key"
	idempotencyTTL       = 24 * time.Hour
)

type idempotencyErrorPayload struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
}

func withIdempotency[T proto.Message](
	s *OrderService,
	ctx context.Context,
	method string,
	req proto.Message,
	newResp func() T,
	handler func(context.Context) (T, error),
) (T, error) {
	var zero T

	if s.idemRepo == nil {
		return handler(ctx)
	}

	idemKey, err := readIdempotencyKey(ctx)
	if err != nil {
		return zero, err
	}

	reqHash, err := buildIdempotencyRequestHash(method, req)
	if err != nil {
		s.logger.WithError(err).WithField("method", method).Warn("failed to build idempotency request hash")
		return zero, status.Error(codes.Internal, "failed to initialize idempotency request")
	}

	record, err := s.idemRepo.CreateProcessing(idemKey, reqHash, time.Now().UTC().Add(idempotencyTTL))
	if err != nil {
		return replayIdempotency(s, err, record, newResp)
	}

	resp, runErr := handler(ctx)
	if runErr != nil {
		s.cacheIdempotencyFailure(idemKey, runErr)
		return resp, runErr
	}

	if cacheErr := s.cacheIdempotencySuccess(idemKey, resp); cacheErr != nil {
		s.logger.WithError(cacheErr).WithField("idempotency_key", idemKey).Warn("failed to store idempotent success response")
	}

	return resp, nil
}

func replayIdempotency[T proto.Message](
	s *OrderService,
	createErr error,
	record domain.IdempotencyRecord,
	newResp func() T,
) (T, error) {
	var zero T

	switch {
	case errors.Is(createErr, domain.ErrIdempotencyHashMismatch):
		return zero, status.Error(codes.AlreadyExists, "idempotency key is already used with different request payload")
	case errors.Is(createErr, domain.ErrIdempotencyKeyAlreadyExists):
		switch record.Status {
		case domain.IdempotencyStatusDone:
			if len(record.ResponseBody) == 0 {
				return zero, status.Error(codes.Internal, "idempotency cache is empty")
			}
			resp := newResp()
			if err := protojson.Unmarshal(record.ResponseBody, resp); err != nil {
				s.logger.WithError(err).WithField("idempotency_key", record.Key).Warn("failed to decode cached idempotency response")
				return zero, status.Error(codes.Internal, "failed to decode cached idempotency response")
			}
			return resp, nil
		case domain.IdempotencyStatusProcessing:
			return zero, status.Error(codes.Aborted, "request with the same idempotency key is already processing")
		case domain.IdempotencyStatusFailed:
			return zero, decodeIdempotencyFailure(record)
		default:
			return zero, status.Error(codes.Internal, "unknown idempotency record status")
		}
	default:
		s.logger.WithError(createErr).Warn("failed to create idempotency record")
		return zero, status.Error(codes.Internal, "failed to initialize idempotency request")
	}
}

func (s *OrderService) cacheIdempotencySuccess(key string, resp proto.Message) error {
	if resp == nil {
		return s.idemRepo.MarkDone(key, nil, int(codes.OK))
	}

	data, err := protojson.Marshal(resp)
	if err != nil {
		return err
	}
	return s.idemRepo.MarkDone(key, data, int(codes.OK))
}

func (s *OrderService) cacheIdempotencyFailure(key string, runErr error) {
	st := status.Convert(runErr)
	code := st.Code()
	if code == codes.OK {
		code = codes.Internal
	}

	payload, err := json.Marshal(idempotencyErrorPayload{
		Code:    int32(code), //nolint:gosec // codes.Code is a bounded enum value.
		Message: st.Message(),
	})
	if err != nil {
		s.logger.WithError(err).WithField("idempotency_key", key).Warn("failed to encode idempotency failure payload")
		payload = nil
	}

	if err := s.idemRepo.MarkFailed(key, payload, int(code)); err != nil {
		s.logger.WithError(err).WithField("idempotency_key", key).Warn("failed to store idempotency failure response")
	}
}

func decodeIdempotencyFailure(record domain.IdempotencyRecord) error {
	if len(record.ResponseBody) > 0 {
		var payload idempotencyErrorPayload
		if err := json.Unmarshal(record.ResponseBody, &payload); err == nil {
			if code, ok := grpcCodeFromInt32(payload.Code); ok {
				if code == codes.OK {
					code = codes.Internal
				}
				if payload.Message == "" {
					payload.Message = "previous request with the same idempotency key failed"
				}
				return status.Error(code, payload.Message)
			}
		}
	}

	if record.HTTPStatus > 0 {
		if code, ok := grpcCodeFromInt(record.HTTPStatus); ok && code != codes.OK {
			return status.Error(code, "previous request with the same idempotency key failed")
		}
	}

	return status.Error(codes.Internal, "previous request with the same idempotency key failed")
}

func grpcCodeFromInt32(value int32) (codes.Code, bool) {
	if value < int32(codes.OK) || value > int32(codes.Unauthenticated) {
		return codes.Internal, false
	}
	return codes.Code(uint32(value)), true
}

func grpcCodeFromInt(value int) (codes.Code, bool) {
	if value < int(codes.OK) || value > int(codes.Unauthenticated) {
		return codes.Internal, false
	}
	return codes.Code(uint32(value)), true
}

func readIdempotencyKey(ctx context.Context) (string, error) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		values := md.Get(idempotencyKeyHeader)
		if len(values) > 0 && strings.TrimSpace(values[0]) != "" {
			return strings.TrimSpace(values[0]), nil
		}
	}

	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		values := md.Get(idempotencyKeyHeader)
		if len(values) > 0 && strings.TrimSpace(values[0]) != "" {
			return strings.TrimSpace(values[0]), nil
		}
	}

	return "", status.Error(codes.InvalidArgument, "idempotency-key metadata is required")
}

func buildIdempotencyRequestHash(method string, req proto.Message) (string, error) {
	if req == nil {
		return "", fmt.Errorf("request is nil")
	}

	data, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	if err != nil {
		return "", err
	}

	payload := make([]byte, 0, len(method)+1+len(data))
	payload = append(payload, method...)
	payload = append(payload, ':')
	payload = append(payload, data...)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func toProtoOrder(order domain.Order) *omsv1.Order {
	items := make([]*omsv1.OrderItem, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, &omsv1.OrderItem{
			Sku: item.SKU,
			Qty: item.Qty,
			Price: &omsv1.Money{
				Currency:    order.Currency,
				AmountMinor: item.PriceMinor,
			},
		})
	}

	return &omsv1.Order{
		Id:         order.ID,
		CustomerId: order.CustomerID,
		Status:     toProtoStatus(order.Status),
		Amount: &omsv1.Money{
			Currency:    order.Currency,
			AmountMinor: order.AmountMinor,
		},
		Items:    items,
		Version:  order.Version,
		Currency: order.Currency,
	}
}

func toProtoStatus(status domain.OrderStatus) omsv1.OrderStatus {
	switch status {
	case domain.OrderStatusPending:
		return omsv1.OrderStatus_ORDER_STATUS_PENDING
	case domain.OrderStatusReserved:
		return omsv1.OrderStatus_ORDER_STATUS_RESERVED
	case domain.OrderStatusPaid:
		return omsv1.OrderStatus_ORDER_STATUS_PAID
	case domain.OrderStatusConfirmed:
		return omsv1.OrderStatus_ORDER_STATUS_CONFIRMED
	case domain.OrderStatusCanceled:
		return omsv1.OrderStatus_ORDER_STATUS_CANCELED
	case domain.OrderStatusRefunded:
		return omsv1.OrderStatus_ORDER_STATUS_REFUNDED
	default:
		return omsv1.OrderStatus_ORDER_STATUS_UNSPECIFIED
	}
}

func joinErrors(errs []error) string {
	builder := strings.Builder{}
	for i, err := range errs {
		builder.WriteString(err.Error())
		if i < len(errs)-1 {
			builder.WriteString("; ")
		}
	}
	return builder.String()
}

func (s *OrderService) appendTimelineEvent(orderID, eventType, reason string) {
	if s.timeline == nil {
		return
	}
	event := domain.TimelineEvent{
		OrderID:  orderID,
		Type:     eventType,
		Reason:   reason,
		Occurred: time.Now().UTC(),
	}
	if err := s.timeline.Append(event); err != nil {
		s.logger.WithError(err).WithFields(log.Fields{
			"order_id": orderID,
			"event":    eventType,
		}).Warn("failed to append timeline event")
	}
}

func (s *OrderService) appendStatusTimeline(orderID string, status domain.OrderStatus, occurred time.Time) {
	if s.timeline == nil {
		return
	}
	if occurred.IsZero() {
		occurred = time.Now().UTC()
	}
	event := domain.TimelineEvent{
		OrderID:  orderID,
		Type:     timelineEventOrderStatusChanged,
		Reason:   string(status),
		Occurred: occurred,
	}
	if err := s.timeline.Append(event); err != nil {
		s.logger.WithError(err).WithField("order_id", orderID).Warn("failed to append status timeline")
	}
}

func (s *OrderService) buildTimeline(orderID string) []*omsv1.TimelineEvent {
	if s.timeline == nil {
		return nil
	}
	events, err := s.timeline.List(orderID)
	if err != nil {
		s.logger.WithError(err).WithField("order_id", orderID).Warn("failed to list timeline events")
		return nil
	}
	result := make([]*omsv1.TimelineEvent, 0, len(events))
	for _, event := range events {
		result = append(result, &omsv1.TimelineEvent{
			Type:     event.Type,
			Reason:   event.Reason,
			UnixTime: event.Occurred.Unix(),
		})
	}
	return result
}

// Shutdown ожидает завершения фоновых saga-задач.
func (s *OrderService) Shutdown(ctx context.Context) error {
	s.sagaMu.Lock()
	s.sagaClosed = true
	s.sagaMu.Unlock()

	waitDone := make(chan struct{})
	go func() {
		s.sagaWG.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *OrderService) runSagaAsync(orderID string, fn func()) {
	s.sagaMu.Lock()
	if s.sagaClosed {
		s.sagaMu.Unlock()
		s.logger.WithField("order_id", orderID).Warn("saga dispatch skipped during shutdown")
		return
	}
	s.sagaWG.Add(1)
	s.sagaMu.Unlock()

	go func() {
		defer s.sagaWG.Done()
		fn()
	}()
}
