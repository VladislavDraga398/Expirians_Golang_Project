package saga

import (
	"encoding/json"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/vladislavdragonenkov/oms/internal/domain"
	"github.com/vladislavdragonenkov/oms/internal/messaging/kafka"
	"github.com/vladislavdragonenkov/oms/internal/metrics"
)

// Orchestrator описывает интерфейс управления сагой.
type Orchestrator interface {
	Start(orderID string)
	Cancel(orderID, reason string)
	Refund(orderID string, amountMinor int64, reason string)
}

// orchestrator реализует последовательность шагов саги: Reserve → Pay → Confirm.
type orchestrator struct {
	orders        domain.OrderRepository
	outbox        domain.OutboxRepository
	timeline      domain.TimelineRepository
	inventory     domain.InventoryService
	payments      domain.PaymentService
	logger        *log.Entry
	metrics       *metrics.SagaMetrics
	kafkaProducer *kafka.Producer // опциональный Kafka producer для event-driven архитектуры
}

// NewOrchestrator создаёт рабочий экземпляр оркестратора.
func NewOrchestrator(
	orders domain.OrderRepository,
	outbox domain.OutboxRepository,
	timeline domain.TimelineRepository,
	inventory domain.InventoryService,
	payments domain.PaymentService,
	logger *log.Entry,
) Orchestrator {
	if logger == nil {
		logger = log.New().WithField("component", "saga")
	}
	return &orchestrator{
		orders:    orders,
		outbox:    outbox,
		timeline:  timeline,
		inventory: inventory,
		payments:  payments,
		logger:    logger,
		metrics:   metrics.NewSagaMetrics(),
	}
}

// NewOrchestratorWithKafka создаёт оркестратор с Kafka producer для event-driven архитектуры.
func NewOrchestratorWithKafka(
	orders domain.OrderRepository,
	outbox domain.OutboxRepository,
	timeline domain.TimelineRepository,
	inventory domain.InventoryService,
	payments domain.PaymentService,
	kafkaProducer *kafka.Producer,
	logger *log.Entry,
) Orchestrator {
	if logger == nil {
		logger = log.New().WithField("component", "saga")
	}
	return &orchestrator{
		orders:        orders,
		outbox:        outbox,
		timeline:      timeline,
		inventory:     inventory,
		payments:      payments,
		logger:        logger,
		metrics:       metrics.NewSagaMetrics(),
		kafkaProducer: kafkaProducer,
	}
}

// NewOrchestratorWithoutMetrics создаёт оркестратор без метрик (для тестов).
func NewOrchestratorWithoutMetrics(
	orders domain.OrderRepository,
	outbox domain.OutboxRepository,
	timeline domain.TimelineRepository,
	inventory domain.InventoryService,
	payments domain.PaymentService,
	logger *log.Entry,
) Orchestrator {
	if logger == nil {
		logger = log.New().WithField("component", "saga")
	}
	return &orchestrator{
		orders:    orders,
		outbox:    outbox,
		timeline:  timeline,
		inventory: inventory,
		payments:  payments,
		logger:    logger,
		metrics:   nil, // Отключаем метрики для тестов
	}
}

// Start запускает обработку заказа. Метод идемпотентен относительно конечных статусов.
func (o *orchestrator) Start(orderID string) {
	start := time.Now()
	if o.metrics != nil {
		o.metrics.RecordSagaStarted()
	}
	defer func() {
		if o.metrics != nil {
			o.metrics.RecordSagaDuration(time.Since(start))
		}
	}()

	order, err := o.orders.Get(orderID)
	if err != nil {
		o.logger.WithError(err).WithField("order_id", orderID).Warn("order not found for saga")
		if o.metrics != nil {
			o.metrics.RecordSagaFailed()
		}
		return
	}

	// Публикуем событие начала саги
	o.publishSagaEvent(kafka.EventTypeSagaStarted, orderID, map[string]interface{}{
		"customer_id": order.CustomerID,
		"status":      string(order.Status),
	})

	switch order.Status {
	case domain.OrderStatusPending:
		if err := o.handleReserve(&order); err != nil {
			return
		}
		fallthrough
	case domain.OrderStatusReserved:
		if err := o.handlePayment(&order); err != nil {
			return
		}
		fallthrough
	case domain.OrderStatusPaid:
		o.handleConfirm(&order)
	default:
		o.logger.WithFields(log.Fields{
			"order_id": order.ID,
		}).Debug("order already processed, skipping saga")
	}
}

func (o *orchestrator) handleReserve(order *domain.Order) error {
    if err := o.inventory.Reserve(order.ID, order.Items); err != nil {
        o.logger.WithError(err).WithField("order_id", order.ID).Warn("reserve failed")
        o.failOrder(order, domain.OrderStatusCanceled, err)
        return err
    }
    if err := o.updateStatus(order, domain.OrderStatusReserved); err != nil {
        return err
    }
    // Публикуем событие в Kafka
    o.publishSagaEvent(kafka.EventTypeStepReserved, order.ID, map[string]interface{}{
        "customer_id": order.CustomerID,
        "items_count": len(order.Items),
    })
    return nil
}

func (o *orchestrator) handlePayment(order *domain.Order) error {
    status, err := o.payments.Pay(order.ID, order.AmountMinor, order.Currency)
    if err != nil {
        o.logger.WithError(err).WithField("order_id", order.ID).Warn("payment failed")
        o.releaseInventory(order)
        o.failOrder(order, domain.OrderStatusCanceled, err)
        return err
    }
    if status != domain.PaymentStatusCaptured && status != domain.PaymentStatusAuthorized {
        o.logger.WithField("status", status).WithField("order_id", order.ID).Warn("unexpected payment status")
        o.releaseInventory(order)
        o.failOrder(order, domain.OrderStatusCanceled, domain.ErrPaymentIndeterminate)
        return domain.ErrPaymentIndeterminate
    }
    if err := o.updateStatus(order, domain.OrderStatusPaid); err != nil {
        return err
    }
    // Публикуем событие в Kafka
    o.publishSagaEvent(kafka.EventTypeStepPaid, order.ID, map[string]interface{}{
        "amount":   order.AmountMinor,
        "currency": order.Currency,
        "status":   string(status),
    })
    return nil
}

func (o *orchestrator) handleConfirm(order *domain.Order) {
    o.logger.WithField("order_id", order.ID).Debug("handleConfirm called")
    if err := o.updateStatus(order, domain.OrderStatusConfirmed); err != nil {
        o.logger.WithError(err).WithField("order_id", order.ID).Error("confirm failed")
        if o.metrics != nil {
            o.metrics.RecordSagaFailed()
        }
        return
    }
    o.logger.WithField("order_id", order.ID).Info("saga completed successfully")
    if o.metrics != nil {
        o.metrics.RecordSagaCompleted()
        o.logger.WithField("order_id", order.ID).Debug("RecordSagaCompleted called")
    }
    // Публикуем событие успешного завершения саги
    o.publishSagaEvent(kafka.EventTypeSagaCompleted, order.ID, map[string]interface{}{
        "customer_id": order.CustomerID,
        "amount":      order.AmountMinor,
    })
}

 

func (o *orchestrator) Cancel(orderID, reason string) {
    if o.metrics != nil {
        o.metrics.RecordSagaCanceled()
    }

    order, err := o.orders.Get(orderID)
    if err != nil {
        o.logger.WithError(err).WithField("order_id", orderID).Warn("order not found for cancel")
        if o.metrics != nil {
            o.metrics.RecordSagaFailed()
        }
        return
    }
    // Если заказ уже отменён или возвращён, ничего не делаем
    if order.Status == domain.OrderStatusCanceled || order.Status == domain.OrderStatusRefunded {
        o.logger.WithFields(log.Fields{
            "order_id": order.ID,
            "status":   order.Status,
        }).Debug("order already canceled or refunded")
        return
    }
    if order.Status == domain.OrderStatusReserved || order.Status == domain.OrderStatusPaid || order.Status == domain.OrderStatusConfirmed {
        // Освобождаем резерв инвентаря
        o.releaseInventory(&order)
    }
    if order.Status == domain.OrderStatusPaid || order.Status == domain.OrderStatusConfirmed {
        // Возвращаем средства
        if _, err := o.payments.Refund(order.ID, order.AmountMinor, order.Currency); err != nil {
            o.logger.WithError(err).WithField("order_id", order.ID).Warn("refund during cancel failed")
            if o.metrics != nil {
                o.metrics.RecordSagaFailed()
            }
            return
        }
    }
    if err := o.updateStatus(&order, domain.OrderStatusCanceled); err != nil {
        return
    }

    payload := map[string]interface{}{
        "reason": reason,
        "ts":     time.Now().UTC().Format(time.RFC3339Nano),
    }
    if reason == "" {
        delete(payload, "reason")
    }
    o.emitEvent(&order, "OrderCanceled", payload)
    
    // Публикуем событие отмены саги в Kafka
    o.publishSagaEvent(kafka.EventTypeSagaCanceled, order.ID, map[string]interface{}{
        "reason":      reason,
        "customer_id": order.CustomerID,
    })
}

// Refund инициирует возврат средств и переводит заказ в статус refunded.
func (o *orchestrator) Refund(orderID string, amountMinor int64, reason string) {
    if o.metrics != nil {
        o.metrics.RecordSagaRefunded()
    }

    order, err := o.orders.Get(orderID)
    if err != nil {
        o.logger.WithError(err).WithField("order_id", orderID).Warn("order not found for refund")
        return
    }

    if order.Status == domain.OrderStatusRefunded {
        o.logger.WithField("order_id", order.ID).Debug("order already refunded")
        return
    }

    if order.Status != domain.OrderStatusPaid && order.Status != domain.OrderStatusConfirmed {
        o.logger.WithFields(log.Fields{
            "order_id": order.ID,
            "status":   order.Status,
        }).Warn("refund skipped for order without payment")
        return
    }

    if amountMinor <= 0 || amountMinor > order.AmountMinor {
        amountMinor = order.AmountMinor
    }

    status, payErr := o.payments.Refund(order.ID, amountMinor, order.Currency)
    if payErr != nil {
        o.logger.WithError(payErr).WithField("order_id", order.ID).Warn("refund failed")
        return
    }
    if status != domain.PaymentStatusRefunded {
        o.logger.WithFields(log.Fields{
            "order_id": order.ID,
            "status":   status,
        }).Warn("unexpected refund status")
        return
    }

    o.releaseInventory(&order)
    if err := o.updateStatus(&order, domain.OrderStatusRefunded); err != nil {
        return
    }

    payload := map[string]interface{}{
        "amount_minor": amountMinor,
        "reason":       reason,
        "ts":           time.Now().UTC().Format(time.RFC3339Nano),
    }
    if reason == "" {
        delete(payload, "reason")
    }
    o.emitEvent(&order, "OrderRefunded", payload)
    
    // Публикуем событие возврата в Kafka
    o.publishSagaEvent(kafka.EventTypeSagaRefunded, order.ID, map[string]interface{}{
        "amount":      amountMinor,
        "reason":      reason,
        "customer_id": order.CustomerID,
    })
}

func (o *orchestrator) failOrder(order *domain.Order, status domain.OrderStatus, rootErr error) {
    if o.metrics != nil {
        o.metrics.RecordSagaFailed()
    }
    if err := o.updateStatus(order, status); err != nil {
        return
    }

    payload := map[string]interface{}{
        "reason": rootErr.Error(),
        "ts":     time.Now().UTC().Format(time.RFC3339Nano),
    }
    o.emitEvent(order, "OrderSagaFailed", payload)
    
    // Публикуем событие провала саги в Kafka
    o.publishSagaEvent(kafka.EventTypeSagaFailed, order.ID, map[string]interface{}{
        "reason":      rootErr.Error(),
        "customer_id": order.CustomerID,
        "status":      string(status),
    })
}

func (o *orchestrator) releaseInventory(order *domain.Order) {
    if err := o.inventory.Release(order.ID, order.Items); err != nil {
        o.logger.WithError(err).WithField("order_id", order.ID).Warn("release failed")
    }
}

// updateStatus меняет статус заказа и эмитит событие в timeline через emitStatusEvent.
// Реализует retry логику с exponential backoff для обработки version conflicts.
func (o *orchestrator) updateStatus(order *domain.Order, newStatus domain.OrderStatus) error {
    if order.Status == newStatus {
        return nil
    }
    
    const maxRetries = 3
    const baseDelay = 10 * time.Millisecond
    
    for attempt := 0; attempt < maxRetries; attempt++ {
        previousStatus := order.Status
        order.Status = newStatus
        order.UpdatedAt = time.Now().UTC()
        prevVersion := order.Version

        if err := o.orders.Save(*order); err != nil {
            // Проверяем, является ли ошибка version conflict
            if domain.IsVersionConflict(err) && attempt < maxRetries-1 {
                o.logger.WithFields(log.Fields{
                    "order_id": order.ID,
                    "attempt":  attempt + 1,
                    "version":  order.Version,
                }).Warn("version conflict detected, retrying")
                
                // Перезагружаем свежую версию заказа
                fresh, loadErr := o.orders.Get(order.ID)
                if loadErr != nil {
                    o.logger.WithError(loadErr).WithField("order_id", order.ID).Error("failed to reload order after conflict")
                    return loadErr
                }
                
                // Обновляем order с новыми данными
                *order = fresh
                
                // Exponential backoff
                delay := baseDelay * time.Duration(1<<uint(attempt))
                time.Sleep(delay)
                continue
            }
            
            // Если это не version conflict или исчерпаны попытки
            order.Status = previousStatus
            o.logger.WithError(err).WithFields(log.Fields{
                "order_id": order.ID,
                "attempt":  attempt + 1,
            }).Error("failed to persist status")
            return err
        }
        
        // Успешно сохранили
        order.Version = prevVersion + 1
        o.emitStatusEvent(order)
        return nil
    }
    
    // Если дошли сюда - все попытки исчерпаны
    return domain.ErrOrderVersionConflict
}

func (o *orchestrator) emitStatusEvent(order *domain.Order) {
    payload := map[string]interface{}{
        "status":     order.Status,
        "updated_at": order.UpdatedAt.Format(time.RFC3339Nano),
        "ts":         order.UpdatedAt.Format(time.RFC3339Nano),
    }
    o.emitEvent(order, "OrderStatusChanged", payload)
}

func (o *orchestrator) emitEvent(order *domain.Order, eventType string, payload map[string]interface{}) {
    if payload == nil {
        payload = make(map[string]interface{})
    }
    payload["order_id"] = order.ID
    data, err := json.Marshal(payload)
    if err != nil {
        o.logger.WithError(err).WithFields(log.Fields{
            "order_id": order.ID,
            "event":    eventType,
        }).Error("marshal event failed")
        return
    }

    msg := domain.OutboxMessage{
        AggregateType: "order",
        AggregateID:   order.ID,
        EventType:     eventType,
        Payload:       data,
    }
    if _, err := o.outbox.Enqueue(msg); err != nil {
        o.logger.WithError(err).WithFields(log.Fields{
            "order_id": order.ID,
            "event":    eventType,
        }).Error("enqueue event failed")
    } else if o.metrics != nil {
        o.metrics.RecordOutboxEvent()
    }

    if o.timeline != nil {
        var reason string
        if r, ok := payload["reason"].(string); ok {
            reason = r
        }
        var occurred time.Time
        if ts, ok := payload["ts"].(string); ok {
            if parsed, parseErr := time.Parse(time.RFC3339Nano, ts); parseErr == nil {
                occurred = parsed
            }
        }
        if occurred.IsZero() {
            if upd, ok := payload["updated_at"].(string); ok {
                if parsed, parseErr := time.Parse(time.RFC3339Nano, upd); parseErr == nil {
                    occurred = parsed
                }
            }
        }
        if occurred.IsZero() {
            occurred = time.Now().UTC()
        }
        event := domain.TimelineEvent{
            OrderID:  order.ID,
            Type:     eventType,
            Reason:   reason,
            Occurred: occurred,
        }
        if err := o.timeline.Append(event); err != nil {
            o.logger.WithError(err).WithFields(log.Fields{
                "order_id": order.ID,
                "event":    eventType,
            }).Warn("append timeline event failed")
        } else if o.metrics != nil {
            o.metrics.RecordTimelineEvent()
        }
    }
}

// publishSagaEvent публикует событие саги в Kafka (если producer настроен)
func (o *orchestrator) publishSagaEvent(eventType kafka.EventType, orderID string, metadata map[string]interface{}) {
	if o.kafkaProducer == nil {
		return // Kafka не настроен, пропускаем
	}

	event := kafka.NewSagaEvent(eventType, orderID, metadata)
	if err := o.kafkaProducer.PublishEvent(kafka.TopicSagaEvents, orderID, event); err != nil {
		// Логируем ошибку, но не прерываем saga - Kafka опциональный
		o.logger.WithError(err).WithFields(log.Fields{
			"event_type": eventType,
			"order_id":   orderID,
		}).Warn("failed to publish saga event to kafka")
	}
}

type noopOrchestrator struct {
    logger *log.Entry
}

func NewNoop(logger *log.Entry) Orchestrator {
	if logger == nil {
		logger = log.New().WithField("component", "saga-noop")
	}
	return &noopOrchestrator{logger: logger}
}

func (n *noopOrchestrator) Start(orderID string) {
	n.logger.WithFields(log.Fields{
		"order_id": orderID,
		"ts":       time.Now().UTC().Format(time.RFC3339Nano),
	}).Info("Saga orchestrator noop invoked")
}

func (n *noopOrchestrator) Cancel(orderID, reason string) {
	n.logger.WithFields(log.Fields{
		"order_id": orderID,
		"reason":   reason,
		"ts":       time.Now().UTC().Format(time.RFC3339Nano),
	}).Info("Saga orchestrator noop cancel")
}

func (n *noopOrchestrator) Refund(orderID string, amountMinor int64, reason string) {
	n.logger.WithFields(log.Fields{
		"order_id":     orderID,
		"amount_minor": amountMinor,
		"reason":       reason,
		"ts":           time.Now().UTC().Format(time.RFC3339Nano),
	}).Info("Saga orchestrator noop refund")
}

var _ Orchestrator = (*orchestrator)(nil)
var _ Orchestrator = (*noopOrchestrator)(nil)
