package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	omsv1 "github.com/vladislavdragonenkov/oms/proto/oms/v1"
)

type fakeOrderServiceClient struct {
	createFn func(context.Context, *omsv1.CreateOrderRequest, ...grpc.CallOption) (*omsv1.CreateOrderResponse, error)
	payFn    func(context.Context, *omsv1.PayOrderRequest, ...grpc.CallOption) (*omsv1.PayOrderResponse, error)
	cancelFn func(context.Context, *omsv1.CancelOrderRequest, ...grpc.CallOption) (*omsv1.CancelOrderResponse, error)
	getFn    func(context.Context, *omsv1.GetOrderRequest, ...grpc.CallOption) (*omsv1.GetOrderResponse, error)
	listFn   func(context.Context, *omsv1.ListOrdersRequest, ...grpc.CallOption) (*omsv1.ListOrdersResponse, error)
	refundFn func(context.Context, *omsv1.RefundOrderRequest, ...grpc.CallOption) (*omsv1.RefundOrderResponse, error)
}

func (f *fakeOrderServiceClient) CreateOrder(ctx context.Context, req *omsv1.CreateOrderRequest, opts ...grpc.CallOption) (*omsv1.CreateOrderResponse, error) {
	if f.createFn == nil {
		return nil, errors.New("unexpected CreateOrder call")
	}
	return f.createFn(ctx, req, opts...)
}

func (f *fakeOrderServiceClient) PayOrder(ctx context.Context, req *omsv1.PayOrderRequest, opts ...grpc.CallOption) (*omsv1.PayOrderResponse, error) {
	if f.payFn == nil {
		return nil, errors.New("unexpected PayOrder call")
	}
	return f.payFn(ctx, req, opts...)
}

func (f *fakeOrderServiceClient) CancelOrder(ctx context.Context, req *omsv1.CancelOrderRequest, opts ...grpc.CallOption) (*omsv1.CancelOrderResponse, error) {
	if f.cancelFn == nil {
		return nil, errors.New("unexpected CancelOrder call")
	}
	return f.cancelFn(ctx, req, opts...)
}

func (f *fakeOrderServiceClient) GetOrder(ctx context.Context, req *omsv1.GetOrderRequest, opts ...grpc.CallOption) (*omsv1.GetOrderResponse, error) {
	if f.getFn == nil {
		return nil, errors.New("unexpected GetOrder call")
	}
	return f.getFn(ctx, req, opts...)
}

func (f *fakeOrderServiceClient) ListOrders(ctx context.Context, req *omsv1.ListOrdersRequest, opts ...grpc.CallOption) (*omsv1.ListOrdersResponse, error) {
	if f.listFn == nil {
		return nil, errors.New("unexpected ListOrders call")
	}
	return f.listFn(ctx, req, opts...)
}

func (f *fakeOrderServiceClient) RefundOrder(ctx context.Context, req *omsv1.RefundOrderRequest, opts ...grpc.CallOption) (*omsv1.RefundOrderResponse, error) {
	if f.refundFn == nil {
		return nil, errors.New("unexpected RefundOrder call")
	}
	return f.refundFn(ctx, req, opts...)
}

func withCLIArgs(t *testing.T, args []string, fn func()) {
	t.Helper()

	oldArgs := os.Args
	oldCommandLine := flag.CommandLine

	os.Args = append([]string{"loadtest"}, args...)
	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flag.CommandLine = fs

	defer func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	}()

	fn()
}

func TestParseMode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    loadMode
		wantErr string
	}{
		{name: "create", input: "create", want: modeCreate},
		{name: "create-pay", input: "create-pay", want: modeCreatePay},
		{name: "create-pay-cancel", input: "create-pay-cancel", want: modeCreatePayCancel},
		{name: "unsupported", input: "bad", wantErr: "unsupported mode"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseMode(tc.input)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("unexpected mode: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestParseConfig(t *testing.T) {
	t.Run("count mode", func(t *testing.T) {
		withCLIArgs(t, []string{
			"-addr=127.0.0.1:50051",
			"-mode=create-pay",
			"-total=12",
			"-concurrency=3",
			"-connections=2",
			"-timeout=2s",
			"-cancel-rate=10",
			"-currency=EUR",
			"-sku=SKU-X",
			"-amount-minor=99",
			"-customer-tag=stage",
			"-output=/tmp/out.json",
		}, func() {
			cfg, err := parseConfig()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !cfg.totalSet {
				t.Fatalf("expected totalSet=true")
			}
			if cfg.duration != 0 {
				t.Fatalf("expected zero duration, got %s", cfg.duration)
			}
			if cfg.mode != modeCreatePay {
				t.Fatalf("unexpected mode: %s", cfg.mode)
			}
			if cfg.total != 12 || cfg.concurrency != 3 || cfg.connections != 2 {
				t.Fatalf("unexpected numeric config: %+v", cfg)
			}
			if cfg.timeout != 2*time.Second {
				t.Fatalf("unexpected timeout: %s", cfg.timeout)
			}
		})
	})

	t.Run("duration mode", func(t *testing.T) {
		withCLIArgs(t, []string{
			"-duration=3s",
			"-concurrency=2",
			"-connections=1",
		}, func() {
			cfg, err := parseConfig()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.duration != 3*time.Second {
				t.Fatalf("unexpected duration: %s", cfg.duration)
			}
			if cfg.totalSet {
				t.Fatalf("expected totalSet=false when -total was not provided")
			}
		})
	})

	t.Run("validation errors", func(t *testing.T) {
		tests := []struct {
			name    string
			args    []string
			wantErr string
		}{
			{name: "invalid duration", args: []string{"-duration=bad"}, wantErr: "parse duration"},
			{name: "negative duration", args: []string{"-duration=-1s"}, wantErr: "duration must be >= 0"},
			{name: "invalid cancel rate", args: []string{"-cancel-rate=101"}, wantErr: "cancel-rate must be between 0 and 100"},
			{name: "empty total", args: []string{"-duration=0s", "-total=0"}, wantErr: "total must be > 0"},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				withCLIArgs(t, tc.args, func() {
					_, err := parseConfig()
					if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
						t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
					}
				})
			})
		}
	})
}

func TestDispatchJobs(t *testing.T) {
	t.Run("count mode", func(t *testing.T) {
		jobs := make(chan int, 16)
		dispatchJobs(jobs, config{total: 5})

		var got []int
		for v := range jobs {
			got = append(got, v)
		}
		if !slices.Equal(got, []int{0, 1, 2, 3, 4}) {
			t.Fatalf("unexpected jobs sequence: %v", got)
		}
	})

	t.Run("duration mode", func(t *testing.T) {
		jobs := make(chan int, 32)
		done := make(chan struct{})
		go func() {
			dispatchJobs(jobs, config{duration: 20 * time.Millisecond})
			close(done)
		}()

		count := 0
		for range jobs {
			count++
		}
		<-done
		if count == 0 {
			t.Fatalf("expected non-zero jobs for duration mode")
		}
	})

	t.Run("duration with explicit max total", func(t *testing.T) {
		jobs := make(chan int, 16)
		dispatchJobs(jobs, config{duration: time.Second, total: 3, totalSet: true})
		count := 0
		for range jobs {
			count++
		}
		if count != 3 {
			t.Fatalf("expected 3 jobs, got %d", count)
		}
	})
}

func TestCollectorAndReport(t *testing.T) {
	c := newCollector()
	c.record("scenario", 10*time.Millisecond, codes.OK)
	c.record("scenario", 20*time.Millisecond, codes.Internal)
	c.record("CreateOrder", 15*time.Millisecond, codes.OK)

	snap, ok := c.snapshot("scenario")
	if !ok {
		t.Fatalf("scenario snapshot missing")
	}
	if snap.Calls != 2 || snap.Success != 1 || snap.Failed != 1 {
		t.Fatalf("unexpected scenario snapshot: %+v", snap)
	}
	if snap.Codes[codes.OK.String()] != 1 || snap.Codes[codes.Internal.String()] != 1 {
		t.Fatalf("unexpected codes: %+v", snap.Codes)
	}

	r := c.buildReport(time.Now(), 2*time.Second)
	if r.TotalScenarios != 2 || r.FailedScenarios != 1 {
		t.Fatalf("unexpected report totals: %+v", r)
	}
	if r.RPS <= 0 {
		t.Fatalf("expected positive rps, got %f", r.RPS)
	}
	if _, ok := r.Methods["CreateOrder"]; !ok {
		t.Fatalf("expected CreateOrder stats in report")
	}
}

func TestUtilityFunctions(t *testing.T) {
	if got := grpcCode(nil); got != codes.OK {
		t.Fatalf("grpcCode(nil) = %s, want OK", got)
	}
	if got := grpcCode(status.Error(codes.Unavailable, "down")); got != codes.Unavailable {
		t.Fatalf("unexpected grpc code: %s", got)
	}

	if got := ratio(1, 4); got != 0.25 {
		t.Fatalf("ratio mismatch: %f", got)
	}
	if got := ratio(1, 0); got != 0 {
		t.Fatalf("ratio with zero total must be 0, got %f", got)
	}

	values := []float64{10, 20, 30, 40}
	summary := buildLatencySummary(values)
	if summary.P50 <= 0 || summary.P95 <= 0 || summary.Max != 40 {
		t.Fatalf("unexpected latency summary: %+v", summary)
	}
	if p := percentile(values, 95); p <= 0 {
		t.Fatalf("unexpected percentile: %f", p)
	}

	if got := runTarget(config{total: 50}); got != "count:50" {
		t.Fatalf("unexpected run target: %s", got)
	}
	if got := runTarget(config{duration: 2 * time.Second}); got != "duration:2s" {
		t.Fatalf("unexpected duration run target: %s", got)
	}
	if got := runTarget(config{duration: 2 * time.Second, total: 10, totalSet: true}); got != "duration:2s,max-total:10" {
		t.Fatalf("unexpected capped duration run target: %s", got)
	}
}

func TestWriteJSONReport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")

	sample := report{TotalScenarios: 2, SuccessScenarios: 2}
	if err := writeJSONReport(path, sample); err != nil {
		t.Fatalf("writeJSONReport error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	var decoded report
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if decoded.TotalScenarios != 2 || decoded.SuccessScenarios != 2 {
		t.Fatalf("unexpected decoded report: %+v", decoded)
	}
}

func TestRPCHelpersAndRunScenario(t *testing.T) {
	c := newCollector()

	client := &fakeOrderServiceClient{
		createFn: func(ctx context.Context, req *omsv1.CreateOrderRequest, _ ...grpc.CallOption) (*omsv1.CreateOrderResponse, error) {
			mustHaveIdempotencyKey(t, ctx, "create-key")
			if req.GetCustomerId() == "" {
				t.Fatalf("customer id is required")
			}
			return &omsv1.CreateOrderResponse{Order: &omsv1.Order{Id: "order-1"}}, nil
		},
		payFn: func(ctx context.Context, req *omsv1.PayOrderRequest, _ ...grpc.CallOption) (*omsv1.PayOrderResponse, error) {
			mustHaveIdempotencyKey(t, ctx, "pay-key")
			if req.GetOrderId() == "" {
				t.Fatalf("order id is required")
			}
			return &omsv1.PayOrderResponse{OrderId: req.GetOrderId(), Status: omsv1.OrderStatus_ORDER_STATUS_PAID}, nil
		},
		cancelFn: func(ctx context.Context, req *omsv1.CancelOrderRequest, _ ...grpc.CallOption) (*omsv1.CancelOrderResponse, error) {
			mustHaveIdempotencyKey(t, ctx, "cancel-key")
			if req.GetOrderId() == "" {
				t.Fatalf("order id is required")
			}
			return &omsv1.CancelOrderResponse{OrderId: req.GetOrderId(), Status: omsv1.OrderStatus_ORDER_STATUS_CANCELED}, nil
		},
	}

	if _, err := callCreateOrder(client, time.Second, &omsv1.CreateOrderRequest{CustomerId: "c-1", Currency: "USD"}, "create-key", c); err != nil {
		t.Fatalf("callCreateOrder failed: %v", err)
	}
	if err := callPayOrder(client, time.Second, "order-1", "pay-key", c); err != nil {
		t.Fatalf("callPayOrder failed: %v", err)
	}
	if err := callCancelOrder(client, time.Second, "order-1", "cancel-key", c); err != nil {
		t.Fatalf("callCancelOrder failed: %v", err)
	}

	snap, ok := c.snapshot("CreateOrder")
	if !ok || snap.Calls == 0 {
		t.Fatalf("CreateOrder metric missing")
	}

	runCfg := config{
		mode:        modeCreatePayCancel,
		timeout:     time.Second,
		currency:    "USD",
		sku:         "SKU-1",
		amountMinor: 100,
		customerTag: "load",
	}
	scenarioClient := &fakeOrderServiceClient{
		createFn: func(ctx context.Context, req *omsv1.CreateOrderRequest, _ ...grpc.CallOption) (*omsv1.CreateOrderResponse, error) {
			mustHaveIdempotencyKeyPrefix(t, ctx, "lt-create-run-1-1")
			if req.GetCustomerId() == "" {
				t.Fatalf("customer id is required")
			}
			return &omsv1.CreateOrderResponse{Order: &omsv1.Order{Id: "order-1"}}, nil
		},
		payFn: func(ctx context.Context, req *omsv1.PayOrderRequest, _ ...grpc.CallOption) (*omsv1.PayOrderResponse, error) {
			mustHaveIdempotencyKeyPrefix(t, ctx, "lt-pay-run-1-1")
			return &omsv1.PayOrderResponse{OrderId: req.GetOrderId(), Status: omsv1.OrderStatus_ORDER_STATUS_PAID}, nil
		},
		cancelFn: func(ctx context.Context, req *omsv1.CancelOrderRequest, _ ...grpc.CallOption) (*omsv1.CancelOrderResponse, error) {
			mustHaveIdempotencyKeyPrefix(t, ctx, "lt-cancel-run-1-1")
			return &omsv1.CancelOrderResponse{OrderId: req.GetOrderId(), Status: omsv1.OrderStatus_ORDER_STATUS_CANCELED}, nil
		},
	}
	if err := runScenario(scenarioClient, runCfg, 1, "run-1", c); err != nil {
		t.Fatalf("runScenario failed: %v", err)
	}

	failingClient := &fakeOrderServiceClient{
		createFn: func(context.Context, *omsv1.CreateOrderRequest, ...grpc.CallOption) (*omsv1.CreateOrderResponse, error) {
			return nil, status.Error(codes.Unavailable, "create unavailable")
		},
	}
	if err := runScenario(failingClient, runCfg, 2, "run-2", c); status.Code(err) != codes.Unavailable {
		t.Fatalf("expected Unavailable error, got %v", err)
	}

	emptyIDClient := &fakeOrderServiceClient{
		createFn: func(context.Context, *omsv1.CreateOrderRequest, ...grpc.CallOption) (*omsv1.CreateOrderResponse, error) {
			return &omsv1.CreateOrderResponse{Order: &omsv1.Order{}}, nil
		},
	}
	if err := runScenario(emptyIDClient, runCfg, 3, "run-3", c); err == nil || !strings.Contains(err.Error(), "empty order id") {
		t.Fatalf("expected empty id error, got %v", err)
	}
}

func TestPrintReport(t *testing.T) {
	r := report{
		TotalScenarios:   2,
		SuccessScenarios: 2,
		Methods: map[string]methodReport{
			"scenario":    {Calls: 2, Success: 2},
			"CreateOrder": {Calls: 2, Success: 2},
		},
	}

	out := captureStdout(t, func() {
		printReport(r, config{mode: modeCreate, total: 2})
	})

	if !strings.Contains(out, "Load test summary") {
		t.Fatalf("expected summary header, got: %s", out)
	}
	if !strings.Contains(out, "CreateOrder") {
		t.Fatalf("expected method section, got: %s", out)
	}
}

type loadtestMainServer struct {
	omsv1.UnimplementedOrderServiceServer
}

func (s *loadtestMainServer) CreateOrder(_ context.Context, req *omsv1.CreateOrderRequest) (*omsv1.CreateOrderResponse, error) {
	return &omsv1.CreateOrderResponse{Order: &omsv1.Order{Id: "order-" + req.GetCustomerId()}}, nil
}

func (s *loadtestMainServer) GetOrder(context.Context, *omsv1.GetOrderRequest) (*omsv1.GetOrderResponse, error) {
	return &omsv1.GetOrderResponse{}, nil
}

func (s *loadtestMainServer) ListOrders(context.Context, *omsv1.ListOrdersRequest) (*omsv1.ListOrdersResponse, error) {
	return &omsv1.ListOrdersResponse{}, nil
}

func (s *loadtestMainServer) PayOrder(context.Context, *omsv1.PayOrderRequest) (*omsv1.PayOrderResponse, error) {
	return &omsv1.PayOrderResponse{Status: omsv1.OrderStatus_ORDER_STATUS_PAID}, nil
}

func (s *loadtestMainServer) CancelOrder(context.Context, *omsv1.CancelOrderRequest) (*omsv1.CancelOrderResponse, error) {
	return &omsv1.CancelOrderResponse{Status: omsv1.OrderStatus_ORDER_STATUS_CANCELED}, nil
}

func (s *loadtestMainServer) RefundOrder(context.Context, *omsv1.RefundOrderRequest) (*omsv1.RefundOrderResponse, error) {
	return &omsv1.RefundOrderResponse{Status: omsv1.OrderStatus_ORDER_STATUS_REFUNDED}, nil
}

func TestMainSmoke(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func(lis net.Listener) {
		if err := lis.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Fatalf("close listener: %v", err)
		}
	}(lis)

	srv := grpc.NewServer()
	omsv1.RegisterOrderServiceServer(srv, &loadtestMainServer{})
	go func() {
		_ = srv.Serve(lis)
	}()
	defer srv.Stop()

	dir := t.TempDir()
	outPath := filepath.Join(dir, "main-report.json")

	withCLIArgs(t, []string{
		"-addr=" + lis.Addr().String(),
		"-mode=create",
		"-total=5",
		"-concurrency=2",
		"-connections=1",
		"-timeout=2s",
		"-output=" + outPath,
	}, func() {
		main()
	})

	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected report file from main: %v", err)
	}
}

func mustHaveIdempotencyKey(t *testing.T, ctx context.Context, want string) {
	t.Helper()

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatalf("missing outgoing metadata")
	}
	values := md.Get(idempotencyHeader)
	if len(values) != 1 || values[0] != want {
		t.Fatalf("unexpected idempotency key: got=%v want=%q", values, want)
	}
}

func mustHaveIdempotencyKeyPrefix(t *testing.T, ctx context.Context, wantPrefix string) {
	t.Helper()

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatalf("missing outgoing metadata")
	}
	values := md.Get(idempotencyHeader)
	if len(values) != 1 || !strings.HasPrefix(values[0], wantPrefix) {
		t.Fatalf("unexpected idempotency key: got=%v want prefix %q", values, wantPrefix)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = oldStdout

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read captured output: %v", err)
	}
	_ = r.Close()

	return string(data)
}

func TestFakeClientImplementsInterface(t *testing.T) {
	var _ omsv1.OrderServiceClient = (*fakeOrderServiceClient)(nil)
	if reflect.TypeOf((*fakeOrderServiceClient)(nil)) == nil {
		t.Fatalf("type check failed")
	}
}
