package grpcsvc

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/vladislavdragonenkov/oms/internal/domain"
	"github.com/vladislavdragonenkov/oms/internal/service/saga"
	omsv1 "github.com/vladislavdragonenkov/oms/proto/oms/v1"
)

// OrderService реализует gRPC API поверх доменного репозитория заказов.
type OrderService struct {
	omsv1.UnimplementedOrderServiceServer

	repo     domain.OrderRepository
	timeline domain.TimelineRepository
	logger   *log.Entry
	saga     saga.Orchestrator
}

// NewOrderService конструирует сервис с зависимостями.
func NewOrderService(repo domain.OrderRepository, timeline domain.TimelineRepository, orchestrator saga.Orchestrator, logger *log.Entry) *OrderService {
	if logger == nil {
		logger = log.New().WithField("component", "order-service")
	}
	return &OrderService{repo: repo, timeline: timeline, saga: orchestrator, logger: logger}
}

// CreateOrder создаёт заказ и запускает обработку.
func (s *OrderService) CreateOrder(ctx context.Context, req *omsv1.CreateOrderRequest) (*omsv1.CreateOrderResponse, error) {
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
		switch err {
		case domain.ErrOrderVersionConflict:
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

	order, err := s.repo.Get(req.OrderId)
	if err != nil {
		s.logger.WithError(err).Warn("failed to get order for PayOrder")
		switch err {
		case domain.ErrOrderNotFound:
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, "failed to load order")
		}
	}

	if s.saga != nil {
		go s.saga.Start(order.ID)
	}

	return &omsv1.PayOrderResponse{OrderId: order.ID, Status: toProtoStatus(order.Status)}, nil
}

// CancelOrder отменяет заказ или запускает компенсирующие действия.
func (s *OrderService) CancelOrder(ctx context.Context, req *omsv1.CancelOrderRequest) (*omsv1.CancelOrderResponse, error) {
	if req == nil || req.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	order, err := s.repo.Get(req.OrderId)
	if err != nil {
		s.logger.WithError(err).Warn("failed to get order for CancelOrder")
		switch err {
		case domain.ErrOrderNotFound:
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, "failed to load order")
		}
	}

	if s.saga != nil {
		go s.saga.Cancel(order.ID, req.Reason)
	} else if order.Status != domain.OrderStatusCanceled {
		order.Status = domain.OrderStatusCanceled
		order.UpdatedAt = time.Now().UTC()
		if err := s.repo.Save(order); err != nil {
			s.logger.WithError(err).WithField("order_id", order.ID).Error("failed to cancel order")
			switch err {
			case domain.ErrOrderNotFound:
				return nil, status.Error(codes.NotFound, err.Error())
			case domain.ErrOrderVersionConflict:
				return nil, status.Error(codes.Aborted, err.Error())
			default:
				return nil, status.Error(codes.Internal, "failed to cancel order")
			}
		}
		s.appendStatusTimeline(order.ID, order.Status, order.UpdatedAt)
		s.appendTimelineEvent(order.ID, "OrderCanceled", req.Reason)
	}

	updated, err := s.repo.Get(order.ID)
	if err != nil {
		s.logger.WithError(err).Error("failed to reload order after cancel")
		return nil, status.Error(codes.Internal, "failed to load order")
	}

	return &omsv1.CancelOrderResponse{OrderId: updated.ID, Status: toProtoStatus(updated.Status)}, nil
}

// RefundOrder инициирует возврат средств.
func (s *OrderService) RefundOrder(ctx context.Context, req *omsv1.RefundOrderRequest) (*omsv1.RefundOrderResponse, error) {
	if req == nil || req.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	order, err := s.repo.Get(req.OrderId)
	if err != nil {
		s.logger.WithError(err).Warn("failed to get order for RefundOrder")
		switch err {
		case domain.ErrOrderNotFound:
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, "failed to load order")
		}
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
		go s.saga.Refund(order.ID, amountMinor, req.Reason)
	} else {
		// Без saga просто меняем статус
		order.Status = domain.OrderStatusRefunded
		order.UpdatedAt = time.Now().UTC()
		if err := s.repo.Save(order); err != nil {
			s.logger.WithError(err).WithField("order_id", order.ID).Error("failed to refund order")
			switch err {
			case domain.ErrOrderNotFound:
				return nil, status.Error(codes.NotFound, err.Error())
			case domain.ErrOrderVersionConflict:
				return nil, status.Error(codes.Aborted, err.Error())
			default:
				return nil, status.Error(codes.Internal, "failed to refund order")
			}
		}
		s.appendStatusTimeline(order.ID, order.Status, order.UpdatedAt)
		s.appendTimelineEvent(order.ID, "OrderRefunded", req.Reason)
	}

	updated, err := s.repo.Get(order.ID)
	if err != nil {
		s.logger.WithError(err).Error("failed to reload order after refund")
		return nil, status.Error(codes.Internal, "failed to load order")
	}

	return &omsv1.RefundOrderResponse{OrderId: updated.ID, Status: toProtoStatus(updated.Status)}, nil
}

// GetOrder возвращает состояние заказа и таймлайн событий.
func (s *OrderService) GetOrder(ctx context.Context, req *omsv1.GetOrderRequest) (*omsv1.GetOrderResponse, error) {
	if req == nil || req.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	order, err := s.repo.Get(req.OrderId)
	if err != nil {
		s.logger.WithError(err).Warn("failed to get order")
		switch err {
		case domain.ErrOrderNotFound:
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, "failed to load order")
		}
	}

	return &omsv1.GetOrderResponse{
		Order:    toProtoOrder(order),
		Timeline: s.buildTimeline(order.ID),
	}, nil
}

// ListOrders возвращает заказы клиента.
func (s *OrderService) ListOrders(ctx context.Context, req *omsv1.ListOrdersRequest) (*omsv1.ListOrdersResponse, error) {
	if req == nil || req.CustomerId == "" {
		return nil, status.Error(codes.InvalidArgument, "customer_id is required")
	}

	limit := int(req.PageSize)
	if limit <= 0 {
		limit = 100
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
		Type:     "OrderStatusChanged",
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
