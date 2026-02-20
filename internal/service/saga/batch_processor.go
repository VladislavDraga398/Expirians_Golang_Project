package saga

import (
	"context"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// BatchProcessor обрабатывает saga операции пакетами для повышения производительности.
type BatchProcessor struct {
	orchestrator Orchestrator
	logger       *log.Entry

	// Конфигурация батчинга
	batchSize      int
	flushTimeout   time.Duration
	maxParallelOps int

	// Внутренние каналы и состояние
	startCh  chan string
	cancelCh chan cancelRequest
	refundCh chan refundRequest
	stopCh   chan struct{}
	wg       sync.WaitGroup

	// Буферы для батчинга
	startBatch  []string
	cancelBatch []cancelRequest
	refundBatch []refundRequest
	mu          sync.Mutex
}

type cancelRequest struct {
	orderID string
	reason  string
}

type refundRequest struct {
	orderID     string
	amountMinor int64
	reason      string
}

// NewBatchProcessor создаёт новый батч-процессор.
func NewBatchProcessor(orchestrator Orchestrator, logger *log.Entry) *BatchProcessor {
	if logger == nil {
		logger = log.New().WithField("component", "batch-processor")
	}

	return &BatchProcessor{
		orchestrator:   orchestrator,
		logger:         logger,
		batchSize:      10,                     // Обрабатываем по 10 операций за раз
		flushTimeout:   100 * time.Millisecond, // Или каждые 100мс
		maxParallelOps: 8,
		startCh:        make(chan string, 100),
		cancelCh:       make(chan cancelRequest, 100),
		refundCh:       make(chan refundRequest, 100),
		stopCh:         make(chan struct{}),
	}
}

// Start запускает батч-процессор.
func (bp *BatchProcessor) Start(ctx context.Context) {
	bp.wg.Add(3)

	// Запускаем воркеры для каждого типа операций
	go bp.processStartBatch(ctx)
	go bp.processCancelBatch(ctx)
	go bp.processRefundBatch(ctx)

	bp.logger.Info("Batch processor started")
}

// Stop останавливает батч-процессор.
func (bp *BatchProcessor) Stop() {
	close(bp.stopCh)
	bp.wg.Wait()
	bp.logger.Info("Batch processor stopped")
}

// StartOrder добавляет заказ в очередь на обработку.
func (bp *BatchProcessor) StartOrder(orderID string) {
	select {
	case bp.startCh <- orderID:
	default:
		// Если канал переполнен, обрабатываем синхронно
		bp.logger.WithField("order_id", orderID).Warn("Start channel full, processing synchronously")
		bp.orchestrator.Start(orderID)
	}
}

// CancelOrder добавляет заказ в очередь на отмену.
func (bp *BatchProcessor) CancelOrder(orderID, reason string) {
	select {
	case bp.cancelCh <- cancelRequest{orderID: orderID, reason: reason}:
	default:
		bp.logger.WithField("order_id", orderID).Warn("Cancel channel full, processing synchronously")
		bp.orchestrator.Cancel(orderID, reason)
	}
}

// RefundOrder добавляет заказ в очередь на возврат.
func (bp *BatchProcessor) RefundOrder(orderID string, amountMinor int64, reason string) {
	select {
	case bp.refundCh <- refundRequest{orderID: orderID, amountMinor: amountMinor, reason: reason}:
	default:
		bp.logger.WithField("order_id", orderID).Warn("Refund channel full, processing synchronously")
		bp.orchestrator.Refund(orderID, amountMinor, reason)
	}
}

func (bp *BatchProcessor) processStartBatch(ctx context.Context) {
	defer bp.wg.Done()

	ticker := time.NewTicker(bp.flushTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			bp.flushStartBatch()
			return
		case <-bp.stopCh:
			bp.flushStartBatch()
			return
		case orderID := <-bp.startCh:
			bp.mu.Lock()
			bp.startBatch = append(bp.startBatch, orderID)
			shouldFlush := len(bp.startBatch) >= bp.batchSize
			bp.mu.Unlock()

			if shouldFlush {
				bp.flushStartBatch()
			}
		case <-ticker.C:
			bp.flushStartBatch()
		}
	}
}

func (bp *BatchProcessor) processCancelBatch(ctx context.Context) {
	defer bp.wg.Done()

	ticker := time.NewTicker(bp.flushTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			bp.flushCancelBatch()
			return
		case <-bp.stopCh:
			bp.flushCancelBatch()
			return
		case req := <-bp.cancelCh:
			bp.mu.Lock()
			bp.cancelBatch = append(bp.cancelBatch, req)
			shouldFlush := len(bp.cancelBatch) >= bp.batchSize
			bp.mu.Unlock()

			if shouldFlush {
				bp.flushCancelBatch()
			}
		case <-ticker.C:
			bp.flushCancelBatch()
		}
	}
}

func (bp *BatchProcessor) processRefundBatch(ctx context.Context) {
	defer bp.wg.Done()

	ticker := time.NewTicker(bp.flushTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			bp.flushRefundBatch()
			return
		case <-bp.stopCh:
			bp.flushRefundBatch()
			return
		case req := <-bp.refundCh:
			bp.mu.Lock()
			bp.refundBatch = append(bp.refundBatch, req)
			shouldFlush := len(bp.refundBatch) >= bp.batchSize
			bp.mu.Unlock()

			if shouldFlush {
				bp.flushRefundBatch()
			}
		case <-ticker.C:
			bp.flushRefundBatch()
		}
	}
}

func (bp *BatchProcessor) flushStartBatch() {
	bp.mu.Lock()
	batch := bp.startBatch
	bp.startBatch = nil
	bp.mu.Unlock()

	if len(batch) == 0 {
		return
	}

	bp.logger.WithField("batch_size", len(batch)).Debug("Processing start batch")

	bp.processInParallel(len(batch), func(index int) {
		bp.orchestrator.Start(batch[index])
	})
}

func (bp *BatchProcessor) flushCancelBatch() {
	bp.mu.Lock()
	batch := bp.cancelBatch
	bp.cancelBatch = nil
	bp.mu.Unlock()

	if len(batch) == 0 {
		return
	}

	bp.logger.WithField("batch_size", len(batch)).Debug("Processing cancel batch")

	bp.processInParallel(len(batch), func(index int) {
		req := batch[index]
		bp.orchestrator.Cancel(req.orderID, req.reason)
	})
}

func (bp *BatchProcessor) flushRefundBatch() {
	bp.mu.Lock()
	batch := bp.refundBatch
	bp.refundBatch = nil
	bp.mu.Unlock()

	if len(batch) == 0 {
		return
	}

	bp.logger.WithField("batch_size", len(batch)).Debug("Processing refund batch")

	bp.processInParallel(len(batch), func(index int) {
		req := batch[index]
		bp.orchestrator.Refund(req.orderID, req.amountMinor, req.reason)
	})
}

func (bp *BatchProcessor) processInParallel(size int, processFn func(index int)) {
	if size == 0 {
		return
	}

	limit := bp.maxParallelOps
	if limit <= 0 {
		limit = 1
	}
	if limit > size {
		limit = size
	}

	semaphore := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for idx := 0; idx < size; idx++ {
		wg.Add(1)
		semaphore <- struct{}{}
		go func(index int) {
			defer wg.Done()
			defer func() { <-semaphore }()
			processFn(index)
		}(idx)
	}

	wg.Wait()
}
