package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	omsv1 "github.com/vladislavdragonenkov/oms/proto/oms/v1"
)

const (
	idempotencyHeader = "idempotency-key"
	defaultAmount     = int64(1000)
	defaultQty        = int32(1)
)

type loadMode string

const (
	modeCreate          loadMode = "create"
	modeCreatePay       loadMode = "create-pay"
	modeCreatePayCancel loadMode = "create-pay-cancel"
)

type config struct {
	addr        string
	total       int
	totalSet    bool
	duration    time.Duration
	concurrency int
	connections int
	timeout     time.Duration
	mode        loadMode
	cancelRate  int
	currency    string
	sku         string
	amountMinor int64
	customerTag string
	outputPath  string
}

type latencySummary struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
	Avg float64 `json:"avg"`
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
}

type methodReport struct {
	Calls     int64            `json:"calls"`
	Success   int64            `json:"success"`
	Failed    int64            `json:"failed"`
	ErrorRate float64          `json:"error_rate"`
	Codes     map[string]int64 `json:"codes"`
	LatencyMs latencySummary   `json:"latency_ms"`
}

type report struct {
	StartedAt         time.Time               `json:"started_at"`
	DurationSeconds   float64                 `json:"duration_seconds"`
	TotalScenarios    int64                   `json:"total_scenarios"`
	SuccessScenarios  int64                   `json:"success_scenarios"`
	FailedScenarios   int64                   `json:"failed_scenarios"`
	ErrorRate         float64                 `json:"error_rate"`
	RPS               float64                 `json:"rps"`
	ScenarioLatencyMs latencySummary          `json:"scenario_latency_ms"`
	Methods           map[string]methodReport `json:"methods"`
}

type methodStats struct {
	calls     int64
	success   int64
	failed    int64
	codes     map[string]int64
	latencies []float64
}

type collector struct {
	mu      sync.Mutex
	methods map[string]*methodStats
}

func newCollector() *collector {
	return &collector{
		methods: make(map[string]*methodStats),
	}
}

func (c *collector) record(method string, latency time.Duration, code codes.Code) {
	c.mu.Lock()
	defer c.mu.Unlock()

	stats, ok := c.methods[method]
	if !ok {
		stats = &methodStats{
			codes: make(map[string]int64),
		}
		c.methods[method] = stats
	}

	stats.calls++
	if code == codes.OK {
		stats.success++
	} else {
		stats.failed++
	}
	stats.codes[code.String()]++
	stats.latencies = append(stats.latencies, float64(latency.Microseconds())/1000.0)
}

func (c *collector) snapshot(name string) (methodReport, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	stats, ok := c.methods[name]
	if !ok {
		return methodReport{}, false
	}

	codesCopy := make(map[string]int64, len(stats.codes))
	for code, count := range stats.codes {
		codesCopy[code] = count
	}

	return methodReport{
		Calls:     stats.calls,
		Success:   stats.success,
		Failed:    stats.failed,
		ErrorRate: ratio(stats.failed, stats.calls),
		Codes:     codesCopy,
		LatencyMs: buildLatencySummary(stats.latencies),
	}, true
}

func (c *collector) buildReport(startedAt time.Time, duration time.Duration) report {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := report{
		StartedAt:       startedAt.UTC(),
		DurationSeconds: duration.Seconds(),
		Methods:         make(map[string]methodReport, len(c.methods)),
	}

	scenarioStats := c.methods["scenario"]
	if scenarioStats != nil {
		result.TotalScenarios = scenarioStats.calls
		result.SuccessScenarios = scenarioStats.success
		result.FailedScenarios = scenarioStats.failed
		result.ErrorRate = ratio(scenarioStats.failed, scenarioStats.calls)
		result.ScenarioLatencyMs = buildLatencySummary(scenarioStats.latencies)
	}
	if duration > 0 {
		result.RPS = float64(result.TotalScenarios) / duration.Seconds()
	}

	for name, stats := range c.methods {
		codesCopy := make(map[string]int64, len(stats.codes))
		for code, count := range stats.codes {
			codesCopy[code] = count
		}
		result.Methods[name] = methodReport{
			Calls:     stats.calls,
			Success:   stats.success,
			Failed:    stats.failed,
			ErrorRate: ratio(stats.failed, stats.calls),
			Codes:     codesCopy,
			LatencyMs: buildLatencySummary(stats.latencies),
		}
	}

	return result
}

func parseConfig() (config, error) {
	var cfg config
	var modeValue string
	var timeoutValue string
	var durationValue string

	flag.StringVar(&cfg.addr, "addr", "localhost:50051", "gRPC target address")
	flag.IntVar(&cfg.total, "total", 400, "total scenarios to execute in count mode; in duration mode only used when explicitly set")
	flag.StringVar(&durationValue, "duration", "0s", "optional time-based run duration (e.g. 10m, 15m)")
	flag.IntVar(&cfg.concurrency, "concurrency", 40, "number of concurrent workers")
	flag.IntVar(&cfg.connections, "connections", 20, "number of gRPC client connections")
	flag.StringVar(&timeoutValue, "timeout", "5s", "per-RPC timeout")
	flag.StringVar(&modeValue, "mode", string(modeCreate), "load mode: create | create-pay | create-pay-cancel")
	flag.IntVar(&cfg.cancelRate, "cancel-rate", 0, "cancel probability in percent for create-pay mode (0..100)")
	flag.StringVar(&cfg.currency, "currency", "USD", "order currency")
	flag.StringVar(&cfg.sku, "sku", "SKU-LOAD", "order item SKU")
	flag.Int64Var(&cfg.amountMinor, "amount-minor", defaultAmount, "order item amount in minor units")
	flag.StringVar(&cfg.customerTag, "customer-tag", "load", "customer id prefix")
	flag.StringVar(&cfg.outputPath, "output", "", "optional JSON report output file path")
	flag.Parse()

	timeout, err := time.ParseDuration(strings.TrimSpace(timeoutValue))
	if err != nil {
		return cfg, fmt.Errorf("parse timeout: %w", err)
	}
	cfg.timeout = timeout

	duration, err := time.ParseDuration(strings.TrimSpace(durationValue))
	if err != nil {
		return cfg, fmt.Errorf("parse duration: %w", err)
	}
	cfg.duration = duration

	flag.CommandLine.Visit(func(f *flag.Flag) {
		if f.Name == "total" {
			cfg.totalSet = true
		}
	})

	mode, err := parseMode(modeValue)
	if err != nil {
		return cfg, err
	}
	cfg.mode = mode

	if cfg.duration < 0 {
		return cfg, errors.New("duration must be >= 0")
	}
	if cfg.duration == 0 && cfg.total <= 0 {
		return cfg, errors.New("total must be > 0 when duration is not set")
	}
	if cfg.duration > 0 && cfg.totalSet && cfg.total <= 0 {
		return cfg, errors.New("total must be > 0 when explicitly set with duration")
	}
	if cfg.concurrency <= 0 {
		return cfg, errors.New("concurrency must be > 0")
	}
	if cfg.connections <= 0 {
		return cfg, errors.New("connections must be > 0")
	}
	if cfg.timeout <= 0 {
		return cfg, errors.New("timeout must be > 0")
	}
	if cfg.amountMinor <= 0 {
		return cfg, errors.New("amount-minor must be > 0")
	}
	if cfg.cancelRate < 0 || cfg.cancelRate > 100 {
		return cfg, errors.New("cancel-rate must be between 0 and 100")
	}
	if strings.TrimSpace(cfg.currency) == "" {
		return cfg, errors.New("currency is required")
	}
	if strings.TrimSpace(cfg.sku) == "" {
		return cfg, errors.New("sku is required")
	}
	if strings.TrimSpace(cfg.customerTag) == "" {
		return cfg, errors.New("customer-tag is required")
	}

	return cfg, nil
}

func parseMode(value string) (loadMode, error) {
	switch loadMode(strings.TrimSpace(value)) {
	case modeCreate:
		return modeCreate, nil
	case modeCreatePay:
		return modeCreatePay, nil
	case modeCreatePayCancel:
		return modeCreatePayCancel, nil
	default:
		return "", fmt.Errorf("unsupported mode: %s", value)
	}
}

func main() {
	cfg, err := parseConfig()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "invalid config: %v\n", err)
		os.Exit(1)
	}

	conns := make([]*grpc.ClientConn, 0, cfg.connections)
	clients := make([]omsv1.OrderServiceClient, 0, cfg.connections)
	for i := 0; i < cfg.connections; i++ {
		conn, dialErr := grpc.NewClient(cfg.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if dialErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to create grpc client connection: %v\n", dialErr)
			os.Exit(1)
		}
		conns = append(conns, conn)
		clients = append(clients, omsv1.NewOrderServiceClient(conn))
	}
	defer func() {
		for _, conn := range conns {
			_ = conn.Close()
		}
	}()

	startedAt := time.Now()
	runID := fmt.Sprintf("%d-%d", startedAt.UnixNano(), os.Getpid())
	col := newCollector()

	jobs := make(chan int, cfg.concurrency*2)
	var failures int64
	var wg sync.WaitGroup

	for workerID := 0; workerID < cfg.concurrency; workerID++ {
		wg.Add(1)
		client := clients[workerID%len(clients)]
		go func(cli omsv1.OrderServiceClient) {
			defer wg.Done()
			for id := range jobs {
				if runErr := runScenario(cli, cfg, id, runID, col); runErr != nil {
					atomic.AddInt64(&failures, 1)
				}
			}
		}(client)
	}

	dispatchJobs(jobs, cfg)
	wg.Wait()

	duration := time.Since(startedAt)
	result := col.buildReport(startedAt, duration)
	if result.FailedScenarios == 0 && failures > 0 {
		result.FailedScenarios = failures
		result.ErrorRate = ratio(result.FailedScenarios, result.TotalScenarios)
	}

	printReport(result, cfg)
	if cfg.outputPath != "" {
		if err := writeJSONReport(cfg.outputPath, result); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to write report: %v\n", err)
			os.Exit(1)
		}
	}

	if result.FailedScenarios > 0 {
		os.Exit(1)
	}
}

func dispatchJobs(jobs chan<- int, cfg config) {
	defer close(jobs)

	if cfg.duration <= 0 {
		for i := 0; i < cfg.total; i++ {
			jobs <- i
		}
		return
	}

	timer := time.NewTimer(cfg.duration)
	defer timer.Stop()

	for i := 0; ; i++ {
		if cfg.totalSet && i >= cfg.total {
			return
		}

		select {
		case <-timer.C:
			return
		case jobs <- i:
		}
	}
}

func runScenario(
	client omsv1.OrderServiceClient,
	cfg config,
	index int,
	runID string,
	col *collector,
) error {
	scenarioStart := time.Now()
	scenarioCode := codes.OK
	defer func() {
		col.record("scenario", time.Since(scenarioStart), scenarioCode)
	}()

	createReq := &omsv1.CreateOrderRequest{
		CustomerId: fmt.Sprintf("%s-%s-%d", cfg.customerTag, runID, index),
		Currency:   cfg.currency,
		Items: []*omsv1.OrderItem{
			{
				Sku: cfg.sku,
				Qty: defaultQty,
				Price: &omsv1.Money{
					Currency:    cfg.currency,
					AmountMinor: cfg.amountMinor,
				},
			},
		},
	}

	createKey := fmt.Sprintf("lt-create-%s-%d", runID, index)
	orderResp, err := callCreateOrder(client, cfg.timeout, createReq, createKey, col)
	if err != nil {
		scenarioCode = grpcCode(err)
		return err
	}
	orderID := orderResp.GetOrder().GetId()
	if orderID == "" {
		scenarioCode = codes.Internal
		return errors.New("create response returned empty order id")
	}

	if cfg.mode == modeCreate {
		return nil
	}

	payKey := fmt.Sprintf("lt-pay-%s-%d", runID, index)
	if err := callPayOrder(client, cfg.timeout, orderID, payKey, col); err != nil {
		scenarioCode = grpcCode(err)
		return err
	}

	if cfg.mode == modeCreatePayCancel || (cfg.mode == modeCreatePay && shouldCancelScenario(index, cfg.cancelRate)) {
		cancelKey := fmt.Sprintf("lt-cancel-%s-%d", runID, index)
		if err := callCancelOrder(client, cfg.timeout, orderID, cancelKey, col); err != nil {
			scenarioCode = grpcCode(err)
			return err
		}
	}

	return nil
}

func callCreateOrder(
	client omsv1.OrderServiceClient,
	timeout time.Duration,
	req *omsv1.CreateOrderRequest,
	key string,
	col *collector,
) (*omsv1.CreateOrderResponse, error) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, idempotencyHeader, key)

	resp, err := client.CreateOrder(ctx, req)
	col.record("CreateOrder", time.Since(start), grpcCode(err))
	return resp, err
}

func callPayOrder(
	client omsv1.OrderServiceClient,
	timeout time.Duration,
	orderID, key string,
	col *collector,
) error {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, idempotencyHeader, key)

	_, err := client.PayOrder(ctx, &omsv1.PayOrderRequest{OrderId: orderID})
	col.record("PayOrder", time.Since(start), grpcCode(err))
	return err
}

func callCancelOrder(
	client omsv1.OrderServiceClient,
	timeout time.Duration,
	orderID, key string,
	col *collector,
) error {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, idempotencyHeader, key)

	_, err := client.CancelOrder(ctx, &omsv1.CancelOrderRequest{
		OrderId: orderID,
		Reason:  "load-cancel",
	})
	col.record("CancelOrder", time.Since(start), grpcCode(err))
	return err
}

func grpcCode(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	return status.Code(err)
}

func shouldCancelScenario(index, cancelRate int) bool {
	if cancelRate <= 0 {
		return false
	}
	if cancelRate >= 100 {
		return true
	}
	return index%100 < cancelRate
}

func writeJSONReport(path string, result report) error {
	cleanPath := filepath.Clean(path)
	if cleanPath == "." || cleanPath == string(filepath.Separator) {
		return errors.New("output path must point to a file")
	}
	if cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return fmt.Errorf("output path must be inside current directory: %s", path)
	}

	// #nosec G304 -- path is an explicit CLI output parameter for local load-test reports.
	file, err := os.Create(cleanPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func printReport(result report, cfg config) {
	fmt.Println("Load test summary")
	fmt.Printf("mode=%s run=%s total=%d success=%d failed=%d error_rate=%.4f\n",
		cfg.mode,
		runTarget(cfg),
		result.TotalScenarios,
		result.SuccessScenarios,
		result.FailedScenarios,
		result.ErrorRate,
	)
	fmt.Printf("duration=%.2fs rps=%.2f\n", result.DurationSeconds, result.RPS)
	fmt.Printf("scenario latency ms: min=%.2f avg=%.2f p50=%.2f p95=%.2f p99=%.2f max=%.2f\n",
		result.ScenarioLatencyMs.Min,
		result.ScenarioLatencyMs.Avg,
		result.ScenarioLatencyMs.P50,
		result.ScenarioLatencyMs.P95,
		result.ScenarioLatencyMs.P99,
		result.ScenarioLatencyMs.Max,
	)

	methodNames := make([]string, 0, len(result.Methods))
	for name := range result.Methods {
		if name == "scenario" {
			continue
		}
		methodNames = append(methodNames, name)
	}
	sort.Strings(methodNames)
	for _, name := range methodNames {
		stats := result.Methods[name]
		fmt.Printf(
			"%s: calls=%d success=%d failed=%d error_rate=%.4f p95=%.2fms\n",
			name,
			stats.Calls,
			stats.Success,
			stats.Failed,
			stats.ErrorRate,
			stats.LatencyMs.P95,
		)
	}
}

func runTarget(cfg config) string {
	if cfg.duration <= 0 {
		return fmt.Sprintf("count:%d", cfg.total)
	}
	if cfg.totalSet {
		return fmt.Sprintf("duration:%s,max-total:%d", cfg.duration, cfg.total)
	}
	return fmt.Sprintf("duration:%s", cfg.duration)
}

func buildLatencySummary(values []float64) latencySummary {
	if len(values) == 0 {
		return latencySummary{}
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	var sum float64
	for _, value := range sorted {
		sum += value
	}

	return latencySummary{
		Min: sorted[0],
		Max: sorted[len(sorted)-1],
		Avg: sum / float64(len(sorted)),
		P50: percentile(sorted, 50),
		P95: percentile(sorted, 95),
		P99: percentile(sorted, 99),
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}

	rank := (p / 100.0) * float64(len(sorted)-1)
	lower := int(math.Floor(rank))
	upper := int(math.Ceil(rank))
	if lower == upper {
		return sorted[lower]
	}

	weight := rank - float64(lower)
	return sorted[lower] + (sorted[upper]-sorted[lower])*weight
}

func ratio(failed, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(failed) / float64(total)
}
