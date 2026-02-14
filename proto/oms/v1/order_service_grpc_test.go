package omsv1

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeClientConn struct {
	invoke func(context.Context, string, any, any, ...grpc.CallOption) error
}

func (f *fakeClientConn) Invoke(ctx context.Context, method string, args any, reply any, opts ...grpc.CallOption) error {
	if f.invoke == nil {
		return errors.New("unexpected Invoke call")
	}
	return f.invoke(ctx, method, args, reply, opts...)
}

func (f *fakeClientConn) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, errors.New("not implemented")
}

type grpcTestOrderService struct {
	UnimplementedOrderServiceServer
}

func (s *grpcTestOrderService) CreateOrder(_ context.Context, req *CreateOrderRequest) (*CreateOrderResponse, error) {
	return &CreateOrderResponse{Order: &Order{Id: "order-" + req.GetCustomerId()}}, nil
}

func (s *grpcTestOrderService) GetOrder(_ context.Context, req *GetOrderRequest) (*GetOrderResponse, error) {
	return &GetOrderResponse{Order: &Order{Id: req.GetOrderId()}}, nil
}

func (s *grpcTestOrderService) ListOrders(context.Context, *ListOrdersRequest) (*ListOrdersResponse, error) {
	return &ListOrdersResponse{Orders: []*Order{{Id: "order-1"}}}, nil
}

func (s *grpcTestOrderService) PayOrder(_ context.Context, req *PayOrderRequest) (*PayOrderResponse, error) {
	return &PayOrderResponse{OrderId: req.GetOrderId(), Status: OrderStatus_ORDER_STATUS_PAID}, nil
}

func (s *grpcTestOrderService) CancelOrder(_ context.Context, req *CancelOrderRequest) (*CancelOrderResponse, error) {
	return &CancelOrderResponse{OrderId: req.GetOrderId(), Status: OrderStatus_ORDER_STATUS_CANCELED}, nil
}

func (s *grpcTestOrderService) RefundOrder(_ context.Context, req *RefundOrderRequest) (*RefundOrderResponse, error) {
	return &RefundOrderResponse{OrderId: req.GetOrderId(), Status: OrderStatus_ORDER_STATUS_REFUNDED}, nil
}

func TestOrderServiceClientMethods(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		methods := map[string]int{}
		conn := &fakeClientConn{
			invoke: func(_ context.Context, method string, _ any, reply any, _ ...grpc.CallOption) error {
				methods[method]++
				switch out := reply.(type) {
				case *CreateOrderResponse:
					out.Order = &Order{Id: "order-1"}
				case *GetOrderResponse:
					out.Order = &Order{Id: "order-1"}
				case *ListOrdersResponse:
					out.Orders = []*Order{{Id: "order-1"}}
				case *PayOrderResponse:
					out.Status = OrderStatus_ORDER_STATUS_PAID
				case *CancelOrderResponse:
					out.Status = OrderStatus_ORDER_STATUS_CANCELED
				case *RefundOrderResponse:
					out.Status = OrderStatus_ORDER_STATUS_REFUNDED
				default:
					t.Fatalf("unexpected reply type: %T", out)
				}
				return nil
			},
		}

		client := NewOrderServiceClient(conn)
		ctx := context.Background()
		if _, err := client.CreateOrder(ctx, &CreateOrderRequest{}); err != nil {
			t.Fatalf("CreateOrder failed: %v", err)
		}
		if _, err := client.GetOrder(ctx, &GetOrderRequest{}); err != nil {
			t.Fatalf("GetOrder failed: %v", err)
		}
		if _, err := client.ListOrders(ctx, &ListOrdersRequest{}); err != nil {
			t.Fatalf("ListOrders failed: %v", err)
		}
		if _, err := client.PayOrder(ctx, &PayOrderRequest{}); err != nil {
			t.Fatalf("PayOrder failed: %v", err)
		}
		if _, err := client.CancelOrder(ctx, &CancelOrderRequest{}); err != nil {
			t.Fatalf("CancelOrder failed: %v", err)
		}
		if _, err := client.RefundOrder(ctx, &RefundOrderRequest{}); err != nil {
			t.Fatalf("RefundOrder failed: %v", err)
		}

		for _, method := range []string{
			OrderService_CreateOrder_FullMethodName,
			OrderService_GetOrder_FullMethodName,
			OrderService_ListOrders_FullMethodName,
			OrderService_PayOrder_FullMethodName,
			OrderService_CancelOrder_FullMethodName,
			OrderService_RefundOrder_FullMethodName,
		} {
			if methods[method] != 1 {
				t.Fatalf("expected method %s called exactly once, got %d", method, methods[method])
			}
		}
	})

	t.Run("error", func(t *testing.T) {
		conn := &fakeClientConn{
			invoke: func(context.Context, string, any, any, ...grpc.CallOption) error {
				return status.Error(codes.Internal, "boom")
			},
		}
		client := NewOrderServiceClient(conn)
		ctx := context.Background()

		for name, call := range map[string]func() error{
			"CreateOrder": func() error { _, err := client.CreateOrder(ctx, &CreateOrderRequest{}); return err },
			"GetOrder":    func() error { _, err := client.GetOrder(ctx, &GetOrderRequest{}); return err },
			"ListOrders":  func() error { _, err := client.ListOrders(ctx, &ListOrdersRequest{}); return err },
			"PayOrder":    func() error { _, err := client.PayOrder(ctx, &PayOrderRequest{}); return err },
			"CancelOrder": func() error { _, err := client.CancelOrder(ctx, &CancelOrderRequest{}); return err },
			"RefundOrder": func() error { _, err := client.RefundOrder(ctx, &RefundOrderRequest{}); return err },
		} {
			if err := call(); status.Code(err) != codes.Internal {
				t.Fatalf("%s expected Internal error, got %v", name, err)
			}
		}
	})
}

func TestUnimplementedOrderServiceServer(t *testing.T) {
	var srv UnimplementedOrderServiceServer
	ctx := context.Background()

	for name, call := range map[string]func() error{
		"CreateOrder": func() error { _, err := srv.CreateOrder(ctx, &CreateOrderRequest{}); return err },
		"GetOrder":    func() error { _, err := srv.GetOrder(ctx, &GetOrderRequest{}); return err },
		"ListOrders":  func() error { _, err := srv.ListOrders(ctx, &ListOrdersRequest{}); return err },
		"PayOrder":    func() error { _, err := srv.PayOrder(ctx, &PayOrderRequest{}); return err },
		"CancelOrder": func() error { _, err := srv.CancelOrder(ctx, &CancelOrderRequest{}); return err },
		"RefundOrder": func() error { _, err := srv.RefundOrder(ctx, &RefundOrderRequest{}); return err },
	} {
		if err := call(); status.Code(err) != codes.Unimplemented {
			t.Fatalf("%s expected Unimplemented error, got %v", name, err)
		}
	}

	srv.mustEmbedUnimplementedOrderServiceServer()
}

type grpcGeneratedHandlerCase struct {
	name   string
	method string
	call   func(interface{}, context.Context, func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error)
}

func TestGeneratedHandlers(t *testing.T) {
	srv := &grpcTestOrderService{}
	ctx := context.Background()

	cases := []grpcGeneratedHandlerCase{
		{name: "CreateOrder", method: OrderService_CreateOrder_FullMethodName, call: _OrderService_CreateOrder_Handler},
		{name: "GetOrder", method: OrderService_GetOrder_FullMethodName, call: _OrderService_GetOrder_Handler},
		{name: "ListOrders", method: OrderService_ListOrders_FullMethodName, call: _OrderService_ListOrders_Handler},
		{name: "PayOrder", method: OrderService_PayOrder_FullMethodName, call: _OrderService_PayOrder_Handler},
		{name: "CancelOrder", method: OrderService_CancelOrder_FullMethodName, call: _OrderService_CancelOrder_Handler},
		{name: "RefundOrder", method: OrderService_RefundOrder_FullMethodName, call: _OrderService_RefundOrder_Handler},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.call(srv, ctx, func(interface{}) error { return errors.New("decode failed") }, nil); err == nil {
				t.Fatalf("expected decode error")
			}

			resp, err := tc.call(srv, ctx, decodeFor(tc.name), nil)
			if err != nil {
				t.Fatalf("handler without interceptor failed: %v", err)
			}
			if resp == nil {
				t.Fatalf("expected non-nil response")
			}

			interceptorCalled := false
			resp, err = tc.call(srv, ctx, decodeFor(tc.name), func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
				interceptorCalled = true
				if info.FullMethod != tc.method {
					t.Fatalf("unexpected full method: got %s want %s", info.FullMethod, tc.method)
				}
				return handler(ctx, req)
			})
			if err != nil {
				t.Fatalf("handler with interceptor failed: %v", err)
			}
			if !interceptorCalled {
				t.Fatalf("interceptor was not called")
			}
			if resp == nil {
				t.Fatalf("expected non-nil response")
			}
		})
	}
}

func TestRegisterAndServiceDescriptor(t *testing.T) {
	g := grpc.NewServer()
	RegisterOrderServiceServer(g, &grpcTestOrderService{})

	if got, want := OrderService_ServiceDesc.ServiceName, "oms.v1.OrderService"; got != want {
		t.Fatalf("unexpected service name: got %s want %s", got, want)
	}
	if len(OrderService_ServiceDesc.Methods) != 6 {
		t.Fatalf("expected 6 method descriptors, got %d", len(OrderService_ServiceDesc.Methods))
	}
	if OrderService_ServiceDesc.Metadata == "" {
		t.Fatalf("metadata should not be empty")
	}
}

func decodeFor(name string) func(interface{}) error {
	return func(v interface{}) error {
		switch req := v.(type) {
		case *CreateOrderRequest:
			req.CustomerId = "cust-1"
			req.Currency = "USD"
		case *GetOrderRequest:
			req.OrderId = "order-1"
		case *ListOrdersRequest:
			req.CustomerId = "cust-1"
		case *PayOrderRequest:
			req.OrderId = "order-1"
		case *CancelOrderRequest:
			req.OrderId = "order-1"
			req.Reason = "test"
		case *RefundOrderRequest:
			req.OrderId = "order-1"
		default:
			return status.Errorf(codes.Internal, "unexpected request type for %s: %T", name, req)
		}
		return nil
	}
}
